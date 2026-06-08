package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultAndNormalize(t *testing.T) {
	cfg := Default()
	if cfg.OBS.Host != "127.0.0.1" {
		t.Fatalf("default OBS host = %q", cfg.OBS.Host)
	}
	if cfg.OBS.Port != 4455 {
		t.Fatalf("default OBS port = %d", cfg.OBS.Port)
	}
	if cfg.AppDetection.ThreeDProcessName != "" {
		t.Fatalf("default 3D process = %q", cfg.AppDetection.ThreeDProcessName)
	}
	if cfg.AppDetection.PNGProcessName != "" {
		t.Fatalf("default PNG process = %q", cfg.AppDetection.PNGProcessName)
	}
	if cfg.AppDetection.IntervalSeconds != 5 {
		t.Fatalf("default detection interval = %d", cfg.AppDetection.IntervalSeconds)
	}
	if !cfg.AppDetection.ApplyTwitchChanges {
		t.Fatalf("default apply twitch changes should be true")
	}
	if cfg.ActiveProfileID != DefaultProfileID {
		t.Fatalf("default active profile = %q", cfg.ActiveProfileID)
	}
	if len(cfg.Profiles) != 0 {
		t.Fatalf("default profiles before normalize = %#v", cfg.Profiles)
	}

	cfg.OBS.Host = "localhost"
	cfg.OBS.Port = 0
	cfg.StartupMode = ""
	cfg.CurrentMode = ""
	cfg.ModeProfiles = []ModeProfile{
		{ID: Mode3D, DisplayName: "3D VTuber Mode"},
		{ID: ModePNG, DisplayName: "PNG " + "VTuber Mode"},
	}
	cfg.AppDetection.IntervalSeconds = 1
	cfg.AppDetection.ConflictBehavior = "invalid"
	cfg.AppDetection.ManualOverrideCooldownSeconds = -1
	cfg.AppDetection.ThreeDProcessName = `  C:\Apps\AvatarApp.exe  `
	cfg.Normalize()

	if cfg.OBS.Host != "127.0.0.1" {
		t.Fatalf("normalized host = %q", cfg.OBS.Host)
	}
	if cfg.OBS.Port != 4455 {
		t.Fatalf("normalized port = %d", cfg.OBS.Port)
	}
	if cfg.StartupMode != StartupRestoreLast {
		t.Fatalf("startup mode = %q", cfg.StartupMode)
	}
	if cfg.CurrentMode != ModePNG {
		t.Fatalf("current mode = %q", cfg.CurrentMode)
	}
	if len(cfg.ModeProfiles) != 2 {
		t.Fatalf("mode profiles = %#v", cfg.ModeProfiles)
	}
	if cfg.ModeProfiles[1].DisplayName != "PNGTuber Mode" {
		t.Fatalf("mode display name = %q", cfg.ModeProfiles[1].DisplayName)
	}
	if len(cfg.Profiles) == 0 || cfg.Profiles[0].ID != DefaultProfileID || cfg.Profiles[0].Name != "Default" {
		t.Fatalf("profiles = %#v", cfg.Profiles)
	}
	if cfg.AppDetection.IntervalSeconds != 2 {
		t.Fatalf("normalized detection interval = %d", cfg.AppDetection.IntervalSeconds)
	}
	if cfg.AppDetection.ConflictBehavior != AppDetectionConflictDoNothing {
		t.Fatalf("normalized conflict behavior = %q", cfg.AppDetection.ConflictBehavior)
	}
	if cfg.AppDetection.ManualOverrideCooldownSeconds != 0 {
		t.Fatalf("normalized manual cooldown = %d", cfg.AppDetection.ManualOverrideCooldownSeconds)
	}
	if cfg.AppDetection.ThreeDProcessName != "AvatarApp.exe" {
		t.Fatalf("normalized 3D process = %q", cfg.AppDetection.ThreeDProcessName)
	}
}

