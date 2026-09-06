package verify

import (
	"context"
	_ "embed"
	"testing"
	"time"

	"github.com/Pinguladora/sigcull/internal/policy"
)

// A real commit signed in gitsign's online Rekor mode: a valid CMS signature and
// Fulcio chain, but no Rekor inclusion proof embedded in the signature.
//
//go:embed testdata/online_no_proof.payload
var onlineNoProofPayload []byte

//go:embed testdata/online_no_proof.sig
var onlineNoProofSig []byte

// Verification is strictly offline, so a signature that embeds no inclusion proof
// must be rejected with ReasonNoTransparencyProof and must never reach the
// network. The Rekor client points at an unreachable address on purpose: a fast
// return proves the online-search fallback is gone.
func TestKeylessRejectsSignatureWithoutEmbeddedProof(t *testing.T) {
	v, err := NewKeyless(context.Background(), policy.Policy{}, TrustParams{Kind: Public, RekorURL: "http://127.0.0.1:9"})
	if err != nil {
		t.Fatalf("new keyless: %v", err)
	}
	cd := CommitData{SHA: "test", Payload: onlineNoProofPayload, Signature: onlineNoProofSig}

	start := time.Now()
	res := v.Verify(context.Background(), cd)
	elapsed := time.Since(start)

	if res.OK {
		t.Fatal("expected rejection, got OK=true")
	}
	if res.Reason != ReasonNoTransparencyProof {
		t.Fatalf("reason = %q, want %q", res.Reason, ReasonNoTransparencyProof)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("verification took %s, so it reached the network instead of failing offline", elapsed)
	}
}
