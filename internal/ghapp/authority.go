package ghapp

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/Pinguladora/sigcull/internal/verify"
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/go-github/v92/github"
)

// webFlowKey is GitHub's web-flow commit-signing public key, vendored from
// https://github.com/web-flow.gpg. GitHub signs its own web and squash merges
// with it. webFlowOnly verifies the commit's signature against this key rather
// than trusting a self-reported issuer key ID, so a forged issuer cannot pass.
// Refresh it if GitHub rotates the key.
//
//go:embed webflow.gpg
var webFlowKey []byte

// GitHubAuthority accepts a commit that GitHub itself cryptographically
// verified. It is evaluated as a fallback in the processor rather than inside
// the crypto composite, because it needs the per-installation GitHub client,
// which only exists at request time.
type GitHubAuthority struct {
	webFlowKeys openpgp.EntityList
	webFlowOnly bool
}

// NewGitHubAuthority builds the authority. When webFlowOnly is true, only
// GitHub's own web and squash merges qualify, verified against GitHub's embedded
// web-flow key, not a user's vigilant-mode key that GitHub merely verifies.
func NewGitHubAuthority(webFlowOnly bool) (*GitHubAuthority, error) {
	g := &GitHubAuthority{webFlowOnly: webFlowOnly}
	if !webFlowOnly {
		return g, nil
	}
	ring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(webFlowKey))
	if err != nil {
		return nil, fmt.Errorf("load embedded web-flow key: %w", err)
	}
	g.webFlowKeys = ring
	return g, nil
}

// Accept reports whether GitHub verified the commit's signature. It fails closed:
// any API error or an unverified signature yields ok=false. The check rests on
// GitHub's own cryptographic verification, which a pusher cannot forge for
// GitHub's identity, so a spoofed committer cannot self-accept. With webFlowOnly,
// the signature must additionally verify against GitHub's own web-flow key.
func (g *GitHubAuthority) Accept(
	ctx context.Context, client *github.Client, owner, repo string, c verify.CommitData,
) (verify.CommitResult, bool) {
	commit, _, err := client.Git.GetCommit(ctx, owner, repo, c.SHA)
	if err != nil {
		return verify.CommitResult{}, false
	}
	v := commit.GetVerification()
	if !v.GetVerified() {
		return verify.CommitResult{}, false
	}
	if g.webFlowOnly && !g.webFlowSigned(v.GetPayload(), v.GetSignature()) {
		return verify.CommitResult{}, false
	}
	return verify.CommitResult{SHA: c.SHA, OK: true, Authority: verify.AuthorityGitHub}, true
}

// webFlowSigned reports whether the armored detached signature over payload
// verifies against GitHub's web-flow key. GitHub returns both the signature and
// the exact signed payload on the verification object.
func (g *GitHubAuthority) webFlowSigned(payload, signature string) bool {
	if payload == "" || signature == "" {
		return false
	}
	_, err := openpgp.CheckArmoredDetachedSignature(
		g.webFlowKeys, strings.NewReader(payload), strings.NewReader(signature), nil)
	return err == nil
}
