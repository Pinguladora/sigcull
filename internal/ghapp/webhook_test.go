package ghapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// recorder captures submitted events. accept controls whether the queue is
// "full" so we can test load shedding.
type recorder struct {
	events []Event
	mu     sync.Mutex
	accept bool
}

func (r *recorder) submit(ev Event) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.accept {
		return false
	}
	r.events = append(r.events, ev)
	return true
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

const pushBody = `{
  "ref": "refs/heads/main",
  "before": "1111111111111111111111111111111111111111",
  "after": "2222222222222222222222222222222222222222",
  "repository": {"name": "repo", "full_name": "Pinguladora/repo"},
  "installation": {"id": 42}
}`

func TestWebhookRejectsBadSignature(t *testing.T) {
	secret := []byte("s3cr3t")
	rec := &recorder{accept: true}
	h := NewHandler(secret, rec.submit, nil)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", strings.NewReader(pushBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if rec.count() != 0 {
		t.Fatal("event submitted despite bad signature")
	}
}

func TestWebhookRejectsMissingSignature(t *testing.T) {
	rec := &recorder{accept: true}
	h := NewHandler([]byte("s3cr3t"), rec.submit, nil)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", strings.NewReader(pushBody))
	req.Header.Set("X-GitHub-Event", "push")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if rec.count() != 0 {
		t.Fatal("event submitted despite missing signature")
	}
}

func TestWebhookShedsLoadWhenFull(t *testing.T) {
	secret := []byte("s3cr3t")
	rec := &recorder{accept: false} // pool full
	h := NewHandler(secret, rec.submit, nil)

	body := []byte(pushBody)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", strings.NewReader(pushBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

const checkRunRerequestedBody = `{
  "action": "rerequested",
  "check_run": {
    "head_sha": "3333333333333333333333333333333333333333",
    "name": "sigcull",
    "pull_requests": [{"base": {"sha": "1111111111111111111111111111111111111111"}}]
  },
  "repository": {"name": "repo", "full_name": "Pinguladora/repo", "owner": {"login": "Pinguladora"}},
  "installation": {"id": 42}
}`

func TestWebhookDispatchesCheckRunRerun(t *testing.T) {
	secret := []byte("s3cr3t")
	rec := &recorder{accept: true}
	h := NewHandler(secret, rec.submit, nil)

	body := []byte(checkRunRerequestedBody)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", strings.NewReader(checkRunRerequestedBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "check_run")
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if rec.count() != 1 {
		t.Fatalf("expected 1 submitted event, got %d", rec.count())
	}
	ev := rec.events[0]
	if ev.Kind != "check_run" || ev.Owner != "Pinguladora" || ev.Repo != "repo" {
		t.Fatalf("bad extraction: %+v", ev)
	}
	if ev.HeadSHA != "3333333333333333333333333333333333333333" {
		t.Fatalf("bad head: %s", ev.HeadSHA)
	}
	if ev.BaseSHA != "1111111111111111111111111111111111111111" {
		t.Fatalf("bad base from PR: %s", ev.BaseSHA)
	}
}

func TestWebhookIgnoresNonRerunCheckRun(t *testing.T) {
	secret := []byte("s3cr3t")
	rec := &recorder{accept: true}
	h := NewHandler(secret, rec.submit, nil)

	body := []byte(strings.Replace(checkRunRerequestedBody, `"action": "rerequested"`, `"action": "created"`, 1))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "check_run")
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if rec.count() != 0 {
		t.Fatal("non-rerun check_run should not be processed")
	}
}

func TestWebhookDispatchesValidPush(t *testing.T) {
	secret := []byte("s3cr3t")
	rec := &recorder{accept: true}
	h := NewHandler(secret, rec.submit, nil)

	body := []byte(pushBody)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", strings.NewReader(pushBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if rec.count() != 1 {
		t.Fatalf("expected 1 submitted event, got %d", rec.count())
	}
	ev := rec.events[0]
	if ev.Owner != "Pinguladora" || ev.Repo != "repo" || ev.InstallationID != 42 {
		t.Fatalf("bad extraction: %+v", ev)
	}
	if ev.HeadSHA != "2222222222222222222222222222222222222222" {
		t.Fatalf("bad head: %s", ev.HeadSHA)
	}
}
