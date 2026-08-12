package verify

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func newSSHSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return s
}

// sshPrincipal is the principal writeAllowedSigners trusts.
const sshPrincipal = "dev@example.com"

// writeAllowedSigners writes a one-line allowed_signers file for sshPrincipal
// and returns its path.
func writeAllowedSigners(t *testing.T, pub ssh.PublicKey) string {
	t.Helper()
	line := sshPrincipal + " " + string(ssh.MarshalAuthorizedKey(pub))
	path := filepath.Join(t.TempDir(), "allowed_signers")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("write allowed_signers: %v", err)
	}
	return path
}

// signSSHSig produces an armored SSHSIG over payload, mirroring ssh-keygen -Y sign.
func signSSHSig(t *testing.T, signer ssh.Signer, payload []byte, namespace string) []byte {
	t.Helper()
	sum := sha512.Sum512(payload)
	signed := append([]byte(sshMagic), ssh.Marshal(sshSignedData{
		Namespace: namespace, HashAlgo: sshHashSHA512, Hash: sum[:],
	})...)
	sig, err := signer.Sign(rand.Reader, signed)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	blob := append([]byte(sshMagic), ssh.Marshal(sshSigBlob{
		Version:   1,
		PublicKey: signer.PublicKey().Marshal(),
		Namespace: namespace,
		HashAlgo:  sshHashSHA512,
		Signature: ssh.Marshal(*sig),
	})...)
	armored := "-----BEGIN SSH SIGNATURE-----\n" +
		base64.StdEncoding.EncodeToString(blob) + "\n-----END SSH SIGNATURE-----\n"
	return []byte(armored)
}

func TestSSHAuthorityAccepts(t *testing.T) {
	signer := newSSHSigner(t)
	path := writeAllowedSigners(t, signer.PublicKey())
	auth, err := NewSSHKey(path, []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewSSHKey: %v", err)
	}
	payload := []byte("tree abc\n\ncommit\n")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c1", Payload: payload, Signature: signSSHSig(t, signer, payload, "git"),
	})
	if !res.OK {
		t.Fatalf("expected acceptance, got %+v", res)
	}
	if res.Authority != AuthoritySSH {
		t.Errorf("Authority = %q, want %q", res.Authority, AuthoritySSH)
	}
	if res.SignerSAN != "dev@example.com" {
		t.Errorf("SignerSAN = %q, want dev@example.com", res.SignerSAN)
	}
}

func TestSSHAuthorityRejectsUntrustedKey(t *testing.T) {
	trusted := newSSHSigner(t)
	path := writeAllowedSigners(t, trusted.PublicKey())
	auth, err := NewSSHKey(path, nil)
	if err != nil {
		t.Fatalf("NewSSHKey: %v", err)
	}
	payload := []byte("payload")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c2", Payload: payload, Signature: signSSHSig(t, newSSHSigner(t), payload, "git"),
	})
	if res.OK {
		t.Fatalf("expected rejection for an untrusted key, got %+v", res)
	}
}

func TestSSHAuthorityRejectsTamperedPayload(t *testing.T) {
	signer := newSSHSigner(t)
	path := writeAllowedSigners(t, signer.PublicKey())
	auth, err := NewSSHKey(path, nil)
	if err != nil {
		t.Fatalf("NewSSHKey: %v", err)
	}
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c3", Payload: []byte("tampered"), Signature: signSSHSig(t, signer, []byte("original"), "git"),
	})
	if res.OK {
		t.Fatalf("expected rejection for a tampered payload, got %+v", res)
	}
}

func TestSSHAuthorityRejectsUnlistedPrincipal(t *testing.T) {
	signer := newSSHSigner(t)
	path := writeAllowedSigners(t, signer.PublicKey())
	auth, err := NewSSHKey(path, []string{"someone-else@example.com"})
	if err != nil {
		t.Fatalf("NewSSHKey: %v", err)
	}
	payload := []byte("payload")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c4", Payload: payload, Signature: signSSHSig(t, signer, payload, "git"),
	})
	if res.OK {
		t.Fatalf("expected rejection for an unlisted principal, got %+v", res)
	}
}

func TestSSHAuthorityRejectsNonGitNamespace(t *testing.T) {
	signer := newSSHSigner(t)
	path := writeAllowedSigners(t, signer.PublicKey())
	auth, err := NewSSHKey(path, nil)
	if err != nil {
		t.Fatalf("NewSSHKey: %v", err)
	}
	payload := []byte("payload")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c5", Payload: payload, Signature: signSSHSig(t, signer, payload, "file"),
	})
	if res.OK {
		t.Fatalf("expected rejection for a non-git namespace, got %+v", res)
	}
}

func TestSSHAuthorityDeclinesNonSSH(t *testing.T) {
	signer := newSSHSigner(t)
	path := writeAllowedSigners(t, signer.PublicKey())
	auth, err := NewSSHKey(path, nil)
	if err != nil {
		t.Fatalf("NewSSHKey: %v", err)
	}
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c6", Payload: []byte("x"), Signature: []byte("-----BEGIN PGP SIGNATURE-----\n..."),
	})
	if res.OK {
		t.Fatalf("expected a non-SSH signature to be declined, got %+v", res)
	}
}
