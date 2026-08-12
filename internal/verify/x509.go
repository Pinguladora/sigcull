package verify

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/digitorus/pkcs7"
)

// x509Authority accepts a commit whose detached CMS (S-MIME) signature verifies
// and whose signing certificate chains to a trusted CA bundle and carries an
// allowlisted email. It is the non-Sigstore sibling of the keyless authority:
// same CMS format, but trust comes from a private CA rather than Fulcio, and
// there is no Rekor transparency requirement.
type x509Authority struct {
	roots *x509.CertPool
	allow map[string]struct{}
}

// Name identifies the x509 authority in Check Run output.
func (a *x509Authority) Name() string { return AuthorityX509 }

// NewX509 builds an x509 authority from a PEM CA bundle and the set of signer
// certificate emails that may be accepted.
func NewX509(caBundlePath string, allowEmails []string) (Authority, error) {
	if caBundlePath == "" {
		return nil, errors.New("caBundlePath is required")
	}
	if len(allowEmails) == 0 {
		return nil, errors.New("allowEmails must list at least one address")
	}
	// The path is operator config, not attacker input.
	pemBytes, err := os.ReadFile(caBundlePath) //nolint:gosec // G304: operator-supplied path
	if err != nil {
		return nil, fmt.Errorf("read ca bundle %q: %w", caBundlePath, err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("ca bundle %q holds no certificates", caBundlePath)
	}
	allow := make(map[string]struct{}, len(allowEmails))
	for _, e := range allowEmails {
		allow[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}
	return &x509Authority{roots: roots, allow: allow}, nil
}

// Verify implements Authority. It fails closed and never returns an error.
func (a *x509Authority) Verify(_ context.Context, commit CommitData) CommitResult {
	res := CommitResult{SHA: commit.SHA}
	if len(commit.Signature) == 0 {
		res.Reason = ReasonUnsigned
		return res
	}
	block, _ := pem.Decode(commit.Signature)
	if block == nil {
		res.Reason = "not a CMS/X.509 signature"
		return res
	}
	p7, err := pkcs7.Parse(block.Bytes)
	if err != nil {
		res.Reason = "malformed CMS signature"
		return res
	}

	// Detached signature: the signed content is the commit payload.
	p7.Content = commit.Payload
	if err := p7.VerifyWithChain(a.roots); err != nil {
		res.Reason = "CMS signature did not verify against the trusted CA bundle"
		return res
	}

	signer := p7.GetOnlySigner()
	if signer == nil {
		res.Reason = ReasonParseFailure
		return res
	}
	res.Issuer = "x509:" + signer.Issuer.CommonName
	if email, ok := a.allowedEmail(signer); ok {
		res.OK = true
		res.Authority = AuthorityX509
		res.SignerSAN = email
		return res
	}
	res.SignerSAN = certEmail(signer)
	res.Reason = NotAllowlistedReason(res.SignerSAN, res.Issuer)
	return res
}

// allowedEmail returns the first SAN email on the signer certificate that is on
// the allowlist.
func (a *x509Authority) allowedEmail(cert *x509.Certificate) (string, bool) {
	for _, e := range cert.EmailAddresses {
		if _, ok := a.allow[strings.ToLower(e)]; ok {
			return e, true
		}
	}
	return "", false
}

// certEmail returns the signer's primary email, for reporting a rejected signer.
func certEmail(cert *x509.Certificate) string {
	if len(cert.EmailAddresses) > 0 {
		return cert.EmailAddresses[0]
	}
	return cert.Subject.CommonName
}
