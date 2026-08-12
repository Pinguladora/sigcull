package config

import (
	"context"
	"strings"
	"testing"

	"github.com/Pinguladora/sigcull/internal/policy"
	"github.com/Pinguladora/sigcull/internal/verify"
)

// publicTrust builds against the embedded public-good root, so the keyless
// authority constructs offline with no network.
var publicTrust = verify.TrustParams{Kind: verify.Public}

func TestBuildAuthoritiesRoutesSSHAndX509(t *testing.T) {
	// An sshKey with a missing path surfaces an sshKey-prefixed error, proving
	// the build routes to the SSH authority.
	ssh := Config{Verify: Verify{Authorities: []AuthoritySpec{{SSHKey: &SSHKeySpec{}}}}}
	if _, _, err := ssh.buildAuthorities(context.Background(), publicTrust); err == nil ||
		!strings.Contains(err.Error(), "sshKey") {
		t.Fatalf("expected an sshKey error, got %v", err)
	}

	x509 := Config{Verify: Verify{Authorities: []AuthoritySpec{{X509: &X509Spec{}}}}}
	if _, _, err := x509.buildAuthorities(context.Background(), publicTrust); err == nil ||
		!strings.Contains(err.Error(), "x509") {
		t.Fatalf("expected an x509 error, got %v", err)
	}
}

func keylessSpec() AuthoritySpec {
	return AuthoritySpec{Keyless: &KeylessSpec{Allow: []policy.Identity{
		{Issuer: "https://accounts.google.com", SAN: "me@example.com"},
	}}}
}

func TestBuildAuthoritiesRequiresAtLeastOne(t *testing.T) {
	c := Config{}
	if _, _, err := c.buildAuthorities(context.Background(), publicTrust); err == nil {
		t.Fatal("expected an error when no authority is configured")
	}
}

func TestBuildAuthoritiesRejectsMultipleKindsInOneEntry(t *testing.T) {
	c := Config{Verify: Verify{Authorities: []AuthoritySpec{{
		Keyless:        &KeylessSpec{},
		GitHubVerified: &GitHubVerifiedSpec{},
	}}}}
	if _, _, err := c.buildAuthorities(context.Background(), publicTrust); err == nil {
		t.Fatal("expected an error when one entry sets two authority kinds")
	}
}

func TestBuildAuthoritiesRejectsDuplicateGitHub(t *testing.T) {
	c := Config{Verify: Verify{Authorities: []AuthoritySpec{
		{GitHubVerified: &GitHubVerifiedSpec{}},
		{GitHubVerified: &GitHubVerifiedSpec{WebFlowOnly: true}},
	}}}
	if _, _, err := c.buildAuthorities(context.Background(), publicTrust); err == nil {
		t.Fatal("expected an error for two githubVerified authorities")
	}
}

func TestBuildAuthoritiesSeparatesGitHubFromCrypto(t *testing.T) {
	c := Config{Verify: Verify{Authorities: []AuthoritySpec{
		keylessSpec(),
		{GitHubVerified: &GitHubVerifiedSpec{WebFlowOnly: true}},
	}}}
	auths, github, err := c.buildAuthorities(context.Background(), publicTrust)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auths) != 1 {
		t.Fatalf("expected 1 crypto authority (github is a processor fallback), got %d", len(auths))
	}
	if github == nil || !github.WebFlowOnly {
		t.Fatalf("expected a githubVerified authority with webFlowOnly, got %+v", github)
	}
}

// TestLoadExampleConfig guards config.example.yaml against schema drift.
func TestLoadExampleConfig(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "test-secret")
	t.Setenv("GITHUB_APP_ID", "12345")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----")

	r, err := Load("../../config.example.yaml")
	if err != nil {
		t.Fatalf("load example config: %v", err)
	}
	if r.Verifier == nil {
		t.Fatal("expected a verifier to be built")
	}
	if r.AppID != 12345 {
		t.Errorf("AppID = %d, want 12345", r.AppID)
	}
	if !r.MaskIdentities {
		t.Error("expected identities to be masked by default")
	}
	if !r.GitHubVerified || !r.GitHubWebFlowOnly {
		t.Error("expected the example to configure a webFlowOnly githubVerified authority")
	}
}
