package main

import (
	"context"
	"log"
	"path/filepath"
	"testing"

	"TuberSwitch/internal/config"
	"TuberSwitch/internal/obs"
	"TuberSwitch/internal/secrets"
)

func TestSIPActivateProfileUsesExistingProfileActivationPath(t *testing.T) {
	fakeOBS := &fakeOBSService{
		sources: map[string][]obs.Source{
			"Gaming": {{Name: "VTuber", SceneItemID: 10}, {Name: "PNG", SceneItemID: 11}},
		},
	}
	fakeTwitch := &fakeTwitchService{}
	app := &App{
		store:       config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		secretStore: &fakeSecretStore{twitchTokens: secrets.TwitchTokens{AccessToken: "token"}},
		logger:      log.Default(),
		obs:         fakeOBS,
		twitch:      fakeTwitch,
		cfg: config.Config{
			OBS:             config.OBSConfig{Host: "127.0.0.1", Port: 4455},
			Twitch:          config.TwitchConfig{ClientID: "client", AccessToken: "token"},
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.ModePNG,
			ActiveProfileID: config.DefaultProfileID,
			Profiles: []config.Profile{
				{ID: config.DefaultProfileID, Name: "Default", Mode: config.ModePNG},
				{
					ID:   "gaming",
					Name: "Gaming Stream",
					Mode: config.Mode3D,
					SceneMappings: []config.SceneMapping{
						{Scene: "Gaming", Enabled: true, VTuberSource: "VTuber", PNGTuberSource: "PNG"},
					},
					RewardMappings: []config.RewardMapping{
						{RewardID: "dance", RewardName: "Dance", Is3DOnly: true, Manageable: true},
					},
				},
			},
		},
	}

	profile, err := app.SIPActivateProfile(context.Background(), "gaming stream")
	if err != nil {
		t.Fatalf("SIPActivateProfile: %v", err)
	}
	if profile.ID != "gaming" || profile.Name != "Gaming Stream" || profile.Mode != "3d" {
		t.Fatalf("profile = %+v", profile)
	}
	if app.cfg.ActiveProfileID != "gaming" || app.cfg.CurrentMode != config.Mode3D {
		t.Fatalf("profile/mode = %q/%q", app.cfg.ActiveProfileID, app.cfg.CurrentMode)
	}
	if len(fakeOBS.visibilityCalls) != 2 {
		t.Fatalf("visibility calls = %#v", fakeOBS.visibilityCalls)
	}
	if len(fakeTwitch.rewardCalls) != 1 || fakeTwitch.rewardCalls[0].rewardID != "dance" || !fakeTwitch.rewardCalls[0].enabled {
		t.Fatalf("reward calls = %#v", fakeTwitch.rewardCalls)
	}
}

func TestSIPProfileAccessors(t *testing.T) {
	app := &App{
		logger: log.Default(),
		cfg: config.Config{
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.ModePNG,
			ActiveProfileID: "chat",
			Profiles: []config.Profile{
				{ID: config.DefaultProfileID, Name: "Default", Mode: config.ModePNG},
				{ID: "chat", Name: "Just Chatting", Mode: config.ModePNG},
			},
		},
	}

	profiles, err := app.SIPProfiles(context.Background())
	if err != nil {
		t.Fatalf("SIPProfiles: %v", err)
	}
	if len(profiles) != 2 || profiles[1].Name != "Just Chatting" {
		t.Fatalf("profiles = %+v", profiles)
	}
	current, err := app.SIPCurrentProfile(context.Background())
	if err != nil {
		t.Fatalf("SIPCurrentProfile: %v", err)
	}
	if current.ID != "chat" || current.Name != "Just Chatting" || current.Mode != "png" {
		t.Fatalf("current = %+v", current)
	}
}

func TestSIPStatusDetailsExposeRuntimeDrawerFields(t *testing.T) {
	app := &App{
		logger: log.Default(),
		obs:    &fakeOBSService{connected: true},
		cfg: config.Config{
			OBS:             config.OBSConfig{Host: "127.0.0.1", Port: 4455},
			Twitch:          config.TwitchConfig{AccessToken: "token"},
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.Mode3D,
			ActiveProfileID: "gaming",
			AppDetection:    config.AppDetectionConfig{Enabled: true},
			Profiles: []config.Profile{
				{
					ID:       "gaming",
					Name:     "Gaming Stream",
					Mode:     config.Mode3D,
					LastUsed: "2026-06-10T12:00:00Z",
					SceneMappings: []config.SceneMapping{
						{Scene: "Disabled", Enabled: false, VTuberSource: "Unused", PNGTuberSource: "Unused"},
						{Scene: "Gaming", Enabled: true, VTuberSource: "VTuber", PNGTuberSource: "PNG"},
					},
					RewardMappings: []config.RewardMapping{
						{RewardID: "dance", RewardName: "Dance", Is3DOnly: true, Manageable: true},
					},
				},
			},
		},
	}

	details, err := app.SIPStatusDetails(context.Background())
	if err != nil {
		t.Fatalf("SIPStatusDetails: %v", err)
	}
	if !details.OBSConnected || details.OBSSummary != "Connected: Gaming / VTuber" {
		t.Fatalf("obs details = %+v", details)
	}
	if details.ActiveScene != "Gaming" || details.ActiveSource != "VTuber" {
		t.Fatalf("active scene/source = %+v", details)
	}
	if !details.RedeemsEnabled || details.RedeemCount != 1 {
		t.Fatalf("redeem details = %+v", details)
	}
	if !details.AppDetectionEnabled || details.AppDetectionStatus != "Enabled" {
		t.Fatalf("app detection details = %+v", details)
	}
	if details.CurrentModeLabel != "3D VTuber Mode" || details.ActiveProfileLastUsed != "2026-06-10T12:00:00Z" {
		t.Fatalf("profile details = %+v", details)
	}
}

func TestSIPStatusDetailsReportUnavailableConfiguration(t *testing.T) {
	app := &App{
		logger: log.Default(),
		obs:    &fakeOBSService{},
		cfg: config.Config{
			OBS:             config.OBSConfig{},
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.ModePNG,
			ActiveProfileID: config.DefaultProfileID,
			Profiles: []config.Profile{
				{ID: config.DefaultProfileID, Name: "Default", Mode: config.ModePNG},
			},
		},
	}

	details, err := app.SIPStatusDetails(context.Background())
	if err != nil {
		t.Fatalf("SIPStatusDetails: %v", err)
	}
	if details.OBSConnected || details.OBSSummary != "OBS not configured" {
		t.Fatalf("obs details = %+v", details)
	}
	if details.RedeemsEnabled || details.RedeemCount != 0 {
		t.Fatalf("redeem details = %+v", details)
	}
}

func TestSIPActivateProfileRejectsUnknownProfile(t *testing.T) {
	app := &App{
		logger: log.Default(),
		cfg: config.Config{
			Profiles: []config.Profile{{ID: config.DefaultProfileID, Name: "Default", Mode: config.ModePNG}},
		},
	}

	if _, err := app.SIPActivateProfile(context.Background(), "Missing"); err == nil {
		t.Fatalf("expected missing profile error")
	}
}
