package ghapp

import (
	"strings"
	"testing"

	"github.com/Pinguladora/sigcull/internal/verify"
)

func TestBuildAnnotations(t *testing.T) {
	results := []verify.CommitResult{
		{SHA: "aaaa1111", OK: true},
		{SHA: "bbbb2222", OK: false, Reason: "unsigned"},
		{SHA: "cccc3333", OK: false, Reason: "not allowlisted"},
	}

	// No anchor path means no annotations, but the summary still names offenders.
	if got := buildAnnotations(results, "", true); got != nil {
		t.Fatalf("expected no annotations without an anchor path, got %d", len(got))
	}

	got := buildAnnotations(results, "cmd/main.go", true)
	if len(got) != 2 {
		t.Fatalf("expected 2 failure annotations, got %d", len(got))
	}
	for _, a := range got {
		if a.GetPath() != "cmd/main.go" {
			t.Errorf("annotation path = %q, want cmd/main.go", a.GetPath())
		}
		if a.GetAnnotationLevel() != "failure" {
			t.Errorf("annotation level = %q, want failure", a.GetAnnotationLevel())
		}
	}
}

func TestBuildSummaryNamesOffenders(t *testing.T) {
	results := []verify.CommitResult{
		{SHA: "bbbb2222", OK: false, Reason: "unsigned commit"},
	}
	summary := buildSummary(results, 1, true, true)
	if !strings.Contains(summary, "bbbb2222") || !strings.Contains(summary, "unsigned commit") {
		t.Fatalf("summary must name the offending SHA and reason: %s", summary)
	}
	if !strings.Contains(summary, "history was rewritten") {
		t.Fatalf("summary must note truncation: %s", summary)
	}
}

func TestBuildSummaryMasksEmails(t *testing.T) {
	results := []verify.CommitResult{
		{SHA: "aaaa1111", OK: true, SignerSAN: "someone@example.com", Issuer: "https://accounts.google.com"},
	}
	masked := buildSummary(results, 0, false, true)
	if strings.Contains(masked, "someone@example.com") {
		t.Fatalf("masked summary must not contain the full email: %s", masked)
	}
	if !strings.Contains(masked, "s***@example.com") {
		t.Fatalf("masked summary should show a masked email: %s", masked)
	}
	// With masking off, the full identity is shown.
	full := buildSummary(results, 0, false, false)
	if !strings.Contains(full, "someone@example.com") {
		t.Fatalf("unmasked summary should show the full email: %s", full)
	}
}

func TestBuildSummaryAuthorityDetail(t *testing.T) {
	results := []verify.CommitResult{
		{SHA: "aaaa1111", OK: true, Authority: verify.AuthorityKeyless, SignerSAN: "me@example.com", Issuer: "https://accounts.google.com"},
		{SHA: "bbbb2222", OK: true, Authority: verify.AuthorityGitHub},
		{SHA: "cccc3333", OK: true, Authority: verify.AuthorityGPG, SignerSAN: "dev@example.com", Issuer: "gpg:ABCDEF0123456789"},
		{SHA: "dddd4444", OK: true, Authority: verify.AuthorityExempt, Reason: "skipped: merge commit (exempt)"},
	}
	summary := buildSummary(results, 0, false, false)

	// A github pass names the authority in the detail column.
	if !strings.Contains(summary, "verified by GitHub") {
		t.Errorf("github pass should note the authority: %s", summary)
	}
	// A gpg pass names the authority too.
	if !strings.Contains(summary, "verified by a trusted OpenPGP key") {
		t.Errorf("gpg pass should note the authority: %s", summary)
	}
	// An exemption renders as skip with its reason, not as a plain pass.
	if !strings.Contains(summary, "| skip |") || !strings.Contains(summary, "merge commit") {
		t.Errorf("exempt commit should render as skip with its reason: %s", summary)
	}
	// A keyless pass carries no extra detail beyond its identity columns.
	if strings.Contains(summary, "verified by keyless") {
		t.Errorf("keyless pass should not add a redundant detail: %s", summary)
	}
}

func TestBuildSummaryColumnsAndCasing(t *testing.T) {
	results := []verify.CommitResult{
		{SHA: "aaaa1111", OK: true, SignerSAN: "me@example.com", Issuer: "https://accounts.google.com"},
		{SHA: "bbbb2222", OK: false, SignerSAN: "evil@example.com", Issuer: "https://accounts.google.com", Reason: "signed by evil@example.com (...) but not on the allowlist"},
		{SHA: "cccc3333", OK: false, Reason: "no valid Sigstore signature"},
	}
	summary := buildSummary(results, 2, false, false)

	// Header has the split columns.
	if !strings.Contains(summary, "| Commit | Result | Identity | Issuer | Detail |") {
		t.Fatalf("missing split-column header: %s", summary)
	}
	// A valid signer that is rejected shows its identity/issuer and a short detail.
	if !strings.Contains(summary, "evil@example.com") || !strings.Contains(summary, "not on the allowlist") {
		t.Fatalf("non-allowlisted row should name the signer with a short detail: %s", summary)
	}
	// Result casing is consistent: no uppercase FAIL.
	if strings.Contains(summary, "FAIL") {
		t.Fatalf("result column casing should be consistent (lowercase): %s", summary)
	}
	if !strings.Contains(summary, "| pass |") || !strings.Contains(summary, "| fail |") {
		t.Fatalf("expected lowercase pass/fail results: %s", summary)
	}
}
