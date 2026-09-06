// Package verify defines the commit-signature verification contract and the
// result types reported back as GitHub Check Run annotations.
//
// Verification runs fully in-process: gitsign is used as a library and roots
// come from a Sigstore trusted_root (embedded, a file, or TUF). No subprocess
// and no filesystem are involved. The Verifier interface takes already-extracted
// commit bytes (CommitData), so it is decoupled from how the commit was fetched.
package verify

import (
	"context"
	"fmt"
	"strings"
)

// CommitData is one commit's verification input: the signed payload (the commit
// object without its signature), the CMS signature from the gpgsig header, and
// cheap metadata used for exemptions and committer matching.
type CommitData struct {
	SHA            string
	CommitterEmail string
	CommitterName  string
	Payload        []byte
	Signature      []byte
	ParentCount    int
}

// CommitResult is the outcome of verifying a single commit. It is the single
// unit the Check Run layer turns into a per-commit annotation. Authority names
// the source that accepted the commit (keyless / gpg / github), or exempt when
// the commit was skipped rather than verified.
type CommitResult struct {
	SHA       string
	SignerSAN string
	Issuer    string
	Reason    string
	Authority string
	OK        bool
}

// Verifier checks one commit against an identity policy.
//
// Verify must never panic and must fail closed: any ambiguity results in
// OK=false with a populated Reason.
type Verifier interface {
	Verify(ctx context.Context, commit CommitData) CommitResult
}

// Failure reasons. Kept here as the single source of truth so the verifier and
// the check-run layer agree on the exact wording.
const (
	ReasonUnsigned     = "no valid Sigstore signature (unsigned or cryptographically invalid)"
	ReasonParseFailure = "signature verified but the signer identity could not be read (failing closed)"
	// ReasonNoTransparencyProof is returned when the CMS signature is valid but
	// carries no embedded Rekor inclusion proof. Verification is strictly offline,
	// so the online Rekor search is never used and such a signature cannot be
	// confirmed as logged. Sign with gitsign's offline Rekor mode to embed it.
	ReasonNoTransparencyProof = "signature has no embedded Rekor transparency proof (sign with gitsign rekorMode=offline)"
)

// NotAllowlistedReason builds the reason for a valid signature whose identity is
// not on the allowlist. This is the case the acceptance criteria require us to
// name explicitly in the annotation.
func NotAllowlistedReason(san, issuer string) string {
	return fmt.Sprintf("signed by %s (%s) but that identity is not on the allowlist", san, issuer)
}

// CommitterMatches reports whether a commit's committer identity matches the
// signing SAN. It mirrors gitsign's sign-time matchCommitter, but is enforced
// here at verify time (gitsign only enforces it while signing): an email SAN
// must equal the committer email, and a URI SAN (a CI or workflow identity)
// must equal the committer name. Enforcing it closes the attribution gap where
// an allowlisted signer commits under someone else's name.
func CommitterMatches(committerEmail, committerName, san string) bool {
	if strings.Contains(san, "://") {
		return committerName == san
	}
	return strings.EqualFold(committerEmail, san)
}

// CommitterMismatchReason builds the failure reason for a committer that does
// not match the signing identity.
func CommitterMismatchReason(san, committerEmail, committerName string) string {
	return fmt.Sprintf("signing identity %s does not match the commit committer (%s / %s)",
		san, committerEmail, committerName)
}
