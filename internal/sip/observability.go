package sip

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxOperations = 256
const operationRetention = 14 * 24 * time.Hour

var correlationPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)
var lookupPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

type requestKey struct{}
type operationKey struct{}

func RequestID(ctx context.Context) string   { v, _ := ctx.Value(requestKey{}).(string); return v }
func OperationID(ctx context.Context) string { v, _ := ctx.Value(operationKey{}).(string); return v }
func randomID() string                       { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func stamp() string                          { return time.Now().UTC().Format(time.RFC3339Nano) }
func digest(value []byte) string             { s := sha256.Sum256(value); return hex.EncodeToString(s[:]) }

type DiagnosticStage struct {
	Phase     string `json:"phase"`
	State     string `json:"state"`
	ErrorCode string `json:"errorCode,omitempty"`
}
type DiagnosticResult struct {
	DestinationID string            `json:"destinationId,omitempty"`
	Platform      string            `json:"platform,omitempty"`
	Integration   string            `json:"integration,omitempty"`
	State         string            `json:"state"`
	Phase         string            `json:"phase"`
	ErrorCode     string            `json:"errorCode,omitempty"`
	Message       string            `json:"message,omitempty"`
	Retryable     bool              `json:"retryable"`
	Published     *bool             `json:"published"`
	StartedAt     string            `json:"startedAt,omitempty"`
	CompletedAt   string            `json:"completedAt,omitempty"`
	Stages        []DiagnosticStage `json:"stages,omitempty"`
}
type DiagnosticOperation struct {
	ID                   string             `json:"id"`
	Kind                 string             `json:"kind"`
	RequestID            string             `json:"requestId"`
	SessionID            string             `json:"sessionId"`
	StartedAt            string             `json:"startedAt"`
	UpdatedAt            string             `json:"updatedAt"`
	CompletedAt          string             `json:"completedAt,omitempty"`
	Phase                string             `json:"phase"`
	Outcome              string             `json:"outcome"`
	ErrorCode            string             `json:"errorCode,omitempty"`
	Message              string             `json:"message,omitempty"`
	Retryable            bool               `json:"retryable"`
	RequiresConfirmation bool               `json:"requiresConfirmation,omitempty"`
	ConfirmationID       string             `json:"confirmationId,omitempty"`
	RequestedProfileID   string             `json:"requestedProfileId,omitempty"`
	ActiveProfileID      string             `json:"activeProfileId,omitempty"`
	Results              []DiagnosticResult `json:"results,omitempty"`
	KeyHash              string             `json:"-"`
	Fingerprint          string             `json:"-"`
}
type storedOperation struct {
	Operation   DiagnosticOperation `json:"operation"`
	KeyHash     string              `json:"keyHash,omitempty"`
	Fingerprint string              `json:"fingerprint,omitempty"`
}

type Observability struct {
	prepare                   func(context.Context, string) context.Context
	mu                        sync.Mutex
	session, version, started string
	operations                map[string]DiagnosticOperation
	history                   string
	historyErr                error
	publishing                bool
	active                    bool
	revisions                 map[string]int64
	confirmations             map[string]DiagnosticOperation
	snapshots                 map[string]string
	changed                   map[string]string
	dependencies              func(context.Context) any
	logger                    func(DiagnosticOperation)
}

func NewObservability(version string, publishing bool) *Observability {
	return &Observability{session: randomID(), version: version, started: stamp(), publishing: publishing, operations: map[string]DiagnosticOperation{}, revisions: map[string]int64{}, confirmations: map[string]DiagnosticOperation{}, snapshots: map[string]string{}, changed: map[string]string{}}
}

// OpenHistory loads only safe operation metadata. Work left running by a previous
// process is unknown and never automatically re-executed.
func (o *Observability) OpenHistory(path string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.history = path
	var data []byte
	f, err := os.Open(path)
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(f, (32<<20)+1))
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		o.historyErr = err
		return err
	}
	if len(data) > 32<<20 {
		o.historyErr = errors.New("history exceeds limit")
		return o.historyErr
	}
	if len(data) > 0 {
		var entries []storedOperation
		if err = json.Unmarshal(data, &entries); err != nil {
			o.historyErr = err
			return err
		}
		if len(entries) > maxOperations {
			o.historyErr = errors.New("history exceeds capacity")
			return o.historyErr
		}
		for _, entry := range entries {
			op := entry.Operation
			began, parseErr := time.Parse(time.RFC3339Nano, op.StartedAt)
			if parseErr != nil || time.Since(began) > operationRetention {
				continue
			}
			if !lookupPattern.MatchString(op.ID) {
				continue
			}
			op.KeyHash, op.Fingerprint = entry.KeyHash, entry.Fingerprint
			if op.Outcome == "running" || op.Outcome == "confirmation_required" {
				op.Outcome = "unknown"
				op.Phase = "interrupted"
				op.ErrorCode = "INTERRUPTED"
				op.Message = safeMessage(op.ErrorCode)
				op.UpdatedAt = stamp()
				op.CompletedAt = op.UpdatedAt
				op.RequiresConfirmation = false
				op.ConfirmationID = ""
			}
			o.operations[op.ID] = op
		}
	}
	err = o.persistLocked()
	o.historyErr = err
	return err
}
func (o *Observability) persistLocked() error {
	if o.history == "" {
		return nil
	}
	var entries []storedOperation
	for _, op := range o.operations {
		entries = append(entries, storedOperation{op, op.KeyHash, op.Fingerprint})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Operation.StartedAt < entries[j].Operation.StartedAt })
	if err := os.MkdirAll(filepath.Dir(o.history), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	if len(data) > 32<<20 {
		return errors.New("operation history exceeds byte limit")
	}
	f, err := os.CreateTemp(filepath.Dir(o.history), ".sip-operations-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), o.history)
}
func (o *Observability) pruneLocked() {
	var completed []DiagnosticOperation
	cutoff := time.Now().Add(-operationRetention)
	for id, op := range o.operations {
		if op.Outcome == "running" {
			continue
		}
		t, _ := time.Parse(time.RFC3339Nano, op.StartedAt)
		if t.Before(cutoff) {
			delete(o.operations, id)
		} else {
			completed = append(completed, op)
		}
	}
	sort.Slice(completed, func(i, j int) bool { return completed[i].StartedAt < completed[j].StartedAt })
	for len(o.operations) >= maxOperations && len(completed) > 0 {
		delete(o.operations, completed[0].ID)
		completed = completed[1:]
	}
}

func safeMessage(code string) string {
	switch code {
	case "":
		return "Operation completed."
	case "INVALID_REQUEST":
		return "The request is invalid."
	case "FORBIDDEN":
		return "Local access is required."
	case "NOT_FOUND", "OPERATION_NOT_FOUND":
		return "The requested resource was not found."
	case "BUSY":
		return "Another operation is still running."
	case "TIMEOUT":
		return "The operation timed out; inspect its outcome before retrying."
	case "CANCELED":
		return "The operation was canceled."
	case "AUTH_REJECTED":
		return "The destination rejected authentication."
	case "MISSING_CREDENTIALS":
		return "Destination credentials are missing."
	case "VALIDATION_FAILED":
		return "Destination validation failed."
	case "INTEGRATION_DISABLED":
		return "The integration is disabled or unavailable."
	case "PUBLISH_FAILED":
		return "Publication failed or could not be confirmed."
	case "FOLLOW_UP_FAILED":
		return "Publication succeeded but follow-up work failed."
	case "PARTIAL_FAILURE":
		return "Some integration steps did not complete."
	case "DUPLICATE_CONFIRMATION_REQUIRED":
		return "A similar announcement requires confirmation."
	case "CONFIRMATION_EXPIRED":
		return "The confirmation expired."
	case "IDEMPOTENCY_CONFLICT":
		return "This idempotency key was used for a different request."
	case "HISTORY_WRITE_FAILED":
		return "The operation ran but its final history could not be saved. Inspect actual application state before retrying."
	case "HISTORY_UNAVAILABLE":
		return "Operation history is unavailable; the action was not started."
	case "INTERRUPTED":
		return "The previous process stopped; the final outcome is unknown."
	case "UNAVAILABLE":
		return "A required integration is unavailable."
	default:
		return "The operation could not be confirmed. Inspect application state before retrying."
	}
}
func allowedCode(code string) string {
	switch code {
	case "INVALID_REQUEST", "FORBIDDEN", "NOT_FOUND", "OPERATION_NOT_FOUND", "BUSY", "INTERNAL_ERROR", "UNAVAILABLE", "TIMEOUT", "CANCELED", "AUTH_REJECTED", "MISSING_CREDENTIALS", "VALIDATION_FAILED", "INTEGRATION_DISABLED", "PUBLISH_FAILED", "FOLLOW_UP_FAILED", "PARTIAL_FAILURE", "DUPLICATE_CONFIRMATION_REQUIRED", "CONFIRMATION_EXPIRED", "IDEMPOTENCY_CONFLICT", "HISTORY_UNAVAILABLE", "HISTORY_WRITE_FAILED", "INTERRUPTED":
		return code
	}
	return ""
}
func codeForStatus(status int) string {
	switch status {
	case 400, 405, 413, 415:
		return "INVALID_REQUEST"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 409, 429:
		return "BUSY"
	case 503:
		return "UNAVAILABLE"
	}
	if status >= 500 {
		return "INTERNAL_ERROR"
	}
	return ""
}

type capture struct {
	header    http.Header
	status    int
	body      bytes.Buffer
	truncated bool
}

func newCapture() *capture             { return &capture{header: http.Header{}} }
func (c *capture) Header() http.Header { return c.header }
func (c *capture) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}
func (c *capture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	if c.truncated {
		return 0, errors.New("SIP response too large")
	}
	if c.body.Len()+len(p) > 1<<20 {
		c.truncated = true
		c.status = 500
		c.body.Reset()
		c.body.WriteString(`{"success":false,"errorCode":"INTERNAL_ERROR"}`)
		return 0, errors.New("SIP response too large")
	}
	return c.body.Write(p)
}
func (o *Observability) response(w http.ResponseWriter, r *http.Request, status int, payload map[string]any, op *DiagnosticOperation) {
	payload["requestId"], payload["sessionId"], payload["observedAt"] = RequestID(r.Context()), o.session, stamp()
	if op != nil {
		payload["operationId"], payload["operation"] = op.ID, op
		payload["errorCode"], payload["message"], payload["retryable"], payload["results"] = op.ErrorCode, op.Message, op.Retryable, op.Results
		if op.ErrorCode == "HISTORY_WRITE_FAILED" {
			payload["success"], payload["error"] = false, op.Message
		}
		if op.Outcome == "success" {
			payload["confirmedAt"], payload["confirmedOperationId"] = op.CompletedAt, op.ID
			o.mu.Lock()
			payload["revision"] = o.revisions[op.Kind]
			o.mu.Unlock()
		}
	}
	code, _ := payload["errorCode"].(string)
	code = allowedCode(code)
	if code == "" {
		code = codeForStatus(status)
	}
	if code != "" {
		payload["errorCode"] = code
		payload["message"] = safeMessage(code)
		payload["retryable"] = code == "BUSY" || code == "UNAVAILABLE"
	}
	if code == "BUSY" {
		payload["retryAfterSeconds"] = 1
		w.Header().Set("Retry-After", "1")
	}
	w.Header().Set("X-Request-ID", RequestID(r.Context()))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
func (o *Observability) failure(w http.ResponseWriter, r *http.Request, status int, code string) {
	legacy := code
	switch code {
	case "INVALID_REQUEST":
		legacy = "InvalidRequest"
	case "FORBIDDEN":
		legacy = "Forbidden"
	}
	o.response(w, r, status, map[string]any{"success": false, "error": legacy, "errorCode": code}, nil)
}
func publishingRoute(path string) bool {
	return path == "/api/v1/announce" || path == "/api/v1/announce/confirm" || path == "/api/v1/end-stream"
}
func mutationRoute(path string) bool {
	return publishingRoute(path) || path == "/api/v1/profile" || path == "/api/v1/redeems" || path == "/api/v1/redeems/manual" || path == "/api/v1/browser-support" || path == "/api/v1/announcement-fields"
}

func (o *Observability) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = randomID()
		}
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, id))
		if !correlationPattern.MatchString(id) {
			r = r.WithContext(context.WithValue(r.Context(), requestKey{}, randomID()))
			o.failure(w, r, 400, "INVALID_REQUEST")
			return
		}
		host := hostWithoutPort(r.Host)
		if host != "localhost" && host != "127.0.0.1" || r.Header.Get("Origin") != "" || strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
			o.failure(w, r, 403, "FORBIDDEN")
			return
		}
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/operations/") {
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/operations/")
			o.mu.Lock()
			op, ok := o.operations[id]
			if began, err := time.Parse(time.RFC3339Nano, op.StartedAt); ok && err == nil && op.Outcome != "running" && time.Since(began) > operationRetention {
				delete(o.operations, id)
				ok = false
			}
			o.mu.Unlock()
			if !ok || !lookupPattern.MatchString(id) {
				o.failure(w, r, 404, "OPERATION_NOT_FOUND")
				return
			}
			o.response(w, r, 200, map[string]any{"operation": op, "operationId": op.ID}, nil)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/v1/diagnostics" {
			started, _ := time.Parse(time.RFC3339Nano, o.started)
			payload := map[string]any{"appVersion": o.version, "buildRevision": buildRevision(), "protocolVersion": 1, "startedAt": o.started, "uptimeSeconds": int64(time.Since(started).Seconds())}
			if o.dependencies != nil {
				payload["dependencies"] = o.dependencies(r.Context())
			}
			o.response(w, r, 200, payload, nil)
			return
		}
		if r.Method == "POST" && mutationRoute(r.URL.Path) {
			o.mutate(w, r, next)
			return
		}
		c := newCapture()
		next.ServeHTTP(c, r)
		if c.status == 0 {
			c.status = 200
		}
		var payload map[string]any
		if json.Unmarshal(c.body.Bytes(), &payload) != nil {
			for k, v := range c.header {
				w.Header()[k] = v
			}
			w.WriteHeader(c.status)
			_, _ = w.Write(c.body.Bytes())
			return
		}
		if r.URL.Path == "/api/v1/status" || r.URL.Path == "/api/v1/profile/current" {
			o.mu.Lock()
			if fingerprint := digest(c.body.Bytes()); o.snapshots[r.URL.Path] != fingerprint {
				o.snapshots[r.URL.Path] = fingerprint
				o.changed[r.URL.Path] = stamp()
			}
			payload["updatedAt"] = o.changed[r.URL.Path]
			if op, ok := o.confirmations["/api/v1/profile"]; ok && (op.ActiveProfileID == "" || payload["id"] == op.ActiveProfileID || payload["activeProfileId"] == op.ActiveProfileID) {
				payload["confirmedAt"], payload["confirmedOperationId"], payload["revision"] = op.CompletedAt, op.ID, o.revisions["/api/v1/profile"]
			}
			o.mu.Unlock()
		}
		if r.URL.Path == "/api/v1/announce/status" || r.URL.Path == "/api/v1/end-stream/status" {
			o.mu.Lock()
			var latest DiagnosticOperation
			for _, op := range o.operations {
				match := r.URL.Path == "/api/v1/end-stream/status" && op.Kind == "/api/v1/end-stream" || r.URL.Path == "/api/v1/announce/status" && (op.Kind == "/api/v1/announce" || op.Kind == "/api/v1/announce/confirm")
				if match && op.Outcome != "running" && op.CompletedAt > latest.CompletedAt {
					latest = op
				}
			}
			o.mu.Unlock()
			if latest.ID != "" {
				payload["operationId"], payload["operation"], payload["results"], payload["errorCode"], payload["message"] = latest.ID, latest, latest.Results, latest.ErrorCode, latest.Message
				if payload["lastRun"] == "" || payload["lastRun"] == nil {
					payload["lastRun"], payload["success"] = latest.CompletedAt, latest.Outcome == "success"
					if latest.Outcome != "success" {
						payload["error"] = latest.Message
					}
				}
			}
		}

		for k, v := range c.header {
			w.Header()[k] = v
		}
		o.response(w, r, c.status, payload, nil)
	})
}

