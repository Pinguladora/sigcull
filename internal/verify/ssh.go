package verify

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSH signature constants from OpenSSH's PROTOCOL.sshsig. A git SSH signature is
// an armored SSHSIG blob whose namespace is "git".
const (
	sshSignatureMarker = "BEGIN SSH SIGNATURE"
	sshMagic           = "SSHSIG"
	sshGitNamespace    = "git"
	sshHashSHA256      = "sha256"
	sshHashSHA512      = "sha512"
)

// sshSigBlob is the SSHSIG wire structure that follows the 6-byte magic. The
// field order is the on-wire order defined by PROTOCOL.sshsig and MUST NOT be
// reordered: ssh.Marshal and ssh.Unmarshal serialise fields in declaration
// order, so reordering would break interoperability with real signatures.
//
//nolint:govet // fieldalignment: wire order is fixed by PROTOCOL.sshsig
type sshSigBlob struct {
	Version   uint32
	PublicKey []byte
	Namespace string
	Reserved  string
	HashAlgo  string
	Signature []byte
}

// sshSignedData is the blob the signature actually covers, following the magic.
// Its field order is the on-wire order from PROTOCOL.sshsig and is fixed.
type sshSignedData struct {
	Namespace string
	Reserved  string
	HashAlgo  string
	Hash      []byte
}

// sshAuthority accepts a commit whose SSH signature verifies against a trusted
// key from an OpenSSH allowed_signers file and whose principal is allowlisted.
// It verifies in-process with x/crypto/ssh, so no ssh binary is needed.
type sshAuthority struct {
	keys  map[string][]string // marshalled public key -> its principals
	allow map[string]struct{} // allowed principals; empty means any in the file
}

// Name identifies the ssh authority in Check Run output.
func (a *sshAuthority) Name() string { return AuthoritySSH }

// NewSSHKey builds an ssh authority from an OpenSSH allowed_signers file and an
// optional principal allowlist. When allowPrincipals is empty, any principal
// named in the file is accepted.
func NewSSHKey(allowedSignersPath string, allowPrincipals []string) (Authority, error) {
	if allowedSignersPath == "" {
		return nil, errors.New("allowedSignersPath is required")
	}
	// The path is operator config, not attacker input.
	data, err := os.ReadFile(allowedSignersPath) //nolint:gosec // G304: operator-supplied path
	if err != nil {
		return nil, fmt.Errorf("read allowed_signers %q: %w", allowedSignersPath, err)
	}
	keys := parseAllowedSigners(data)
	if len(keys) == 0 {
		return nil, fmt.Errorf("allowed_signers %q holds no usable keys", allowedSignersPath)
	}
	allow := make(map[string]struct{}, len(allowPrincipals))
	for _, p := range allowPrincipals {
		allow[strings.ToLower(strings.TrimSpace(p))] = struct{}{}
	}
	return &sshAuthority{keys: keys, allow: allow}, nil
}

// parseAllowedSigners reads an OpenSSH allowed_signers file into a map of
// marshalled public key to its principals. Each line is
// "principals[,principal...] [options] keytype base64 [comment]".
func parseAllowedSigners(data []byte) map[string][]string {
	keys := map[string][]string{}
	for raw := range strings.SplitSeq(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexAny(line, " \t")
		if i <= 0 {
			continue
		}
		principals := strings.Split(line[:i], ",")
		pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(line[i:])))
		if err != nil {
			continue
		}
		k := string(pub.Marshal())
		for _, p := range principals {
			p = strings.ToLower(strings.TrimSpace(p))
			if p != "" {
				keys[k] = append(keys[k], p)
			}
		}
	}
	return keys
}

