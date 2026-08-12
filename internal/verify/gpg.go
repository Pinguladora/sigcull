package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// pgpSignatureMarker identifies an armored OpenPGP signature in a commit's
// gpgsig header, distinguishing it from a Sigstore CMS blob so the gpg authority
// can decline signatures that are not its concern.
const pgpSignatureMarker = "BEGIN PGP SIGNATURE"

// gpgAuthority accepts a commit whose OpenPGP signature verifies against a
// trusted keyring and whose signing key carries an allowlisted UID email. It
// verifies in-process with go-crypto, so no gpg binary is needed.
type gpgAuthority struct {
	allow   map[string]struct{}
	keyring openpgp.EntityList
}

// Name identifies the gpg authority in Check Run output.
func (g *gpgAuthority) Name() string { return AuthorityGPG }

// NewGPGKey builds a gpg authority from a public keyring file (armored or
// binary) and the set of UID emails a signing key may carry to be accepted.
func NewGPGKey(keyringPath string, allowEmails []string) (Authority, error) {
	if keyringPath == "" {
		return nil, errors.New("keyringPath is required")
	}
	if len(allowEmails) == 0 {
		return nil, errors.New("allowEmails must list at least one address")
	}

	// The keyring path is operator config, not attacker input.
	data, err := os.ReadFile(keyringPath) //nolint:gosec // G304: operator-supplied keyring path
	if err != nil {
		return nil, fmt.Errorf("read keyring %q: %w", keyringPath, err)
	}
	keyring, err := readKeyRing(data)
	if err != nil {
		return nil, fmt.Errorf("parse keyring %q: %w", keyringPath, err)
	}
	if len(keyring) == 0 {
		return nil, fmt.Errorf("keyring %q holds no keys", keyringPath)
	}

	allow := make(map[string]struct{}, len(allowEmails))
	for _, e := range allowEmails {
		allow[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}
	return &gpgAuthority{keyring: keyring, allow: allow}, nil
}

// readKeyRing parses an armored or binary OpenPGP public keyring.
func readKeyRing(data []byte) (openpgp.EntityList, error) {
	if bytes.Contains(data, []byte("-----BEGIN PGP")) {
		ring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("read armored keyring: %w", err)
		}
		return ring, nil
	}
	ring, err := openpgp.ReadKeyRing(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("read binary keyring: %w", err)
	}
	return ring, nil
}

// Verify implements Authority. It fails closed and never returns an error.
func (g *gpgAuthority) Verify(_ context.Context, commit CommitData) CommitResult {
	res := CommitResult{SHA: commit.SHA}
	if len(commit.Signature) == 0 {
		res.Reason = ReasonUnsigned
		return res
	}
	sig := string(commit.Signature)
	if !strings.Contains(sig, pgpSignatureMarker) {
		// A Sigstore CMS signature, not OpenPGP. This authority does not apply.
		res.Reason = "not an OpenPGP signature"
		return res
	}

	signer, err := openpgp.CheckArmoredDetachedSignature(
		g.keyring, bytes.NewReader(commit.Payload), strings.NewReader(sig), nil)
	if err != nil {
		res.Reason = "OpenPGP signature did not verify against the trusted keyring"
		return res
	}

	res.Issuer = keyIssuer(signer)
	if email, ok := g.allowedEmail(signer); ok {
		res.OK = true
		res.Authority = AuthorityGPG
		res.SignerSAN = email
		return res
	}
	res.SignerSAN = entityEmail(signer)
	res.Reason = NotAllowlistedReason(res.SignerSAN, res.Issuer)
	return res
}

// allowedEmail returns the first UID email on the signing key that is on the
// allowlist.
func (g *gpgAuthority) allowedEmail(e *openpgp.Entity) (string, bool) {
	for _, id := range e.Identities {
		if id.UserId == nil {
			continue
		}
		if _, ok := g.allow[strings.ToLower(id.UserId.Email)]; ok {
			return id.UserId.Email, true
		}
	}
	return "", false
}

// entityEmail returns the key's primary UID email, for reporting a rejected
// signer.
func entityEmail(e *openpgp.Entity) string {
	id := e.PrimaryIdentity()
	if id != nil && id.UserId != nil {
		return id.UserId.Email
	}
	return ""
}

// keyIssuer renders the signing key's long key ID for the Check Run Issuer
// column, mirroring how the keyless authority reports the OIDC issuer.
func keyIssuer(e *openpgp.Entity) string {
	return fmt.Sprintf("gpg:%016X", e.PrimaryKey.KeyId)
}
