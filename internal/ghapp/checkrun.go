package ghapp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Pinguladora/sigcull/internal/verify"
	"github.com/google/go-github/v90/github"
)

// checkRunName is the Check Run name operators pin as a required status check in
// a ruleset or branch protection.
const checkRunName = "sigcull"

// maxAnnotations is GitHub's per-request cap on Check Run annotations.
const maxAnnotations = 50

// shortSHALen is how many leading hex chars of a commit SHA we show.
const shortSHALen = 8

// CheckRun wraps the go-github checks client for the create/complete lifecycle.
type CheckRun struct {
	client         *github.Client
	owner          string
	repo           string
	maskIdentities bool
}

// NewCheckRun binds a check-run helper to one repository. When maskIdentities is
// true, signer email addresses are masked in the (potentially public) Check Run
// output; the full identity still goes to the app logs.
func NewCheckRun(client *github.Client, owner, repo string, maskIdentities bool) *CheckRun {
	return &CheckRun{client: client, owner: owner, repo: repo, maskIdentities: maskIdentities}
}

// emailRe matches an email address, used to mask signer identities in output.
var emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// maskEmails masks the local part of every email in s (a***@example.com). URI
// identities (CI/workflow SANs) contain no email and pass through unchanged.
func maskEmails(s string, mask bool) string {
	if !mask {
		return s
	}
	masked := emailRe.ReplaceAllStringFunc(s, func(addr string) string {
		at := strings.IndexByte(addr, '@')
		if at <= 0 {
			return addr
		}
		return addr[:1] + "***" + addr[at:]
	})
	return masked
}

// Create opens an in_progress Check Run for headSHA and returns its ID. It is
// idempotent: if a run of this name already exists for the SHA (a webhook
// redelivery, or a push and a pull_request for the same commit), it reuses that
// run instead of creating a duplicate.
func (c *CheckRun) Create(ctx context.Context, headSHA string) (int64, error) {
	existing, _, err := c.client.Checks.ListCheckRunsForRef(ctx, c.owner, c.repo, headSHA,
		&github.ListCheckRunsOptions{CheckName: new(checkRunName)})
	if err == nil && existing.GetTotal() > 0 {
		id := existing.CheckRuns[0].GetID()
		_, _, err := c.client.Checks.UpdateCheckRun(ctx, c.owner, c.repo, id, github.UpdateCheckRunOptions{
			Name:   checkRunName,
			Status: new("in_progress"),
		})
		if err != nil {
			return 0, fmt.Errorf("reset existing check run: %w", err)
		}
		return id, nil
	}

	run, _, err := c.client.Checks.CreateCheckRun(ctx, c.owner, c.repo, github.CreateCheckRunOptions{
		Name:      checkRunName,
		HeadSHA:   headSHA,
		Status:    new("in_progress"),
		StartedAt: &github.Timestamp{Time: time.Now()},
	})
	if err != nil {
		return 0, fmt.Errorf("create check run: %w", err)
	}
	return run.GetID(), nil
}

// Complete finalises the Check Run. The conclusion is success only when every
// result is OK. The summary always names every offending SHA and reason. When
// anchorPath is non-empty, failing commits also get a failure annotation
// anchored to that path. GitHub requires an existing path, so an empty
// anchorPath means annotations are omitted and the summary carries the detail.
// If the fetch was truncated the summary says so.
func (c *CheckRun) Complete(
	ctx context.Context,
	id int64,
	results []verify.CommitResult,
	truncated bool,
	anchorPath string,
) error {
	conclusion := "success"
	var failed int
	for _, r := range results {
		if !r.OK {
			conclusion = "failure"
			failed++
		}
	}

	summary := buildSummary(results, failed, truncated, c.maskIdentities)
	annotations := buildAnnotations(results, anchorPath, c.maskIdentities)

	// First update carries the conclusion plus the first batch of annotations.
	first, rest := splitAnnotations(annotations)
	_, _, err := c.client.Checks.UpdateCheckRun(ctx, c.owner, c.repo, id, github.UpdateCheckRunOptions{
		Name:        checkRunName,
		Status:      new("completed"),
		Conclusion:  new(conclusion),
		CompletedAt: &github.Timestamp{Time: time.Now()},
		Output: &github.CheckRunOutput{
			Title:       new(checkRunTitle(conclusion, failed, len(results))),
			Summary:     new(summary),
			Annotations: first,
		},
	})
	if err != nil {
		return fmt.Errorf("complete check run: %w", err)
	}

	// Any annotations beyond the first 50 go in follow-up updates.
	for len(rest) > 0 {
		var batch []*github.CheckRunAnnotation
		batch, rest = splitAnnotations(rest)
		if _, _, err := c.client.Checks.UpdateCheckRun(ctx, c.owner, c.repo, id, github.UpdateCheckRunOptions{
			Name: checkRunName,
			Output: &github.CheckRunOutput{
				Title:       new(checkRunTitle(conclusion, failed, len(results))),
				Summary:     new(summary),
				Annotations: batch,
			},
		}); err != nil {
			return fmt.Errorf("append annotations: %w", err)
		}
	}
	return nil
}

