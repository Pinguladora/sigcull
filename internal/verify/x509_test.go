package verify

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
)

// testPKI is a throwaway CA plus a leaf S-MIME signing certificate.
type testPKI struct {
	leaf    *x509.Certificate
	leafKey *rsa.PrivateKey
	caPEM   []byte
}

// x509Email is the signer email newTestPKI issues its leaf certificate for.
const x509Email = "dev@example.com"

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	now := time.Now()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber:   big.NewInt(2),
		Subject:        pkix.Name{CommonName: x509Email},
		EmailAddresses: []string{x509Email},
		NotBefore:      now.Add(-time.Hour),
		NotAfter:       now.Add(time.Hour),
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	return testPKI{caPEM: caPEM, leaf: leaf, leafKey: leafKey}
}

// signCMS returns a detached, PEM-armored CMS signature over payload.
func signCMS(t *testing.T, pki testPKI, payload []byte) []byte {
	t.Helper()
	sd, err := pkcs7.NewSignedData(payload)
	if err != nil {
		t.Fatalf("new signed data: %v", err)
	}
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)
	if err := sd.AddSigner(pki.leaf, pki.leafKey, pkcs7.SignerInfoConfig{}); err != nil {
		t.Fatalf("add signer: %v", err)
	}
	sd.Detach()
	der, err := sd.Finish()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "SIGNED MESSAGE", Bytes: der})
}

func writeCABundle(t *testing.T, pemBytes []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("write ca bundle: %v", err)
	}
	return path
}

func TestX509AuthorityAccepts(t *testing.T) {
	pki := newTestPKI(t)
	auth, err := NewX509(writeCABundle(t, pki.caPEM), []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewX509: %v", err)
	}
	payload := []byte("tree abc\n\ncommit\n")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c1", Payload: payload, Signature: signCMS(t, pki, payload),
	})
	if !res.OK {
		t.Fatalf("expected acceptance, got %+v", res)
	}
	if res.Authority != AuthorityX509 {
		t.Errorf("Authority = %q, want %q", res.Authority, AuthorityX509)
	}
	if res.SignerSAN != "dev@example.com" {
		t.Errorf("SignerSAN = %q, want dev@example.com", res.SignerSAN)
	}
}

func TestX509AuthorityRejectsUntrustedCA(t *testing.T) {
	pki := newTestPKI(t)
	other := newTestPKI(t)
	// Trust the other CA, but the commit is signed under pki's leaf.
	auth, err := NewX509(writeCABundle(t, other.caPEM), []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewX509: %v", err)
	}
	payload := []byte("payload")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c2", Payload: payload, Signature: signCMS(t, pki, payload),
	})
	if res.OK {
		t.Fatalf("expected rejection for a cert not chaining to the bundle, got %+v", res)
	}
}

func TestX509AuthorityRejectsTamperedPayload(t *testing.T) {
	pki := newTestPKI(t)
	auth, err := NewX509(writeCABundle(t, pki.caPEM), []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewX509: %v", err)
	}
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c3", Payload: []byte("tampered"), Signature: signCMS(t, pki, []byte("original")),
	})
	if res.OK {
		t.Fatalf("expected rejection for a tampered payload, got %+v", res)
	}
}

func TestX509AuthorityRejectsUnlistedEmail(t *testing.T) {
	pki := newTestPKI(t)
	auth, err := NewX509(writeCABundle(t, pki.caPEM), []string{"someone-else@example.com"})
	if err != nil {
		t.Fatalf("NewX509: %v", err)
	}
	payload := []byte("payload")
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c4", Payload: payload, Signature: signCMS(t, pki, payload),
	})
	if res.OK {
		t.Fatalf("expected rejection for an unlisted signer email, got %+v", res)
	}
	if res.SignerSAN != "dev@example.com" {
		t.Errorf("rejected result should still name the signer, got %q", res.SignerSAN)
	}
}

func TestX509AuthorityDeclinesNonCMS(t *testing.T) {
	pki := newTestPKI(t)
	auth, err := NewX509(writeCABundle(t, pki.caPEM), []string{"dev@example.com"})
	if err != nil {
		t.Fatalf("NewX509: %v", err)
	}
	res := auth.Verify(context.Background(), CommitData{
		SHA: "c5", Payload: []byte("x"), Signature: []byte("not a pem block"),
	})
	if res.OK {
		t.Fatalf("expected a non-CMS signature to be declined, got %+v", res)
	}
}
