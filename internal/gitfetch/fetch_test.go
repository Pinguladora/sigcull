package gitfetch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScrub(t *testing.T) {
	err := errors.New("fatal: auth failed for ghs_secret123 at github.com")
	got := scrub(err, "ghs_secret123")
	if strings.Contains(got.Error(), "ghs_secret123") {
		t.Fatalf("scrub leaked the token: %q", got)
	}
	if !strings.Contains(got.Error(), "***") {
		t.Fatalf("scrub did not mask the token: %q", got)
	}
}

// gitRepo builds a small repo on disk (git only for setup) that go-git can fetch
// from over the local file transport, and returns its URL plus commit SHAs
// oldest-first.
func gitRepo(t *testing.T) (url string, shas []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available for test setup")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--quiet", "-b", "main")
	run("config", "uploadpack.allowAnySHA1InWant", "true")
	run("config", "commit.gpgsign", "false")
	for i := range 3 {
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte{byte('0' + i)}, 0o600); err != nil {
			t.Fatal(err)
		}
		run("add", "f")
		run("commit", "--quiet", "--no-gpg-sign", "-m", "c")
		shas = append(shas, run("rev-parse", "HEAD"))
	}
	return "file://" + dir, shas
}

func TestFetchRangeInMemory(t *testing.T) {
	url, shas := gitRepo(t)
	base, head := shas[0], shas[2]

	res, err := Fetch(context.Background(), url, "", Range{Base: base, Head: head})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// base..head excludes base, so the two commits after it, oldest first.
	if len(res.Commits) != 2 {
		t.Fatalf("got %d commits, want 2: %+v", len(res.Commits), res.Commits)
	}
	if res.Commits[0].SHA != shas[1] || res.Commits[1].SHA != shas[2] {
		t.Errorf("range order wrong: %s, %s", res.Commits[0].SHA, res.Commits[1].SHA)
	}
	if res.Truncated {
		t.Error("Truncated should be false for a reachable base")
	}
	// Commit data is populated in-memory.
	if len(res.Commits[0].Payload) == 0 {
		t.Error("commit payload should be non-empty")
	}
	if res.Commits[0].CommitterEmail != "t@example.com" {
		t.Errorf("committer email = %q", res.Commits[0].CommitterEmail)
	}
	if res.AnchorPath != "f" {
		t.Errorf("anchor path = %q, want f", res.AnchorPath)
	}
}

func TestFetchNewBranchExcludesExistingBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available for test setup")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(content string) string {
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		run("add", "f")
		run("commit", "--quiet", "--no-gpg-sign", "-m", content)
		return run("rev-parse", "HEAD")
	}
	run("init", "--quiet", "-b", "main")
	run("config", "uploadpack.allowAnySHA1InWant", "true")
	run("config", "commit.gpgsign", "false")
	commit("m0")
	commit("m1") // two commits on main
	run("checkout", "--quiet", "-b", "feature")
	f0 := commit("f0")
	f1 := commit("f1") // two new commits on feature

	// Branch-creation push of feature: base is the zero SHA.
	res, err := Fetch(context.Background(), "file://"+dir, "", Range{Base: zeroSHA, Head: f1})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// Only feature's two new commits, not main's, oldest first.
	if len(res.Commits) != 2 {
		shas := make([]string, len(res.Commits))
		for i, c := range res.Commits {
			shas[i] = c.SHA
		}
		t.Fatalf("got %d commits, want 2 (only new): %v", len(res.Commits), shas)
	}
	if res.Commits[0].SHA != f0 || res.Commits[1].SHA != f1 {
		t.Errorf("wrong new commits: %s, %s (want %s, %s)", res.Commits[0].SHA, res.Commits[1].SHA, f0, f1)
	}
}

func TestFetchZeroBaseHeadOnly(t *testing.T) {
	url, shas := gitRepo(t)
	res, err := Fetch(context.Background(), url, "", Range{Base: zeroSHA, Head: shas[2]})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Truncated {
		t.Error("zero base is branch creation, not truncation")
	}
	if len(res.Commits) == 0 {
		t.Fatal("expected head-only commits")
	}
	// Head-only window includes all reachable commits, oldest first.
	if res.Commits[len(res.Commits)-1].SHA != shas[2] {
		t.Errorf("head should be last: %s", res.Commits[len(res.Commits)-1].SHA)
	}
}
