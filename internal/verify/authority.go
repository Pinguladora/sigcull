package verify

import (
	"context"
	"strings"
)

// Authority names, reported on each CommitResult so the Check Run can say which
// signature source accepted a commit, and so exemptions render distinctly from
// verified passes.
const (
	AuthorityKeyless = "keyless" // Sigstore / gitsign keyless signature
	AuthorityGPG     = "gpg"     // trusted OpenPGP key
	AuthoritySSH     = "ssh"     // trusted SSH key (allowed_signers)
	AuthorityX509    = "x509"    // trusted X.509 / S-MIME CA
	AuthorityGitHub  = "github"  // GitHub-verified signature
	AuthorityExempt  = "exempt"  // skipped by an exemption, not verified
)

// Authority is one accepted signature source. A commit satisfies the policy if
// any configured authority accepts it. Name identifies the authority in output.
type Authority interface {
	Verify(ctx context.Context, commit CommitData) CommitResult
	Name() string
}

// Composite evaluates a commit against several authorities with OR semantics:
// the first authority that accepts wins. When none accept, it reports why each
// rejected. Composite is itself a Verifier.
type Composite struct {
	authorities []Authority
}

// NewComposite builds a Composite from the given authorities, tried in order.
func NewComposite(authorities ...Authority) *Composite {
	return &Composite{authorities: authorities}
}

// Verify tries each authority in order and returns the first acceptance,
// stamped with the accepting authority's name. When no authority accepts it
// returns a single failing result: the most informative rejection (one that
// identified a signer) carrying an aggregated reason.
//
// A lone authority is returned verbatim, so a single keyless policy reports
// exactly as it did before authorities existed.
func (c *Composite) Verify(ctx context.Context, commit CommitData) CommitResult {
	if len(c.authorities) == 0 {
		// No crypto authority is configured (a githubVerified-only policy). The
		// processor's GitHub fallback decides acceptance.
		return CommitResult{SHA: commit.SHA, Reason: "no keyless or gpg authority accepted this commit"}
	}
	if len(c.authorities) == 1 {
		res := c.authorities[0].Verify(ctx, commit)
		if res.OK && res.Authority == "" {
			res.Authority = c.authorities[0].Name()
		}
		return res
	}

	reasons := make([]string, 0, len(c.authorities))
	best := CommitResult{SHA: commit.SHA}
	for _, a := range c.authorities {
		res := a.Verify(ctx, commit)
		if res.OK {
			if res.Authority == "" {
				res.Authority = a.Name()
			}
			return res
		}
		reasons = append(reasons, a.Name()+": "+res.Reason)
		// Keep the first rejection that identified a signer, so the Check Run can
		// still name who signed the commit even when it was not accepted.
		if best.SignerSAN == "" && res.SignerSAN != "" {
			best.SignerSAN = res.SignerSAN
			best.Issuer = res.Issuer
		}
	}
	best.Reason = "no configured authority accepted this commit (" + strings.Join(reasons, "; ") + ")"
	return best
}