func checkRunTitle(conclusion string, failed, total int) string {
	if conclusion == "success" {
		return fmt.Sprintf("All %d commit(s) verified", total)
	}
	return fmt.Sprintf("%d of %d commit(s) failed verification", failed, total)
}

func buildSummary(results []verify.CommitResult, failed int, truncated, mask bool) string {
	var b strings.Builder
	if failed == 0 {
		fmt.Fprintf(&b, "All %d commit(s) satisfy the gitsign identity policy.\n\n", len(results))
	} else {
		fmt.Fprintf(&b, "%d of %d commit(s) failed gitsign verification.\n\n", failed, len(results))
	}
	if truncated {
		b.WriteString("> Note: history was rewritten or the base commit was unreachable, ")
		b.WriteString("so only the most recent commits in range were verified.\n\n")
	}
	b.WriteString("| Commit | Result | Identity | Issuer | Detail |\n|---|---|---|---|---|\n")
	for _, r := range results {
		status, detail := "pass", ""
		switch {
		case !r.OK && r.SignerSAN != "":
			// Valid signature, identity rejected. The Identity and Issuer
			// columns already name the signer, so keep the detail short.
			status, detail = "fail", "not on the allowlist"
		case !r.OK:
			status, detail = "fail", r.Reason
		case r.Authority == verify.AuthorityExempt:
			status, detail = "skip", r.Reason
		default:
			detail = passDetail(r)
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n",
			short(r.SHA), status,
			sanitiseCell(maskEmails(r.SignerSAN, mask)), sanitiseCell(r.Issuer), sanitiseCell(maskEmails(detail, mask)))
	}
	return b.String()
}

// passDetail annotates a verified commit with the authority that accepted it,
// so a non-keyless pass is not silently indistinguishable from a keyless one.
// Keyless passes keep an empty detail, unchanged.
func passDetail(r verify.CommitResult) string {
	switch r.Authority {
	case verify.AuthorityGPG:
		return "verified by a trusted OpenPGP key"
	case verify.AuthoritySSH:
		return "verified by a trusted SSH key"
	case verify.AuthorityX509:
		return "verified by a trusted X.509 certificate"
	case verify.AuthorityGitHub:
		return "verified by GitHub"
	default:
		return ""
	}
}

func buildAnnotations(results []verify.CommitResult, anchorPath string, mask bool) []*github.CheckRunAnnotation {
	if anchorPath == "" {
		return nil // no valid path to anchor to, the summary still names offenders
	}
	var out []*github.CheckRunAnnotation
	for _, r := range results {
		if r.OK {
			continue
		}
		out = append(out, &github.CheckRunAnnotation{
			Path:            new(anchorPath),
			StartLine:       new(1),
			EndLine:         new(1),
			AnnotationLevel: new("failure"),
			Title:           new("Unverified commit " + short(r.SHA)),
			Message:         new(maskEmails(fmt.Sprintf("%s: %s", r.SHA, r.Reason), mask)),
		})
	}
	return out
}

// splitAnnotations peels off up to maxAnnotations from the front.
func splitAnnotations(all []*github.CheckRunAnnotation) (batch, rest []*github.CheckRunAnnotation) {
	if len(all) <= maxAnnotations {
		return all, nil
	}
	return all[:maxAnnotations], all[maxAnnotations:]
}

func short(sha string) string {
	if len(sha) > shortSHALen {
		return sha[:shortSHALen]
	}
	return sha
}

// sanitiseCell keeps a markdown table cell on one line.
func sanitiseCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