func TestNormalizeMigratesLegacySingleSceneConfig(t *testing.T) {
	cfg := Default()
	cfg.Sources = SourcesConfig{
		Scene:          "Main",
		VTuberSource:   "VTuber",
		VTuberItemID:   10,
		PNGTuberSource: "PNG",
		PNGTuberItemID: 11,
	}
	cfg.SceneMappings = nil

	cfg.Normalize()

	if len(cfg.SceneMappings) != 1 {
		t.Fatalf("scene mappings = %d", len(cfg.SceneMappings))
	}
	mapping := cfg.SceneMappings[0]
	if mapping.Scene != "Main" || mapping.VTuberSource != "VTuber" || mapping.PNGTuberSource != "PNG" {
		t.Fatalf("unexpected mapping: %#v", mapping)
	}
	if !mapping.Enabled {
		t.Fatalf("migrated mapping should be enabled")
	}
}

func TestNormalizeCreatesDefaultProfileFromCurrentConfiguration(t *testing.T) {
	cfg := Default()
	cfg.CurrentMode = Mode3D
	cfg.SceneMappings = []SceneMapping{{Scene: "Main", Enabled: true, VTuberSource: "VTuber"}}
	cfg.RewardMappings = []RewardMapping{{RewardID: "dance", RewardName: "Dance", Is3DOnly: true, Manageable: true}}

	cfg.Normalize()

	profile, ok := cfg.ActiveProfile()
	if !ok {
		t.Fatalf("active profile missing: %#v", cfg.Profiles)
	}
	if profile.ID != DefaultProfileID || profile.Mode != Mode3D {
		t.Fatalf("profile = %#v", profile)
	}
	if len(profile.SceneMappings) != 1 || profile.SceneMappings[0].Scene != "Main" {
		t.Fatalf("profile scene mappings = %#v", profile.SceneMappings)
	}
	if len(profile.RewardMappings) != 1 || profile.RewardMappings[0].RewardID != "dance" {
		t.Fatalf("profile reward mappings = %#v", profile.RewardMappings)
	}
}

func TestStoreLoadCreatesDefaultAndSaveRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}
	if cfg.OBS.Host != "127.0.0.1" {
		t.Fatalf("default host = %q", cfg.OBS.Host)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default config was not written: %v", err)
	}

	cfg.Twitch.ChannelName = "Streamer"
	cfg.RewardMappings = []RewardMapping{{RewardID: "1", RewardName: "Dance", Is3DOnly: true, Manageable: true}}
	cfg.Profiles = []Profile{{ID: DefaultProfileID, Name: "Default", Mode: ModePNG}, {ID: "gaming", Name: "Gaming", Mode: Mode3D}}
	cfg.ActiveProfileID = "gaming"
	cfg.AppDetection.Enabled = true
	cfg.AppDetection.ThreeDProcessName = "custom-3d.exe"
	cfg.AppDetection.ApplyTwitchChanges = false
	if err := store.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Twitch.ChannelName != "Streamer" {
		t.Fatalf("channel name = %q", loaded.Twitch.ChannelName)
	}
	if len(loaded.RewardMappings) != 1 || !loaded.RewardMappings[0].Manageable {
		t.Fatalf("reward mappings = %#v", loaded.RewardMappings)
	}
	if loaded.ActiveProfileID != "gaming" || len(loaded.Profiles) != 2 {
		t.Fatalf("profiles not round-tripped: %#v active=%q", loaded.Profiles, loaded.ActiveProfileID)
	}
	if !loaded.AppDetection.Enabled || loaded.AppDetection.ThreeDProcessName != "custom-3d.exe" || loaded.AppDetection.ApplyTwitchChanges {
		t.Fatalf("app detection = %#v", loaded.AppDetection)
	}
}

func TestStoreLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := NewStore(path).Load()
	if err == nil {
		t.Fatalf("expected invalid JSON error")
	}
	if _, ok := err.(*json.SyntaxError); !ok {
		t.Fatalf("expected syntax error, got %T %v", err, err)
	}
}

func TestStoreSaveDoesNotPersistSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := NewStore(path)
	cfg := Default()
	cfg.OBS.Password = "obs-secret"
	cfg.Twitch.AccessToken = "access-token"
	cfg.Twitch.RefreshToken = "refresh-token"
	cfg.Twitch.TokenExpiry = "2026-01-01T00:00:00Z"

	if err := store.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "obs-secret") || strings.Contains(text, "access-token") || strings.Contains(text, "refresh-token") {
		t.Fatalf("secrets were persisted: %s", text)
	}
}
