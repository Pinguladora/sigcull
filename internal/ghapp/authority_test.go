package ghapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pinguladora/sigcull/internal/verify"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/go-github/v91/github"
)

// newEntity generates a throwaway OpenPGP key for signing test payloads.
func newEntity(t *testing.T) *openpgp.Entity {
	t.Helper()
	e, err := openpgp.NewEntity("Signer", "", "signer@example.com", nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return e
}

// armoredSignature returns a detached armored OpenPGP signature over data.
func armoredSignature(t *testing.T, signer *openpgp.Entity, data []byte) string {
	t.Helper()
	var sig bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&sig, signer, bytes.NewReader(data), nil); err != nil {
		t.Fatalf("detach sign: %v", err)
	}
	return sig.String()
}

// newTestClient returns a github client whose requests hit baseURL.
func newTestClient(t *testing.T, baseURL string) *github.Client {
	t.Helper()
	client, err := github.NewClient(github.WithEnterpriseURLs(baseURL+"/", baseURL+"/"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

// commitResponse renders a git-data commit JSON body carrying a verification.
func commitResponse(t *testing.T, verified bool, payload, signature string) string {
	t.Helper()
	v := map[string]any{"verified": verified, "reason": "test"}
	if payload != "" {
		v["payload"] = payload
	}
	if signature != "" {
		v["signature"] = signature
	}
	b, err := json.Marshal(map[string]any{"sha": "abc", "verification": v})
	if err != nil {
		t.Fatalf("marshal commit: %v", err)
	}
	return string(b)
}

// commitPath is where the go-github enterprise client requests a git commit: it
// appends /api/v3/ to the configured base URL.
const commitPath = "/api/v3/repos/o/r/git/commits/abc"

// serveCommit stands up a server returning body for the git-commit endpoint.
func serveCommit(t *testing.T, body string) *github.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(commitPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return newTestClient(t, srv.URL)
}

func TestGitHubAuthorityAcceptsVerified(t *testing.T) {
	g, err := NewGitHubAuthority(false)
	if err != nil {
		t.Fatalf("NewGitHubAuthority: %v", err)
	}
	client := serveCommit(t, commitResponse(t, true, "", ""))

	res, ok := g.Accept(context.Background(), client, "o", "r", verify.CommitData{SHA: "abc"})
	if !ok || !res.OK {
		t.Fatalf("expected acceptance, got ok=%v res=%+v", ok, res)
	}
	if res.Authority != verify.AuthorityGitHub {
		t.Errorf("Authority = %q, want %q", res.Authority, verify.AuthorityGitHub)
	}
}

func TestGitHubAuthorityRejectsUnverified(t *testing.T) {
	g, err := NewGitHubAuthority(false)
	if err != nil {
		t.Fatalf("NewGitHubAuthority: %v", err)
	}
	client := serveCommit(t, commitResponse(t, false, "", ""))

	if _, ok := g.Accept(context.Background(), client, "o", "r", verify.CommitData{SHA: "abc"}); ok {
		t.Fatal("an unverified commit must not be accepted")
	}
}

func TestGitHubAuthorityFailsClosedOnAPIError(t *testing.T) {
	g, err := NewGitHubAuthority(false)
	if err != nil {
		t.Fatalf("NewGitHubAuthority: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(commitPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	if _, ok := g.Accept(context.Background(), client, "o", "r", verify.CommitData{SHA: "abc"}); ok {
		t.Fatal("an API error must fail closed")
	}
}

func TestGitHubAuthorityWebFlowOnlyRejectsForeignKey(t *testing.T) {
	// verified=true, but the signature is from a random key, not GitHub web-flow.
	g, err := NewGitHubAuthority(true)
	if err != nil {
		t.Fatalf("NewGitHubAuthority: %v", err)
	}
	payload := "tree abc\n\nmerge\n"
	sig := armoredSignature(t, newEntity(t), []byte(payload))
	client := serveCommit(t, commitResponse(t, true, payload, sig))

	if _, ok := g.Accept(context.Background(), client, "o", "r", verify.CommitData{SHA: "abc"}); ok {
		t.Fatal("webFlowOnly must reject a signature not made by GitHub's web-flow key")
	}
}

func TestWebFlowSignedAcceptsRingMember(t *testing.T) {
	signer := newEntity(t)
	g := &GitHubAuthority{webFlowKeys: openpgp.EntityList{signer}}
	payload := "commit payload"
	if !g.webFlowSigned(payload, armoredSignature(t, signer, []byte(payload))) {
		t.Fatal("a signature from a ring member should verify")
	}
}

func TestWebFlowSignedRejectsNonMember(t *testing.T) {
	g := &GitHubAuthority{webFlowKeys: openpgp.EntityList{newEntity(t)}}
	payload := "commit payload"
	if g.webFlowSigned(payload, armoredSignature(t, newEntity(t), []byte(payload))) {
		t.Fatal("a signature from a non-member key must be rejected")
	}
}

func TestWebFlowSignedRejectsEmpty(t *testing.T) {
	g := &GitHubAuthority{webFlowKeys: openpgp.EntityList{newEntity(t)}}
	if g.webFlowSigned("", "sig") || g.webFlowSigned("payload", "") {
		t.Fatal("empty payload or signature must be rejected")
	}
}

// TestWebFlowKeyEmbedded guards the vendored web-flow key against corruption.
func TestWebFlowKeyEmbedded(t *testing.T) {
	g, err := NewGitHubAuthority(true)
	if err != nil {
		t.Fatalf("load embedded web-flow key: %v", err)
	}
	if len(g.webFlowKeys) == 0 {
		t.Fatal("embedded web-flow keyring is empty")
	}
	found := false
	for _, e := range g.webFlowKeys {
		for _, id := range e.Identities {
			if id.UserId != nil && id.UserId.Email == "noreply@github.com" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("embedded key should carry the GitHub noreply@github.com UID")
	}
}
