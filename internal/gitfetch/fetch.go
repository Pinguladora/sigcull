// Package gitfetch shallow-fetches a commit range entirely in memory using
// go-git, and returns the per-commit data the verifier needs. No temp dir, no
// git subprocess: everything runs in-process against an in-memory object store.
package gitfetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Pinguladora/sigcull/internal/verify"
	"github.com/go-git/go-billy/v5/memfs"
	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// defaultDepth bounds the shallow fetch. A range deeper than this is reported as
// truncated rather than fetched unbounded.
const defaultDepth = 50

// zeroSHA is git's all-zero object id (a created or deleted ref).
const zeroSHA = "0000000000000000000000000000000000000000"

// remoteName is the single remote the in-memory repo fetches from.
const remoteName = "origin"

// wantRefSpec fetches a specific SHA into a private ref.
func wantRefSpec(sha string) gogitconfig.RefSpec {
	spec := gogitconfig.RefSpec(sha + ":refs/sigcull/" + sha)
	return spec
}

// Range is the span of commits to verify. Base may be the zero SHA or
// unreachable. Head is always required.
type Range struct {
	Base string
	Head string
}

// Result is the fetched range as verifier-ready commit data.
type Result struct {
	AnchorPath string
	Commits    []verify.CommitData
	Truncated  bool
}

// Fetch fetches the commit range from cloneURL in memory and returns each
// commit's verification data. The installation token is passed via HTTP basic
// auth (not in the URL), so it cannot leak through a URL. token may be empty for
// unauthenticated remotes (used in tests).
func Fetch(ctx context.Context, cloneURL, token string, rng Range) (*Result, error) {
	var auth transport.AuthMethod
	if token != "" {
		auth = &http.BasicAuth{Username: "x-access-token", Password: token}
	}

	r, err := gogit.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		return nil, fmt.Errorf("init in-memory repo: %w", err)
	}
	if _, err := r.CreateRemote(&gogitconfig.RemoteConfig{Name: remoteName, URLs: []string{cloneURL}}); err != nil {
		return nil, fmt.Errorf("add remote: %w", err)
	}

	if err := fetchSHA(ctx, r, auth, rng.Head); err != nil {
		return nil, scrub(fmt.Errorf("fetch head: %w", err), token)
	}

	res := &Result{}
	order, truncated := resolveRange(ctx, r, auth, rng)
	res.Truncated = truncated

	res.Commits = make([]verify.CommitData, 0, len(order))
	for _, h := range order {
		cd, err := commitData(r, h)
		if err != nil {
			return nil, scrub(err, token)
		}
		res.Commits = append(res.Commits, cd)
	}
	res.AnchorPath, _ = representativePath(r, rng.Head)
	return res, nil
}

// resolveRange determines the commits to verify and whether the range was
// truncated. For an update to an existing branch it returns base..head. For a
// branch creation (base is the zero SHA) it returns the commits this push
// introduces to the repo: reachable from head but not from any existing remote
// branch, so commits already on other branches are not re-verified.
func resolveRange(
	ctx context.Context, r *gogit.Repository, auth transport.AuthMethod, rng Range,
) ([]plumbing.Hash, bool) {
	if rng.Base != "" && rng.Base != zeroSHA {
		if err := fetchSHA(ctx, r, auth, rng.Base); err == nil {
			if order, complete := commitsBetween(r, rng.Base, rng.Head); complete {
				return order, false
			}
		}
		// Base unreachable (force-push) or range deeper than the window.
		window := headWindow(r, rng.Head)
		return window, true
	}
	order, truncated := newCommits(ctx, r, auth, rng.Head)
	return order, truncated
}

// newCommits returns the commits reachable from head but not from any existing
// remote branch, oldest first. It is used on branch creation, where there is no
// "before" state to diff against. If the repo has no other branches (the first
// push) it falls back to the head-only window.
func newCommits(
	ctx context.Context, r *gogit.Repository, auth transport.AuthMethod, head string,
) ([]plumbing.Hash, bool) {
	remote, err := r.Remote(remoteName)
	if err != nil {
		window := headWindow(r, head)
		return window, false
	}
	refs, err := remote.ListContext(ctx, &gogit.ListOptions{Auth: auth})
	if err != nil {
		window := headWindow(r, head)
		return window, false
	}

	headHash := plumbing.NewHash(head)
	var specs []gogitconfig.RefSpec
	var tips []plumbing.Hash
	for _, ref := range refs {
		if ref.Name().IsBranch() && ref.Hash() != headHash {
			specs = append(specs, wantRefSpec(ref.Hash().String()))
			tips = append(tips, ref.Hash())
		}
	}
	if len(specs) == 0 {
		// First branch in the repo, so everything is new.
		window := headWindow(r, head)
		return window, false
	}

	// Shallow-fetch every existing branch tip so we can exclude their ancestry.
	if err := r.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName: remoteName, Auth: auth, Depth: defaultDepth, RefSpecs: specs,
	}); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		window := headWindow(r, head)
		return window, true
	}

	exclude := map[plumbing.Hash]struct{}{}
	for _, t := range tips {
		collectAncestors(r, t, exclude)
	}
	order, boundary := topoOrder(r, headHash, exclude)
	return order, boundary
}

