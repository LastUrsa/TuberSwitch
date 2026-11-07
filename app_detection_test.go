package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appdto "TuberSwitch/internal/app"
	"TuberSwitch/internal/appdetect"
	"TuberSwitch/internal/config"
	"TuberSwitch/internal/obs"
	"TuberSwitch/internal/sip"
)

func detectionTestApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Default()
	cfg.Profiles = []config.Profile{
		{ID: "default", Name: "Default", Mode: config.Mode3D, LastUsed: "2026-11-07T12:00:00Z"},
		{ID: "avatar", Name: "Avatar", Mode: config.Mode3D},
		{ID: "png", Name: "PNG", Mode: config.ModePNG},
		{ID: "other-png", Name: "Other PNG", Mode: config.ModePNG, LastUsed: "2026-11-07T12:00:00Z"},
	}
	cfg.SceneMappings = []config.SceneMapping{{Scene: "Main", Enabled: true, VTuberSource: "Avatar", PNGTuberSource: "PNG"}}
	for i := range cfg.Profiles {
		cfg.Profiles[i].SceneMappings = append([]config.SceneMapping(nil), cfg.SceneMappings...)
	}
	cfg.CurrentMode = config.Mode3D
	cfg.AppDetection.ThreeDProfileID = "avatar"
	cfg.AppDetection.PNGProfileID = "png"
	return &App{cfg: cfg, store: config.NewStore(filepath.Join(t.TempDir(), "config.json")), secretStore: &fakeSecretStore{}, obs: &fakeOBSService{sources: map[string][]obs.Source{"Main": {{Name: "Avatar", SceneItemID: 10}, {Name: "PNG", SceneItemID: 11}}}}, twitch: &fakeTwitchService{}, logger: log.Default()}
}

func TestConfiguredDetectionTargetsOverrideRecencyAndSameModeDefault(t *testing.T) {
	app := detectionTestApp(t)
	target, err := app.resolveDetectionTarget(config.Mode3D)
	if err != nil || target.Active || target.ProfileID != "avatar" {
		t.Fatalf("target=%+v err=%v", target, err)
	}
	if err := app.applyModeFromDetection(config.Mode3D, false); err != nil {
		t.Fatal(err)
	}
	if app.cfg.ActiveProfileID != "avatar" || app.cfg.CurrentMode != config.Mode3D {
		t.Fatal("explicit 3D target not applied")
	}
	target, err = app.resolveDetectionTarget(config.Mode3D)
	if err != nil || !target.Active {
		t.Fatal("applied target not recognized")
	}
	if err := app.applyModeFromDetection(config.ModePNG, false); err != nil {
		t.Fatal(err)
	}
	if app.cfg.ActiveProfileID != "png" || app.cfg.CurrentMode != config.ModePNG {
		t.Fatal("explicit PNG target not applied")
	}
	current, err := app.SIPCurrentProfile(context.Background())
	if err != nil || current.ID != "png" {
		t.Fatal("SIP profile did not reflect detection")
	}
}

func TestMissingOrMismatchedDetectionTargetDoesNotFallback(t *testing.T) {
	for _, test := range []struct {
		id  string
		err error
	}{{"missing", appdetect.ErrTargetMissing}, {"png", appdetect.ErrTargetModeMismatch}} {
		t.Run(test.id, func(t *testing.T) {
			app := detectionTestApp(t)
			app.cfg.AppDetection.ThreeDProfileID = test.id
			if err := app.applyModeFromDetection(config.Mode3D, false); !errors.Is(err, test.err) {
				t.Fatalf("error=%v", err)
			}
			if app.cfg.ActiveProfileID != "default" {
				t.Fatal("invalid target changed active profile")
			}
		})
	}
}

func TestBlankDetectionTargetsPreserveLegacyBehavior(t *testing.T) {
	app := detectionTestApp(t)
	app.cfg.AppDetection.ThreeDProfileID = ""
	app.cfg.AppDetection.PNGProfileID = ""
	target, err := app.resolveDetectionTarget(config.Mode3D)
	if err != nil || !target.Active {
		t.Fatal("legacy same-mode check changed")
	}
	if err := app.applyModeFromDetection(config.ModePNG, false); err != nil {
		t.Fatal(err)
	}
	if app.cfg.ActiveProfileID != "other-png" {
		t.Fatal("legacy recency behavior changed")
	}
}

func TestSaveRejectsInvalidDetectionTargetsAndKeepsPreviousConfig(t *testing.T) {
	for _, id := range []string{"missing", "png"} {
		t.Run(id, func(t *testing.T) {
			app := detectionTestApp(t)
			settings := app.settingsLocked()
			settings.AppDetection.ThreeDProfileID = id
			result := app.SaveConfig(appdto.SettingsInput{Config: settings})
			if result.OK || len(result.Errors) == 0 {
				t.Fatal("invalid target accepted")
			}
			if app.cfg.AppDetection.ThreeDProfileID != "avatar" {
				t.Fatal("rejected save mutated configuration")
			}
		})
	}
	// Editing a target's saved mode must validate the final profile snapshot.
	app := detectionTestApp(t)
	app.cfg.ApplyStreamProfile(app.cfg.Profiles[1])
	settings := app.settingsLocked()
	settings.CurrentMode = config.ModePNG
	result := app.SaveProfile(appdto.SettingsInput{Config: settings})
	if result.OK || app.cfg.Profiles[1].Mode != config.Mode3D {
		t.Fatal("target mode mismatch accepted or rejected save mutated profile")
	}
}

