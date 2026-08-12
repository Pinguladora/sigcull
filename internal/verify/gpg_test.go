package verify

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// testKeyEmail is the UID email carried by the key newTestKey generates.
const testKeyEmail = "dev@example.com"

// newTestKey generates an OpenPGP key and writes its armored public half to a
// keyring file, returning the signer and the path.
func newTestKey(t *testing.T) (*openpgp.Entity, string) {
	t.Helper()
	entity, err := openpgp.NewEntity("Test Signer", "", testKeyEmail, nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatalf("armor encode: %v", err)
	}
	if err := entity.Serialize(w); err != nil {
		t.Fatalf("serialize public key: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close armor: %v", err)
	}

	path := filepath.Join(t.TempDir(), "trusted-keys.asc")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write keyring: %v", err)
	}
	return entity, path
}

// detachSign returns an armored detached signature over payload.
func detachSign(t *testing.T, signer *openpgp.Entity, payload []byte) []byte {
	t.Helper()
	var sig bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&sig, signer, bytes.NewReader(payload), nil); err != nil {
		t.Fatalf("detach sign: %v", err)
	}
	return sig.Bytes()
}

func TestGPGAuthorityAccepts(t *testing.T) {
	signer, keyring := newTestKey(t)
	auth, err := NewGPGKey(keyring, []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewGPGKey: %v", err)
	}

	payload := []byte("tree abc\nauthor Dev <dev@example.com> 1 +0000\n\nmessage\n")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "commit1", Payload: payload, Signature: detachSign(t, signer, payload),
	})
	if !res.OK {
		t.Fatalf("expected acceptance, got %+v", res)
	}
	if res.Authority != AuthorityGPG {
		t.Errorf("Authority = %q, want %q", res.Authority, AuthorityGPG)
	}
	if res.SignerSAN != "dev@example.com" {
		t.Errorf("SignerSAN = %q, want dev@example.com", res.SignerSAN)
	}
}

func TestGPGAuthorityRejectsUnlistedEmail(t *testing.T) {
	signer, keyring := newTestKey(t)
	// Trust the key, but only accept a different UID email.
	auth, err := NewGPGKey(keyring, []string{"someone-else@example.com"})
	if err != nil {
		t.Fatalf("NewGPGKey: %v", err)
	}

	payload := []byte("payload\n")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "commit2", Payload: payload, Signature: detachSign(t, signer, payload),
	})
	if res.OK {
		t.Fatalf("expected rejection for an unlisted UID email, got %+v", res)
	}
	if res.SignerSAN != "dev@example.com" {
		t.Errorf("rejected result should still name the signer, got %q", res.SignerSAN)
	}
}

func TestGPGAuthorityRejectsUntrustedKey(t *testing.T) {
	_, keyring := newTestKey(t)
	auth, err := NewGPGKey(keyring, []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewGPGKey: %v", err)
	}

	// Sign with a different key that is not in the trusted keyring.
	attacker, err := openpgp.NewEntity("Attacker", "", "dev@example.com", nil)
	if err != nil {
		t.Fatalf("generate attacker key: %v", err)
	}
	payload := []byte("payload\n")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "commit3", Payload: payload, Signature: detachSign(t, attacker, payload),
	})
	if res.OK {
		t.Fatalf("expected rejection for an untrusted signing key, got %+v", res)
	}
}

func TestGPGAuthorityRejectsTamperedPayload(t *testing.T) {
	signer, keyring := newTestKey(t)
	auth, err := NewGPGKey(keyring, []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewGPGKey: %v", err)
	}

	sig := detachSign(t, signer, []byte("original payload\n"))
	res := auth.Verify(context.Background(), CommitData{
		SHA: "commit4", Payload: []byte("tampered payload\n"), Signature: sig,
	})
	if res.OK {
		t.Fatalf("expected rejection for a tampered payload, got %+v", res)
	}
}

func TestGPGAuthorityDeclinesNonPGP(t *testing.T) {
	_, keyring := newTestKey(t)
	auth, err := NewGPGKey(keyring, []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewGPGKey: %v", err)
	}

	// A Sigstore CMS blob is not an OpenPGP signature; this authority declines.
	res := auth.Verify(context.Background(), CommitData{
		SHA: "commit5", Payload: []byte("x"), Signature: []byte("-----BEGIN SIGNED MESSAGE-----\n..."),
	})
	if res.OK {
		t.Fatalf("expected non-PGP signature to be declined, got %+v", res)
	}
}
