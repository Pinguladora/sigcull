package verify

import (
	"context"
	"strings"
	"testing"
)

// fakeAuthority returns a fixed result, stamped with the commit SHA.
type fakeAuthority struct {
	name string
	res  CommitResult
}

func (f fakeAuthority) Name() string { return f.name }

func (f fakeAuthority) Verify(_ context.Context, c CommitData) CommitResult {
	res := f.res
	res.SHA = c.SHA
	return res
}

func TestCompositeFirstAcceptWins(t *testing.T) {
	c := NewComposite(
		fakeAuthority{name: AuthorityKeyless, res: CommitResult{Reason: "no sig"}},
		fakeAuthority{name: AuthorityGitHub, res: CommitResult{OK: true}},
	)
	got := c.Verify(context.Background(), CommitData{SHA: "abc"})
	if !got.OK {
		t.Fatalf("expected acceptance, got %+v", got)
	}
	if got.Authority != AuthorityGitHub {
		t.Fatalf("Authority = %q, want %q", got.Authority, AuthorityGitHub)
	}
}

func TestCompositeAllRejectAggregates(t *testing.T) {
	c := NewComposite(
		fakeAuthority{name: AuthorityKeyless, res: CommitResult{Reason: "not allowlisted", SignerSAN: "x@y.z", Issuer: "iss"}},
		fakeAuthority{name: AuthorityGPG, res: CommitResult{Reason: "no pgp signature"}},
	)
	got := c.Verify(context.Background(), CommitData{SHA: "abc"})
	if got.OK {
		t.Fatalf("expected rejection, got %+v", got)
	}
	// The signer identified by keyless is kept so the Check Run can name it.
	if got.SignerSAN != "x@y.z" || got.Issuer != "iss" {
		t.Fatalf("expected the informative signer to be kept, got SAN=%q issuer=%q", got.SignerSAN, got.Issuer)
	}
	if !strings.Contains(got.Reason, AuthorityKeyless) || !strings.Contains(got.Reason, AuthorityGPG) {
		t.Fatalf("aggregated reason should mention each authority: %q", got.Reason)
	}
}

func TestCompositeSingleAuthorityVerbatim(t *testing.T) {
	// A lone authority is returned unchanged, so single-keyless reporting is
	// identical to the pre-authorities behaviour.
	c := NewComposite(fakeAuthority{name: AuthorityKeyless, res: CommitResult{Reason: ReasonUnsigned}})
	got := c.Verify(context.Background(), CommitData{SHA: "abc"})
	if got.Reason != ReasonUnsigned {
		t.Fatalf("lone authority reason should be verbatim, got %q", got.Reason)
	}
}

func TestCompositeSingleAuthorityStampsName(t *testing.T) {
	c := NewComposite(fakeAuthority{name: AuthorityKeyless, res: CommitResult{OK: true, SignerSAN: "me@example.com"}})
	got := c.Verify(context.Background(), CommitData{SHA: "abc"})
	if got.Authority != AuthorityKeyless {
		t.Fatalf("Authority = %q, want %q", got.Authority, AuthorityKeyless)
	}
}
