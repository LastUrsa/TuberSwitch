package sip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSIPV11ContractForLivePanel(t *testing.T) {
	service := newTestService()
	handler := service.Handler()

	appPayload := requestRawJSON(t, handler, http.MethodGet, "/api/v1/app", nil, http.StatusOK)
	assertJSONFields(t, appPayload, map[string]any{
		"appId":           "tuberswitch",
		"name":            "TuberSwitch",
		"version":         "0.5.0",
		"mode":            "standalone",
		"protocolVersion": "1.1",
	})

	healthPayload := requestRawJSON(t, handler, http.MethodGet, "/api/v1/health", nil, http.StatusOK)
	assertJSONFields(t, healthPayload, map[string]any{
		"status":  HealthReady,
		"message": "TuberSwitch operational",
	})

	capabilitiesPayload := requestRawJSON(t, handler, http.MethodGet, "/api/v1/capabilities", nil, http.StatusOK)
	assertJSONFields(t, capabilitiesPayload, map[string]any{
		"supportsProfiles":        true,
		"supportsStatusReporting": true,
	})

	statusPayload := requestRawJSON(t, handler, http.MethodGet, "/api/v1/status", nil, http.StatusOK)
	assertJSONFields(t, statusPayload, map[string]any{
		"state":                 StatusReady,
		"message":               "Profile active",
		"healthy":               true,
		"activeProfile":         "Default",
		"activeProfileId":       "default",
		"activeMode":            "png",
		"obsSummary":            "Connected: Gaming / PNG",
		"obsConnected":          true,
		"activeScene":           "Gaming",
		"activeSource":          "PNG",
		"redeemsEnabled":        true,
		"redeemCount":           float64(2),
		"appDetectionStatus":    "PNG app detected",
		"appDetectionEnabled":   true,
		"currentModeLabel":      "PNGTuber Mode",
		"activeProfileLastUsed": "2026-06-10T12:00:00Z",
	})
}

func TestAppReportsConfiguredRuntimeMode(t *testing.T) {
	service := NewService(AppInfo{
		AppID:    "tuberswitch",
		Name:     "TuberSwitch",
		Version:  "0.5.0",
		Mode:     ServiceMode,
		Protocol: ProtocolVersion,
	}, &fakeController{})

	var app AppResponse
	requestJSON(t, service.Handler(), http.MethodGet, "/api/v1/app", nil, http.StatusOK, &app)
	if app.Mode != ServiceMode {
		t.Fatalf("expected service mode, got %+v", app)
	}
}

func TestProfilesAndActivation(t *testing.T) {
	controller := &fakeController{
		profiles: []Profile{
			{ID: "default", Name: "Default", Mode: "png"},
			{ID: "gaming", Name: "Gaming Stream", Mode: "3d"},
		},
		current: Profile{ID: "default", Name: "Default", Mode: "png"},
	}
	service := newTestService()
	service.controller = controller

	var listed ProfilesResponse
	requestJSON(t, service.Handler(), http.MethodGet, "/api/v1/profiles", nil, http.StatusOK, &listed)
	if len(listed.Profiles) != 2 || listed.Profiles[1] != "Gaming Stream" {
		t.Fatalf("profiles = %+v", listed)
	}

	var activated ProfileActivationResponse
	requestJSON(t, service.Handler(), http.MethodPost, "/api/v1/profile", map[string]string{"profile": "gaming stream"}, http.StatusOK, &activated)
	if !activated.Success || activated.Profile != "Gaming Stream" || activated.ProfileID != "gaming" {
		t.Fatalf("activation = %+v", activated)
	}

	var current CurrentProfileResponse
	requestJSON(t, service.Handler(), http.MethodGet, "/api/v1/profile/current", nil, http.StatusOK, &current)
	if current.ID != "gaming" || current.Name != "Gaming Stream" {
		t.Fatalf("current = %+v", current)
	}
}

func TestCurrentProfileReturnsEmptyState(t *testing.T) {
	service := newTestService()
	service.controller = &fakeController{
		profiles: []Profile{{ID: "default", Name: "Default", Mode: "png"}},
	}

	var current CurrentProfileResponse
	requestJSON(t, service.Handler(), http.MethodGet, "/api/v1/profile/current", nil, http.StatusOK, &current)
	if current.ID != "" || current.Name != "" {
		t.Fatalf("current = %+v", current)
	}
}

func TestHealthAndStatusReportControllerFailures(t *testing.T) {
	service := newTestService()
	service.controller = &fakeController{err: errors.New("profile store unavailable")}

	var health HealthResponse
	requestJSON(t, service.Handler(), http.MethodGet, "/api/v1/health", nil, http.StatusOK, &health)
	if health.Status != HealthError || health.Message != "TuberSwitch profiles are unavailable." {
		t.Fatalf("health = %+v", health)
	}

	var status StatusResponse
	requestJSON(t, service.Handler(), http.MethodGet, "/api/v1/status", nil, http.StatusOK, &status)
	if status.State != StatusError || status.Healthy || status.Message != "TuberSwitch profiles are unavailable." {
		t.Fatalf("status = %+v", status)
	}
}