func TestDeletingDetectionTargetKeepsReferenceAndBlocksFallback(t *testing.T) {
	app := detectionTestApp(t)
	if result := app.DeleteProfile("avatar"); !result.OK {
		t.Fatalf("delete: %+v", result)
	}
	target, err := app.resolveDetectionTarget(config.Mode3D)
	if !errors.Is(err, appdetect.ErrTargetMissing) || target.ProfileID != "avatar" {
		t.Fatalf("deleted target silently fell back: %+v %v", target, err)
	}
}

func TestDetectionTargetsPersistAcrossRestart(t *testing.T) {
	app := detectionTestApp(t)
	result := app.SaveConfig(appdto.SettingsInput{Config: app.settingsLocked()})
	if !result.OK {
		t.Fatalf("save: %+v", result)
	}
	loaded, err := app.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AppDetection.ThreeDProfileID != "avatar" || loaded.AppDetection.PNGProfileID != "png" {
		t.Fatal("targets not persisted")
	}
}

type fixedDetectionStub struct {
	appDetectorStub
	snapshot appdetect.Snapshot
}

func (s fixedDetectionStub) Snapshot() appdetect.Snapshot { return s.snapshot }

func TestSIPDetectionDiagnosticsAreSafeProducerObservations(t *testing.T) {
	app := detectionTestApp(t)
	observed := "2026-11-07T12:00:00Z"
	app.detector = fixedDetectionStub{snapshot: appdetect.Snapshot{Status: appdetect.StatusThreeDDetected, DetectedNames: []string{"private-process.exe"}, Diagnostics: appdetect.Diagnostics{DesiredMode: config.Mode3D, TargetProfileID: "avatar", Reason: "manual_cooldown", ObservedAt: observed, CooldownUntil: "2026-11-07T12:00:15Z", ThreeDRunning: true}}}
	details, err := app.SIPStatusDetails(context.Background())
	if err != nil || !strings.Contains(details.AppDetectionStatus, "manual override cooldown") {
		t.Fatal("legacy status did not expose safe block reason")
	}
	service := sip.NewService(sip.AppInfo{Version: "test", Mode: sip.ServiceMode}, app)
	for _, route := range []string{"/api/v1/status", "/api/v1/diagnostics"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+route, nil)
		service.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %s", route, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "private-process") {
			t.Fatal("process inventory leaked into SIP")
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if route == "/api/v1/diagnostics" {
			if err := json.Unmarshal(payload["dependencies"], &payload); err != nil {
				t.Fatal(err)
			}
		}
		var d appdetect.Diagnostics
		if err := json.Unmarshal(payload["appDetection"], &d); err != nil {
			t.Fatal(err)
		}
		if d.Reason != "manual_cooldown" || d.TargetProfileID != "avatar" || d.ObservedAt != observed || !d.ThreeDRunning {
			t.Fatalf("diagnostics=%+v", d)
		}
	}
}

func TestDetectionLoopUpdatesSIPProfileInServiceMode(t *testing.T) {
	app := detectionTestApp(t)
	app.runtimeMode = sip.ServiceMode
	app.cfg.AppDetection.Enabled = true
	app.cfg.AppDetection.ThreeDProcessName = "vnyan.exe"
	app.detector = appdetect.New(app.logger, &fakeProcessProvider{processes: []appdetect.ProcessSummary{{ProcessName: "VNYAN.EXE"}}}, app.applyModeFromDetection, app.currentMode, app.resolveDetectionTarget)
	app.detector.Start(app.cfg.AppDetection)
	defer app.detector.Stop()
	service := sip.NewService(sip.AppInfo{Mode: sip.ServiceMode}, app)
	deadline := time.Now().Add(2 * time.Second)
	for {
		status, err := service.Status(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if status.ActiveProfileID == "avatar" && status.ActiveMode == "3d" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("auto detection did not reach SIP: %+v", status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAutomaticTwitchWarningIsReportedAsIntegrationFailure(t *testing.T) {
	app := detectionTestApp(t)
	app.cfg.Twitch.AccessToken = "test-token"
	app.cfg.Profiles[1].RewardMappings = []config.RewardMapping{{RewardID: "reward", RewardName: "Test reward", Enabled: true, Manageable: true}}
	app.twitch = &fakeTwitchService{rewardErrors: map[string]error{"reward": errors.New("test integration failure")}}
	if err := app.applyModeFromDetection(config.Mode3D, true); err == nil {
		t.Fatal("partial Twitch failure was reported as successful detection")
	}
	if app.cfg.ActiveProfileID != "avatar" {
		t.Fatal("non-fatal Twitch warning lost active target")
	}
}
