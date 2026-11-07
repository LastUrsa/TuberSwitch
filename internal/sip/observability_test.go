package sip

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func observeRequest(t *testing.T, handler http.Handler, method, path, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Request-ID", "client-request-1")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func observationPayload(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestObservationCorrelationAndSafeErrors(t *testing.T) {
	o := NewObservability("test", false)
	handler := o.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) != "client-request-1" {
			t.Error("missing request context")
		}
		writeJSON(w, 500, map[string]any{"success": false, "error": "password=secret"})
	}))
	w := observeRequest(t, handler, "POST", "/api/v1/profile", `{"profile":"private"}`, "")
	p := observationPayload(t, w)
	if p["requestId"] != "client-request-1" || w.Header().Get("X-Request-ID") != "client-request-1" {
		t.Fatal("missing correlation")
	}
	op := p["operation"].(map[string]any)
	data, _ := json.Marshal(op)
	if bytes.Contains(data, []byte("secret")) || bytes.Contains(data, []byte("private")) {
		t.Fatal("unsafe operation snapshot")
	}
	if op["errorCode"] != "INTERNAL_ERROR" || op["outcome"] != "unknown" {
		t.Fatalf("unexpected operation: %v", op)
	}
	id := p["operationId"].(string)
	lookup := observeRequest(t, handler, "GET", "/api/v1/operations/"+id, "", "")
	if lookup.Code != 200 {
		t.Fatal(lookup.Body.String())
	}
	invalid := httptest.NewRequest("GET", "http://localhost/api/v1/app", nil)
	invalid.Header.Set("X-Request-ID", strings.Repeat("x", 65))
	bad := httptest.NewRecorder()
	handler.ServeHTTP(bad, invalid)
	if bad.Code != 400 {
		t.Fatal("invalid ID accepted")
	}
}
func TestPublishingIdempotencyAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	o := NewObservability("test", true)
	if err := o.OpenHistory(path); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	handler := o.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeJSON(w, 200, map[string]any{"success": true})
	}))
	first := observeRequest(t, handler, "POST", "/api/v1/announce", `{}`, "publish-key")
	second := observeRequest(t, handler, "POST", "/api/v1/announce", `{}`, "publish-key")
	a, b := observationPayload(t, first), observationPayload(t, second)
	if calls.Load() != 1 || a["operationId"] != b["operationId"] {
		t.Fatal("duplicate publishing executed")
	}
	conflict := observeRequest(t, handler, "POST", "/api/v1/end-stream", `{}`, "publish-key")
	if conflict.Code != 409 {
		t.Fatal("key conflict accepted")
	}
	restored := NewObservability("test", true)
	if err := restored.OpenHistory(path); err != nil {
		t.Fatal(err)
	}
	replay := observeRequest(t, restored.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("replayed after restart") })), "POST", "/api/v1/announce", `{}`, "publish-key")
	if observationPayload(t, replay)["operationId"] != a["operationId"] {
		t.Fatal("operation ID changed across restart")
	}
	data, _ := os.ReadFile(path)
	if bytes.Contains(data, []byte("publish-key")) {
		t.Fatal("raw idempotency key persisted")
	}
}
func TestAsyncExecutionSurvivesRequestCancellation(t *testing.T) {
	o := NewObservability("test", true)
	if err := o.OpenHistory(filepath.Join(t.TempDir(), "operations.json")); err != nil {
		t.Fatal(err)
	}
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := o.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		if r.Context().Err() != nil {
			t.Error("async operation tied to client cancellation")
		}
		writeJSON(w, 200, map[string]any{"success": true})
		close(finished)
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("POST", "http://localhost/api/v1/announce", strings.NewReader(`{}`)).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Prefer", "respond-async")
	r.Header.Set("Idempotency-Key", "async-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	<-started
	cancel()
	replay := observeRequest(t, handler, "POST", "/api/v1/announce", `{}`, "async-key")
	if replay.Code != 202 {
		t.Fatal("running replay must return accepted")
	}
	close(release)
	<-finished
	deadline := time.Now().Add(time.Second)
	for {
		o.mu.Lock()
		running := false
		for _, op := range o.operations {
			running = running || op.Outcome == "running"
		}
		o.mu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("async completion timed out")
		}
		time.Sleep(time.Millisecond)
	}

	// Persistence completion is synchronized by the observer mutex; closing the
	// handler is not required to retrieve the operation after completion.
}
func TestHistoryFailurePreventsPublishing(t *testing.T) {
	parent := t.TempDir()
	blocked := filepath.Join(parent, "file")
	os.WriteFile(blocked, []byte("blocked"), 0o600)
	o := NewObservability("test", true)
	_ = o.OpenHistory(filepath.Join(blocked, "operations.json"))
	handler := o.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("published without history") }))
	w := observeRequest(t, handler, "POST", "/api/v1/announce", `{}`, "key")
	if w.Code != 503 {
		t.Fatal(w.Body.String())
	}
}
func TestInterruptedOperationIsNotReplayed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	o := NewObservability("test", true)
	_ = o.OpenHistory(path)
	op := DiagnosticOperation{ID: randomID(), Kind: "/api/v1/announce", RequestID: "request", SessionID: o.session, StartedAt: stamp(), UpdatedAt: stamp(), Outcome: "running", KeyHash: digest([]byte("key")), Fingerprint: digest([]byte("POST /api/v1/announce\n{}"))}
	o.mu.Lock()
	o.operations[op.ID] = op
	err := o.persistLocked()
	o.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	restored := NewObservability("test", true)
	if err := restored.OpenHistory(path); err != nil {
		t.Fatal(err)
	}
	w := observeRequest(t, restored.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("interrupted request replayed") })), "POST", "/api/v1/announce", `{}`, "key")
	outcome := observationPayload(t, w)["operation"].(map[string]any)
	if outcome["outcome"] != "unknown" || outcome["errorCode"] != "INTERRUPTED" {
		t.Fatal(outcome)
	}
}

func TestHistoryFailureAfterExecutionNeverClaimsTheActionDidNotRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	o := NewObservability("test", true)
	if err := o.OpenHistory(path); err != nil {
		t.Fatal(err)
	}
	handler := o.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		writeJSON(w, 200, map[string]any{"success": true})
	}))
	response := observeRequest(t, handler, "POST", "/api/v1/announce", `{}`, "key")
	payload := observationPayload(t, response)
	if payload["success"] != false || payload["errorCode"] != "HISTORY_WRITE_FAILED" {
		t.Fatal(payload)
	}
	operation := payload["operation"].(map[string]any)
	if operation["outcome"] != "unknown" || strings.Contains(payload["message"].(string), "not started") {
		t.Fatal("history failure misrepresented completed side effects")
	}
}

func TestBrowserOriginsCannotCreateOperationHistory(t *testing.T) {
	o := NewObservability("test", false)
	handler := o.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("browser request reached mutation") }))
	request := httptest.NewRequest("POST", "http://localhost/api/v1/profile", strings.NewReader(`{"profile":"example"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://example.invalid")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 403 || len(o.operations) != 0 {
		t.Fatal("browser request changed operation history")
	}
}
