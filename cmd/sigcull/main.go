// Command sigcull is a GitHub App that verifies gitsign (Sigstore
// keyless) commit signatures and reports the result as a Check Run, so it can
// be made a required status check via rulesets or branch protection.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Pinguladora/sigcull/internal/config"
	"github.com/Pinguladora/sigcull/internal/ghapp"
	"github.com/Pinguladora/sigcull/internal/gitfetch"
	"github.com/Pinguladora/sigcull/internal/verify"
	"github.com/google/go-github/v91/github"
)

const (
	// queueSize bounds how many events can wait for a worker before the handler
	// starts shedding load with a 503.
	queueSize = 128
	// shutdownTimeout bounds graceful drain on SIGINT/SIGTERM.
	shutdownTimeout = 30 * time.Second
	// readHeaderTimeout guards against slow-header clients.
	readHeaderTimeout = 10 * time.Second
)

// Build metadata, injected by GoReleaser through -ldflags -X main.*. The defaults
// apply to a plain `go build` or `go run`.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Structured logs go to stdout so a 12-factor platform collects them as the
	// app's event stream (the server writes no other stdout output). LOG_LEVEL
	// sets verbosity, defaulting to info.
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()}))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// logLevel reads LOG_LEVEL (debug/info/warn/error), defaulting to info on an
// empty or unrecognised value so a typo never silences or crashes the app.
func logLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// run wires everything together and serves until a shutdown signal. It returns
// an error rather than exiting so main owns the single os.Exit.
func run(log *slog.Logger) error {
	configPath := flag.String("config", "config.yaml", "path to the YAML config file")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("sigcull %s (commit %s, built %s, %s)\n", version, commit, date, runtime.Version())
		return nil
	}
	log.Info("starting sigcull", "version", version, "commit", commit)

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	factory, err := ghapp.NewClientFactory(cfg.AppID, cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("init github app: %w", err)
	}

	var githubAuthority *ghapp.GitHubAuthority
	if cfg.GitHubVerified {
		githubAuthority, err = ghapp.NewGitHubAuthority(cfg.GitHubWebFlowOnly)
		if err != nil {
			return fmt.Errorf("init github authority: %w", err)
		}
	}
	proc := &processor{
		factory:         factory,
		verifier:        cfg.Verifier,
		githubAuthority: githubAuthority,
		exempt:          cfg.Exempt,
		matchCommitter:  cfg.MatchCommitter,
		maskIdentities:  cfg.MaskIdentities,
		log:             log,
	}
	// Bounded pool so a burst of deliveries cannot spawn unbounded clones.
	pool := ghapp.NewPool(runtime.NumCPU(), queueSize, proc.Process, log)

	mux := http.NewServeMux()
	mux.Handle("/webhook", ghapp.NewHandler(cfg.WebhookSecret, pool.Submit, log))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: readHeaderTimeout}

	return serve(srv, pool, log)
}

// serve runs the HTTP server until it fails or a shutdown signal arrives, then
// drains in-flight work within shutdownTimeout.
func serve(srv *http.Server, pool *ghapp.Pool, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("listening", "addr", srv.Addr)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		// Stop accepting connections, then drain in-flight verification work.
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("http shutdown", "err", err)
		}
		pool.Shutdown(shutdownCtx)
		return nil
	}
}

// processor runs the end-to-end verification flow for one webhook event.
type processor struct {
	verifier        verify.Verifier
	factory         *ghapp.ClientFactory
	log             *slog.Logger
	githubAuthority *ghapp.GitHubAuthority
	exempt          config.Exempt
	matchCommitter  bool
	maskIdentities  bool
}

// processTimeout bounds the whole per-event flow, including clone and all
// gitsign invocations.
const processTimeout = 5 * time.Minute