func (o *Observability) mutate(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if (r.ContentLength != 0 || r.Header.Get("Content-Type") != "" || !publishingRoute(r.URL.Path)) && !strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]), "application/json") {
		o.failure(w, r, 415, "INVALID_REQUEST")
		return
	}
	if r.Body == nil {
		r.Body = http.NoBody
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		o.failure(w, r, 413, "INVALID_REQUEST")
		return
	}
	if o.prepare != nil {
		r = r.WithContext(o.prepare(r.Context(), r.URL.Path))
	}
	key := r.Header.Get("Idempotency-Key")
	if key != "" && !correlationPattern.MatchString(key) {
		o.failure(w, r, 400, "INVALID_REQUEST")
		return
	}
	async := o.publishing && publishingRoute(r.URL.Path) && strings.EqualFold(strings.TrimSpace(r.Header.Get("Prefer")), "respond-async")
	durable := o.publishing && publishingRoute(r.URL.Path)
	fingerprint := digest(append([]byte(r.Method+" "+r.URL.Path+"\n"), data...))
	hash := ""
	if key != "" {
		hash = digest([]byte(key))
	}
	o.mu.Lock()
	if hash != "" {
		for _, op := range o.operations {
			if op.KeyHash != hash {
				continue
			}
			o.mu.Unlock()
			if op.Fingerprint != fingerprint {
				o.failure(w, r, 409, "IDEMPOTENCY_CONFLICT")
				return
			}
			status := 200
			if op.Outcome == "running" {
				status = 202
			}
			w.Header().Set("Location", "/api/v1/operations/"+op.ID)
			o.response(w, r, status, replayPayload(op), &op)
			return
		}
	}
	if durable && (o.historyErr != nil || ((async || hash != "") && o.history == "")) {
		o.mu.Unlock()
		o.failure(w, r, 503, "HISTORY_UNAVAILABLE")
		return
	}
	if o.publishing && o.active {
		o.mu.Unlock()
		o.failure(w, r, 409, "BUSY")
		return
	}
	o.pruneLocked()
	if len(o.operations) >= maxOperations {
		o.mu.Unlock()
		o.failure(w, r, 409, "BUSY")
		return
	}
	op := DiagnosticOperation{ID: randomID(), Kind: r.URL.Path, RequestID: RequestID(r.Context()), SessionID: o.session, StartedAt: stamp(), UpdatedAt: stamp(), Phase: "executing", Outcome: "running", KeyHash: hash, Fingerprint: fingerprint}
	o.operations[op.ID] = op
	if err := o.persistLocked(); err != nil {
		delete(o.operations, op.ID)
		o.historyErr = err
		o.mu.Unlock()
		o.failure(w, r, 503, "HISTORY_UNAVAILABLE")
		return
	}
	if durable {
		o.active = true
	}
	o.mu.Unlock()
	execute := func(ctx context.Context) (*capture, DiagnosticOperation) {
		ctx = context.WithValue(context.WithValue(ctx, requestKey{}, op.RequestID), operationKey{}, op.ID)
		request := r.Clone(ctx)
		request.Body = io.NopCloser(bytes.NewReader(data))
		request.ContentLength = int64(len(data))
		c := newCapture()
		func() {
			defer func() {
				if recover() != nil {
					c = newCapture()
					c.status = 500
					c.body.WriteString(`{"success":false,"errorCode":"INTERNAL_ERROR"}`)
				}
			}()
			next.ServeHTTP(c, request)
		}()
		if c.status == 0 {
			c.status = 200
		}
		var payload map[string]any
		_ = json.Unmarshal(c.body.Bytes(), &payload)
		completed := operationFromPayload(op, c.status, payload)
		if ctx.Err() != nil && completed.Outcome != "success" {
			completed.Outcome = "unknown"
			completed.ErrorCode = "TIMEOUT"
			completed.Message = safeMessage(completed.ErrorCode)
		}
		o.mu.Lock()
		o.operations[op.ID] = completed
		if durable {
			o.active = false
		}
		if completed.Outcome == "success" {
			o.revisions[op.Kind]++
			o.confirmations[op.Kind] = completed
		}
		if err := o.persistLocked(); err != nil {
			o.historyErr = err
			completed.Outcome = "unknown"
			completed.ErrorCode = "HISTORY_WRITE_FAILED"
			completed.Message = "The operation ran but its final history could not be saved. Inspect actual application state before retrying."
			o.operations[op.ID] = completed
		}
		o.mu.Unlock()
		o.logOperation(completed)
		return c, completed
	}
	o.logOperation(op)
	if async {
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 4*time.Minute+30*time.Second)
			defer cancel()
			execute(ctx)
		}()
		w.Header().Set("Location", "/api/v1/operations/"+op.ID)
		w.Header().Set("Preference-Applied", "respond-async")
		o.response(w, r, 202, map[string]any{}, &op)
		return
	}
	c, completed := execute(r.Context())
	var payload map[string]any
	if json.Unmarshal(c.body.Bytes(), &payload) != nil {
		payload = replayPayload(completed)
	}
	for k, v := range c.header {
		w.Header()[k] = v
	}
	o.response(w, r, c.status, payload, &completed)
}
func replayPayload(op DiagnosticOperation) map[string]any {
	return map[string]any{"error": safeReplayError(op), "success": op.Outcome == "success", "requiresConfirmation": op.RequiresConfirmation, "confirmationId": op.ConfirmationID, "errorCode": op.ErrorCode, "message": op.Message, "results": op.Results}
}
func operationFromPayload(op DiagnosticOperation, status int, payload map[string]any) DiagnosticOperation {
	op.CompletedAt = stamp()
	op.UpdatedAt = op.CompletedAt
	op.Phase = "completed"
	op.Outcome = "success"
	if status >= 400 || payload["success"] == false {
		op.Outcome = "failed"
		if status >= 500 {
			op.Outcome = "unknown"
		}
		op.ErrorCode = codeForStatus(status)
		if op.ErrorCode == "" {
			op.ErrorCode = "INTERNAL_ERROR"
		}
	}
	if confirmation, _ := payload["requiresConfirmation"].(bool); confirmation {
		op.Outcome = "confirmation_required"
		op.Phase = "awaiting_confirmation"
		op.ErrorCode = "DUPLICATE_CONFIRMATION_REQUIRED"
		op.RequiresConfirmation = true
		op.ConfirmationID, _ = payload["confirmationId"].(string)
	}
	if code, _ := payload["errorCode"].(string); allowedCode(code) != "" {
		op.ErrorCode = code
	}
	if outcome, _ := payload["operationOutcome"].(string); outcome == "partial" || outcome == "unknown" || outcome == "canceled" {
		op.Outcome = outcome
	}
	op.RequestedProfileID, _ = payload["requestedProfileId"].(string)
	op.ActiveProfileID, _ = payload["activeProfileId"].(string)
	if op.ActiveProfileID == "" {
		op.ActiveProfileID, _ = payload["profileId"].(string)
	}
	if data, err := json.Marshal(payload["results"]); err == nil {
		_ = json.Unmarshal(data, &op.Results)
	}
	if len(op.Results) > 128 {
		op.Results = op.Results[:128]
	}
	for i := range op.Results {
		op.Results[i].ErrorCode = allowedCode(op.Results[i].ErrorCode)
		op.Results[i].Message = safeMessage(op.Results[i].ErrorCode)
	}
	op.Message = safeMessage(op.ErrorCode)
	return op
}

func (o *Observability) logOperation(op DiagnosticOperation) {
	if o.logger != nil {
		o.logger(op)
		return
	}
	log.Printf("sip_operation request_id=%s operation_id=%s session_id=%s outcome=%s code=%s", op.RequestID, op.ID, op.SessionID, op.Outcome, op.ErrorCode)
}
func (s *Service) UseOperationLogger(logger func(DiagnosticOperation)) { s.observer.logger = logger }

func safeReplayError(op DiagnosticOperation) string {
	if op.Outcome == "success" || op.RequiresConfirmation {
		return ""
	}
	return op.Message
}

func buildRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && len(setting.Value) <= 64 {
			if _, err := hex.DecodeString(setting.Value); err == nil {
				return setting.Value
			}
		}
	}
	return "unknown"
}