func TestStatusIgnoresOptionalDetailsFailures(t *testing.T) {
	service := newTestService()
	service.controller = &fakeController{
		profiles:   []Profile{{ID: "default", Name: "Default", Mode: "png"}},
		current:    Profile{ID: "default", Name: "Default", Mode: "png"},
		detailsErr: errors.New("details unavailable"),
	}

	var status StatusResponse
	requestJSON(t, service.Handler(), http.MethodGet, "/api/v1/status", nil, http.StatusOK, &status)
	if status.State != StatusReady || !status.Healthy || status.ActiveProfile != "Default" {
		t.Fatalf("status = %+v", status)
	}
	if status.OBSSummary != "" || status.OBSConnected || status.RedeemsEnabled || status.AppDetectionEnabled {
		t.Fatalf("optional details should be omitted/zeroed: %+v", status)
	}
}

func TestActivateProfileErrors(t *testing.T) {
	service := newTestService()

	var missing ErrorResponse
	requestJSON(t, service.Handler(), http.MethodPost, "/api/v1/profile", map[string]string{"profile": "Missing"}, http.StatusNotFound, &missing)
	if missing.Error != "ProfileNotFound" {
		t.Fatalf("missing = %+v", missing)
	}

	var invalid ErrorResponse
	requestJSON(t, service.Handler(), http.MethodPost, "/api/v1/profile", map[string]string{"profile": ""}, http.StatusBadRequest, &invalid)
	if invalid.Error != "InvalidRequest" {
		t.Fatalf("invalid = %+v", invalid)
	}
}

func TestHandlerValidationAndLocalhostProtection(t *testing.T) {
	service := newTestService()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	request.Host = "127.0.0.1"
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("method status = %d allow=%q", response.Code, response.Header().Get("Allow"))
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/profile", strings.NewReader(`{"profile":"Default"}`))
	request.Host = "127.0.0.1"
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing content-type status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/profile", strings.NewReader(`{"profile":`))
	request.Host = "127.0.0.1"
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/profile", strings.NewReader(`{"profile":"Default","padding":"`+strings.Repeat("x", int(maxRequestBodyBytes))+`"}`))
	request.Host = "127.0.0.1"
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/profile", strings.NewReader(`{"profile":"Default","padding":"`+strings.Repeat("x", int(maxRequestBodyBytes))+`"}`))
	request.Host = "127.0.0.1"
	request.ContentLength = -1
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("streamed oversized status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/profile", bytes.NewBufferString(`{"profile":"Default","extra":true}`))
	request.Host = "127.0.0.1"
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/app", nil)
	request.Host = "example.com"
	response = httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("remote host status = %d", response.Code)
	}
}

func TestHandlerSecurityHeaders(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/app", nil)
	request.Host = "localhost:47040"
	newTestService().Handler().ServeHTTP(response, request)

	headers := map[string]string{
		"Cache-Control":           "no-store",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
	}
	for name, want := range headers {
		if got := response.Header().Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func newTestService() *Service {
	return NewService(AppInfo{
		AppID:    "tuberswitch",
		Name:     "TuberSwitch",
		Version:  "0.5.0",
		Mode:     StandaloneMode,
		Protocol: ProtocolVersion,
	}, &fakeController{
		profiles: []Profile{
			{ID: "default", Name: "Default", Mode: "png"},
			{ID: "gaming", Name: "Gaming Stream", Mode: "3d"},
		},
		current: Profile{ID: "default", Name: "Default", Mode: "png"},
		details: StatusDetails{
			OBSConnected:          true,
			OBSSummary:            "Connected: Gaming / PNG",
			ActiveScene:           "Gaming",
			ActiveSource:          "PNG",
			RedeemsEnabled:        true,
			RedeemCount:           2,
			AppDetectionEnabled:   true,
			AppDetectionStatus:    "PNG app detected",
			CurrentModeLabel:      "PNGTuber Mode",
			ActiveProfileLastUsed: "2026-06-10T12:00:00Z",
		},
	})
}

type fakeController struct {
	profiles   []Profile
	current    Profile
	details    StatusDetails
	detailsErr error
	err        error
}

func (f fakeController) SIPProfiles(context.Context) ([]Profile, error) {
	return append([]Profile(nil), f.profiles...), f.err
}

func (f fakeController) SIPCurrentProfile(context.Context) (Profile, error) {
	return f.current, f.err
}

func (f *fakeController) SIPActivateProfile(_ context.Context, name string) (Profile, error) {
	if f.err != nil {
		return Profile{}, f.err
	}
	for _, profile := range f.profiles {
		if stringsEqualFold(profile.Name, name) {
			f.current = profile
			return profile, nil
		}
	}
	return Profile{}, ErrProfileNotFound
}

func (f fakeController) SIPStatusDetails(context.Context) (StatusDetails, error) {
	return f.details, f.detailsErr
}

func stringsEqualFold(left string, right string) bool {
	return strings.ToLower(left) == strings.ToLower(right)
}

func requestRawJSON(t *testing.T, handler http.Handler, method string, path string, body any, wantStatus int) map[string]any {
	t.Helper()
	var decoded map[string]any
	requestJSON(t, handler, method, path, body, wantStatus, &decoded)
	return decoded
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body any, wantStatus int, target any) {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Host = "127.0.0.1"
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d body=%s", method, path, response.Code, wantStatus, response.Body.String())
	}
	if target != nil {
		if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
			t.Fatalf("decode response: %v body=%s", err, response.Body.String())
		}
	}
}

func assertJSONFields(t *testing.T, payload map[string]any, fields map[string]any) {
	t.Helper()
	for key, want := range fields {
		if got := payload[key]; got != want {
			t.Fatalf("%s = %#v, want %#v in %#v", key, got, want, payload)
		}
	}
}
