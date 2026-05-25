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

	cfg.OBS.Host = "localhost"
	cfg.OBS.Port = 0
	cfg.StartupMode = ""
	cfg.CurrentMode = ""
	cfg.ModeProfiles = nil
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
		t.Fatalf("mode profiles = %d", len(cfg.ModeProfiles))
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