func (p *processor) Process(parent context.Context, ev ghapp.Event) {
	ctx, cancel := context.WithTimeout(parent, processTimeout)
	defer cancel()

	log := p.log.With("owner", ev.Owner, "repo", ev.Repo, "kind", ev.Kind)

	client, err := p.factory.ForInstallation(ev.InstallationID)
	if err != nil {
		log.Error("installation client", "err", err)
		return
	}
	check := ghapp.NewCheckRun(client, ev.Owner, ev.Repo, p.maskIdentities)

	runID, err := check.Create(ctx, ev.HeadSHA)
	if err != nil {
		log.Error("create check run", "err", err)
		return
	}

	results, truncated, anchorPath, err := p.verifyRange(ctx, client, ev)
	if err != nil {
		// Report the failure on the check run itself so it is visible in the PR.
		// gitfetch already scrubs the token from any error it returns.
		log.Error("verify range", "err", err)
		_ = check.Complete(ctx, runID, []verify.CommitResult{{
			SHA:    ev.HeadSHA,
			OK:     false,
			Reason: "internal error while fetching or verifying commits",
		}}, truncated, "")
		return
	}

	if err := check.Complete(ctx, runID, results, truncated, anchorPath); err != nil {
		log.Error("complete check run", "err", err)
		return
	}
	log.Info("check run complete", "commits", len(results), "truncated", truncated)
}

// verifyRange fetches the commit range and verifies each commit, applying the
// exemption settings. It also returns a repository path to anchor annotations
// to, empty if none is available.
func (p *processor) verifyRange(
	ctx context.Context,
	client *github.Client,
	ev ghapp.Event,
) (results []verify.CommitResult, truncated bool, anchorPath string, err error) {
	token, err := p.factory.InstallationToken(ctx, ev.InstallationID)
	if err != nil {
		return nil, false, "", fmt.Errorf("mint installation token: %w", err)
	}
	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", ev.Owner, ev.Repo)

	fetched, err := gitfetch.Fetch(ctx, cloneURL, token, gitfetch.Range{Base: ev.BaseSHA, Head: ev.HeadSHA})
	if err != nil {
		return nil, false, "", fmt.Errorf("fetch commits: %w", err)
	}

	results = make([]verify.CommitResult, 0, len(fetched.Commits))
	for _, c := range fetched.Commits {
		if reason, skip := p.exemption(c); skip {
			results = append(results, verify.CommitResult{
				SHA: c.SHA, OK: true, Reason: reason, Authority: verify.AuthorityExempt,
			})
			continue
		}
		res := p.verifier.Verify(ctx, c)
		// The githubVerified authority is a fallback: only commits that no crypto
		// authority accepted are checked against GitHub's own verification.
		if !res.OK && p.githubAuthority != nil {
			if accepted, ok := p.githubAuthority.Accept(ctx, client, ev.Owner, ev.Repo, c); ok {
				res = accepted
			}
		}
		res = p.enforceCommitter(res, c)
		results = append(results, res)
	}
	return results, fetched.Truncated, fetched.AnchorPath, nil
}

// enforceCommitter optionally rejects an otherwise-valid commit whose committer
// identity does not match the signing identity, closing the attribution gap.
func (p *processor) enforceCommitter(res verify.CommitResult, c verify.CommitData) verify.CommitResult {
	if !p.matchCommitter || !res.OK || res.SignerSAN == "" {
		return res
	}
	if verify.CommitterMatches(c.CommitterEmail, c.CommitterName, res.SignerSAN) {
		return res
	}
	res.OK = false
	res.Reason = verify.CommitterMismatchReason(res.SignerSAN, c.CommitterEmail, c.CommitterName)
	return res
}

// exemption reports whether a commit is exempt from verification and why. It
// fails closed: any doubt means the commit is verified, not skipped. GitHub-
// signed commits are handled by the githubVerified authority, not here.
func (p *processor) exemption(c verify.CommitData) (reason string, skip bool) {
	if p.exempt.SkipMergeCommits && c.ParentCount > 1 {
		return "skipped: merge commit (exempt)", true
	}
	return "", false
}