// fetchSHA shallow-fetches a single commit by SHA into a private ref. GitHub
// allows arbitrary-SHA wants for token clients.
func fetchSHA(ctx context.Context, r *gogit.Repository, auth transport.AuthMethod, sha string) error {
	err := r.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName: remoteName,
		Auth:       auth,
		Depth:      defaultDepth,
		RefSpecs:   []gogitconfig.RefSpec{wantRefSpec(sha)},
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch %s: %w", sha, err)
	}
	return nil
}

// commitsBetween returns the commits reachable from head but not from base
// (base..head), oldest first (topological order). complete is false if the
// shallow window did not reach base, so the caller can fall back and flag
// truncation.
func commitsBetween(r *gogit.Repository, base, head string) (order []plumbing.Hash, complete bool) {
	exclude := map[plumbing.Hash]struct{}{}
	collectAncestors(r, plumbing.NewHash(base), exclude)

	order, boundary := topoOrder(r, plumbing.NewHash(head), exclude)
	if boundary {
		return nil, false
	}
	return order, true
}

// headWindow returns every commit reachable from head within the shallow window,
// oldest first (topological order).
func headWindow(r *gogit.Repository, head string) []plumbing.Hash {
	order, _ := topoOrder(r, plumbing.NewHash(head), nil)
	return order
}

// topoOrder returns commits reachable from start (excluding the exclude set) in
// topological order, parents before children. boundary is true if a parent was
// not fetched (the shallow edge), meaning the set is incomplete. The shallow
// window bounds recursion depth.
func topoOrder(
	r *gogit.Repository, start plumbing.Hash, exclude map[plumbing.Hash]struct{},
) (order []plumbing.Hash, boundary bool) {
	visited := map[plumbing.Hash]struct{}{}
	var visit func(h plumbing.Hash)
	visit = func(h plumbing.Hash) {
		if _, ex := exclude[h]; ex {
			return
		}
		if _, v := visited[h]; v {
			return
		}
		visited[h] = struct{}{}
		c, err := r.CommitObject(h)
		if err != nil {
			boundary = true
			return
		}
		for _, p := range c.ParentHashes {
			visit(p)
		}
		order = append(order, h)
	}
	visit(start)
	return order, boundary
}

// collectAncestors fills set with start and all of its fetched ancestors. A
// shallow boundary is expected here (base's deep history is not fetched) and is
// not an error: the set only needs to cover ancestors within the window.
func collectAncestors(r *gogit.Repository, start plumbing.Hash, set map[plumbing.Hash]struct{}) {
	queue := []plumbing.Hash{start}
	for len(queue) > 0 {
		h := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if _, ok := set[h]; ok {
			continue
		}
		set[h] = struct{}{}
		c, err := r.CommitObject(h)
		if err != nil {
			continue
		}
		queue = append(queue, c.ParentHashes...)
	}
}

// commitData extracts the signed payload, signature, and committer metadata from
// an in-memory commit, matching exactly what gitsign signs.
func commitData(r *gogit.Repository, h plumbing.Hash) (verify.CommitData, error) {
	c, err := r.CommitObject(h)
	if err != nil {
		return verify.CommitData{}, fmt.Errorf("read commit %s: %w", h, err)
	}
	enc := &plumbing.MemoryObject{}
	if err := c.EncodeWithoutSignature(enc); err != nil {
		return verify.CommitData{}, fmt.Errorf("encode commit %s: %w", h, err)
	}
	rd, err := enc.Reader()
	if err != nil {
		return verify.CommitData{}, fmt.Errorf("read commit object %s: %w", h, err)
	}
	defer func() { _ = rd.Close() }()
	payload, err := io.ReadAll(rd)
	if err != nil {
		return verify.CommitData{}, fmt.Errorf("read commit body %s: %w", h, err)
	}
	return verify.CommitData{
		SHA:            h.String(),
		Payload:        payload,
		Signature:      []byte(c.PGPSignature),
		CommitterEmail: c.Committer.Email,
		CommitterName:  c.Committer.Name,
		ParentCount:    c.NumParents(),
	}, nil
}

// representativePath returns any file tracked at head, to anchor Check Run
// annotations. GitHub requires an existing path.
func representativePath(r *gogit.Repository, head string) (string, bool) {
	c, err := r.CommitObject(plumbing.NewHash(head))
	if err != nil {
		return "", false
	}
	tree, err := c.Tree()
	if err != nil {
		return "", false
	}
	f, err := tree.Files().Next()
	if err != nil {
		return "", false
	}
	return f.Name, true
}

// scrub redacts the installation token from an error before it is logged. With
// basic auth the token is not in the remote URL, but redact defensively.
func scrub(err error, token string) error {
	if err == nil || token == "" {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), token, "***"))
}