// Verify implements Authority. It fails closed and never returns an error.
func (a *sshAuthority) Verify(_ context.Context, commit CommitData) CommitResult {
	res := CommitResult{SHA: commit.SHA}
	if len(commit.Signature) == 0 {
		res.Reason = ReasonUnsigned
		return res
	}
	if !strings.Contains(string(commit.Signature), sshSignatureMarker) {
		res.Reason = "not an SSH signature"
		return res
	}

	blob, pub, reason := parseSSHSig(commit.Signature)
	if reason != "" {
		res.Reason = reason
		return res
	}

	principals, trusted := a.keys[string(pub.Marshal())]
	if !trusted {
		res.Reason = "SSH signing key is not in the trusted allowed_signers set"
		return res
	}
	res.Issuer = "ssh:" + ssh.FingerprintSHA256(pub)

	if err := verifySSHSignature(blob, pub, commit.Payload); err != nil {
		res.Reason = "SSH signature did not verify"
		return res
	}

	principal, ok := a.allowedPrincipal(principals)
	if !ok {
		res.SignerSAN = principals[0]
		res.Reason = NotAllowlistedReason(principals[0], res.Issuer)
		return res
	}
	res.OK = true
	res.Authority = AuthoritySSH
	res.SignerSAN = principal
	return res
}

// parseSSHSig decodes and parses an armored SSHSIG blob, returning a non-empty
// reason string on failure.
func parseSSHSig(signature []byte) (sshSigBlob, ssh.PublicKey, string) {
	blob, err := decodeSSHSig(string(signature))
	if err != nil || len(blob) < len(sshMagic) || string(blob[:len(sshMagic)]) != sshMagic {
		return sshSigBlob{}, nil, "malformed SSH signature"
	}
	var parsed sshSigBlob
	if err := ssh.Unmarshal(blob[len(sshMagic):], &parsed); err != nil {
		return sshSigBlob{}, nil, "malformed SSH signature"
	}
	if parsed.Namespace != sshGitNamespace {
		return sshSigBlob{}, nil, "SSH signature is not a git signature"
	}
	pub, err := ssh.ParsePublicKey(parsed.PublicKey)
	if err != nil {
		return sshSigBlob{}, nil, "malformed SSH signature key"
	}
	return parsed, pub, ""
}

// allowedPrincipal picks a principal for the signing key that passes the
// allowlist. With no allowlist, the key's first principal is used.
func (a *sshAuthority) allowedPrincipal(principals []string) (string, bool) {
	if len(a.allow) == 0 {
		if len(principals) > 0 {
			return principals[0], true
		}
		return "", false
	}
	for _, p := range principals {
		if _, ok := a.allow[strings.ToLower(p)]; ok {
			return p, true
		}
	}
	return "", false
}

// verifySSHSignature reconstructs the SSHSIG signed data and checks it against
// the public key, matching what ssh-keygen -Y verify does.
func verifySSHSignature(blob sshSigBlob, pub ssh.PublicKey, payload []byte) error {
	hash, ok := sshHash(blob.HashAlgo, payload)
	if !ok {
		return fmt.Errorf("unsupported hash %q", blob.HashAlgo)
	}
	signed := append([]byte(sshMagic), ssh.Marshal(sshSignedData{
		Namespace: blob.Namespace,
		Reserved:  blob.Reserved,
		HashAlgo:  blob.HashAlgo,
		Hash:      hash,
	})...)
	var inner ssh.Signature
	if err := ssh.Unmarshal(blob.Signature, &inner); err != nil {
		return fmt.Errorf("parse inner signature: %w", err)
	}
	if err := pub.Verify(signed, &inner); err != nil {
		return fmt.Errorf("verify ssh signature: %w", err)
	}
	return nil
}

// sshHash hashes data with the algorithm named in the signature.
func sshHash(algo string, data []byte) ([]byte, bool) {
	switch algo {
	case sshHashSHA256:
		sum := sha256.Sum256(data)
		return sum[:], true
	case sshHashSHA512:
		sum := sha512.Sum512(data)
		return sum[:], true
	default:
		return nil, false
	}
}

// decodeSSHSig strips the SSHSIG armor and base64-decodes the body.
func decodeSSHSig(armored string) ([]byte, error) {
	const begin = "-----BEGIN SSH SIGNATURE-----"
	const end = "-----END SSH SIGNATURE-----"
	_, body, ok := strings.Cut(armored, begin)
	if !ok {
		return nil, errors.New("missing SSH signature header")
	}
	inner, _, ok := strings.Cut(body, end)
	if !ok {
		return nil, errors.New("missing SSH signature footer")
	}
	var b strings.Builder
	for _, r := range inner {
		if r != '\n' && r != '\r' && r != ' ' && r != '\t' {
			b.WriteRune(r)
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(b.String())
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	return decoded, nil
}
