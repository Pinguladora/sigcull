package ghapp

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/go-github/v90/github"
)

// Event is the normalised unit the webhook handler hands off for processing. It
// carries only what the verification flow needs, extracted from a push or
// pull_request event.
type Event struct {
	Owner          string
	Repo           string
	HeadSHA        string
	BaseSHA        string
	Kind           string
	InstallationID int64
}

// SubmitFunc hands a validated event to the processing pool. It returns false if
// the pool is full, so the handler can shed load instead of blocking.
type SubmitFunc func(ev Event) bool

// Handler verifies the webhook HMAC and dispatches supported events.
type Handler struct {
	submit SubmitFunc
	log    *slog.Logger
	secret []byte
}

// NewHandler builds the webhook HTTP handler.
func NewHandler(secret []byte, submit SubmitFunc, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{secret: secret, submit: submit, log: log}
}

// ServeHTTP validates X-Hub-Signature-256 before doing anything else, then
// parses the event and hands it to the pool. Processing happens asynchronously
// so GitHub gets a fast response and does not time out the delivery.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Defence in depth: an empty secret would make ValidatePayload accept a
	// payload HMAC'd with an empty key. Config already rejects this at startup,
	// but refuse here too so the trust boundary is self-contained.
	if len(h.secret) == 0 {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}

	// ValidatePayload rejects a missing or bad signature. Nothing downstream
	// runs unless the HMAC checks out.
	payload, err := github.ValidatePayload(r, h.secret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := github.WebHookType(r)
	raw, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		http.Error(w, "cannot parse payload", http.StatusBadRequest)
		return
	}

	ev, ok := normalise(raw)
	if !ok {
		// Event we do not act on (ping, unsupported action, etc). Acknowledge.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if !h.submit(ev) {
		h.log.Warn("dropping event, processing pool is full",
			"kind", ev.Kind, "owner", ev.Owner, "repo", ev.Repo)
		http.Error(w, "server busy", http.StatusServiceUnavailable)
		return
	}

	h.log.Info("dispatching event",
		"kind", ev.Kind, "owner", ev.Owner, "repo", ev.Repo, "installation", ev.InstallationID)
	w.WriteHeader(http.StatusAccepted)
}

// normalise converts a parsed webhook into an Event, returning ok=false for
// events or actions we do not act on.
func normalise(raw any) (Event, bool) {
	switch e := raw.(type) {
	case *github.PushEvent:
		return normalisePush(e)
	case *github.PullRequestEvent:
		return normalisePullRequest(e)
	case *github.CheckRunEvent:
		return normaliseCheckRun(e)
	default:
		return Event{}, false
	}
}

func normalisePush(e *github.PushEvent) (Event, bool) {
	if e.GetInstallation() == nil || e.GetRepo() == nil {
		return Event{}, false
	}
	head := e.GetAfter()
	if head == "" || head == zeroSHA {
		// Branch deletion or empty push, nothing to verify.
		return Event{}, false
	}
	// Push event repo owner fields are inconsistent, so derive owner and repo
	// from the "owner/repo" full name.
	owner, repo, ok := splitFullName(e.GetRepo().GetFullName())
	if !ok {
		return Event{}, false
	}
	return Event{
		Owner:          owner,
		Repo:           repo,
		InstallationID: e.GetInstallation().GetID(),
		HeadSHA:        head,
		BaseSHA:        e.GetBefore(),
		Kind:           "push",
	}, true
}

func normalisePullRequest(e *github.PullRequestEvent) (Event, bool) {
	switch e.GetAction() {
	case "opened", "synchronize", "reopened":
	default:
		return Event{}, false
	}
	pr := e.GetPullRequest()
	if pr == nil {
		return Event{}, false
	}
	return eventFromRepo(e.GetRepo(), e.GetInstallation(),
		pr.GetHead().GetSHA(), pr.GetBase().GetSHA(), "pull_request")
}

// normaliseCheckRun handles the "Re-run" button on our own check. GitHub only
// sends check_run events to the app that owns the check, and we guard on the
// name as well. The commit range comes from the associated pull request when
// there is one, otherwise verification falls back to the head-only window.
func normaliseCheckRun(e *github.CheckRunEvent) (Event, bool) {
	if e.GetAction() != "rerequested" {
		return Event{}, false
	}
	cr := e.GetCheckRun()
	if cr == nil || cr.GetName() != checkRunName {
		return Event{}, false
	}
	return eventFromRepo(e.GetRepo(), e.GetInstallation(),
		cr.GetHeadSHA(), pullRequestBase(cr.GetPullRequests()), "check_run")
}

// eventFromRepo builds an Event from a full Repository, failing closed on any
// missing field.
func eventFromRepo(repo *github.Repository, inst *github.Installation, head, base, kind string) (Event, bool) {
	if repo == nil || inst == nil {
		return Event{}, false
	}
	owner := repo.GetOwner().GetLogin()
	name := repo.GetName()
	if owner == "" || name == "" || head == "" {
		return Event{}, false
	}
	return Event{
		Owner:          owner,
		Repo:           name,
		InstallationID: inst.GetID(),
		HeadSHA:        head,
		BaseSHA:        base,
		Kind:           kind,
	}, true
}

// pullRequestBase returns the base SHA of the first associated pull request, or
// empty when the check is not tied to one (a push-triggered check).
func pullRequestBase(prs []*github.PullRequest) string {
	if len(prs) > 0 {
		return prs[0].GetBase().GetSHA()
	}
	return ""
}

// zeroSHA duplicates gitfetch's constant to avoid an import cycle risk and keep
// the webhook layer self-contained.
const zeroSHA = "0000000000000000000000000000000000000000"

// splitFullName splits an "owner/repo" string.
func splitFullName(full string) (owner, repo string, ok bool) {
	i := strings.IndexByte(full, '/')
	if i <= 0 || i == len(full)-1 {
		return "", "", false
	}
	return full[:i], full[i+1:], true
}
