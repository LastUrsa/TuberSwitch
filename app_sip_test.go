package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"TuberSwitch/internal/config"
	"TuberSwitch/internal/obs"
	"TuberSwitch/internal/secrets"
	"TuberSwitch/internal/sip"
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
						{RewardID: "dance", RewardName: "Dance", Enabled: true, Manageable: true},
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

func TestSIPActivateProfileReconcilesOutgoingRewardsAndOBSSources(t *testing.T) {
	fakeOBS := &fakeOBSService{sources: map[string][]obs.Source{
		"Old": {{Name: "OldPNG", SceneItemID: 10}},
		"New": {{Name: "NewVTuber", SceneItemID: 20}},
	}}
	fakeTwitch := &fakeTwitchService{}
	app := &App{
		store: config.NewStore(filepath.Join(t.TempDir(), "config.json")), secretStore: &fakeSecretStore{},
		logger: log.Default(), obs: fakeOBS, twitch: fakeTwitch,
		cfg: config.Config{
			OBS: config.OBSConfig{Host: "127.0.0.1", Port: 4455}, Twitch: config.TwitchConfig{AccessToken: "token"},
			ModeProfiles: config.DefaultProfiles(), CurrentMode: config.ModePNG, ActiveProfileID: "old",
			SceneMappings:  []config.SceneMapping{{Scene: "Old", Enabled: true, PNGTuberSource: "OldPNG", PNGTuberItemID: 10}},
			RewardMappings: []config.RewardMapping{{RewardID: "old", RewardName: "Old", Enabled: true, Manageable: true}},
			Profiles: []config.Profile{
				{ID: "old", Name: "Old", Mode: config.ModePNG},
				{ID: "new", Name: "New", Mode: config.Mode3D,
					SceneMappings:  []config.SceneMapping{{Scene: "New", Enabled: true, VTuberSource: "NewVTuber"}},
					RewardMappings: []config.RewardMapping{{RewardID: "new", RewardName: "New", Enabled: true, Manageable: true}}},
			},
		},
	}
	profile, err := app.SIPActivateProfile(context.Background(), "New")
	if err != nil || profile.ID != "new" {
		t.Fatalf("SIPActivateProfile = %+v, %v", profile, err)
	}
	wantOBS := []visibilityCall{{scene: "Old", source: "OldPNG", id: 10, enabled: false}, {scene: "New", source: "NewVTuber", id: 20, enabled: true}}
	if fmt.Sprint(fakeOBS.visibilityCalls) != fmt.Sprint(wantOBS) {
		t.Fatalf("OBS calls = %#v, want %#v", fakeOBS.visibilityCalls, wantOBS)
	}
	wantRewards := []rewardCall{{rewardID: "old", enabled: false}, {rewardID: "new", enabled: true}}
	if fmt.Sprint(fakeTwitch.rewardCalls) != fmt.Sprint(wantRewards) {
		t.Fatalf("Twitch calls = %#v, want %#v", fakeTwitch.rewardCalls, wantRewards)
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
						{RewardID: "dance", RewardName: "Dance", Enabled: true, Manageable: true},
						{RewardID: "hydrate", RewardName: "Hydrate", Manageable: false},
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
	if !details.RedeemsEnabled || details.RedeemCount != 2 || details.ManageableRedeemCount != 1 || details.UnmanageableRedeemCount != 1 {
		t.Fatalf("redeem details = %+v", details)
	}
	if !details.AppDetectionEnabled || details.AppDetectionStatus != "Enabled" {
		t.Fatalf("app detection details = %+v", details)
	}
	if details.CurrentModeLabel != "3D VTuber Mode" || details.ActiveProfileLastUsed != "2026-06-10T12:00:00Z" {
		t.Fatalf("profile details = %+v", details)
	}
}

func TestSIPRedeemsReadAndPersistActiveProfileRewardState(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	app := &App{
		store:  store,
		logger: log.Default(),
		cfg: config.Config{
			ModeProfiles:    config.DefaultProfiles(),
			Twitch:          config.TwitchConfig{AccessToken: "token"},
			CurrentMode:     config.Mode3D,
			ActiveProfileID: "gaming",
			Profiles: []config.Profile{
				{ID: config.DefaultProfileID, Name: "Default", Mode: config.ModePNG},
				{
					ID:   "gaming",
					Name: "Gaming Stream",
					Mode: config.Mode3D,
					RewardMappings: []config.RewardMapping{
						{RewardID: "headpat", RewardName: "Headpat", Enabled: true, Manageable: true},
						{RewardID: "hydrate", RewardName: "Hydrate", Enabled: false, Manageable: true},
					},
				},
			},
		},
	}

	redeems, err := app.SIPRedeems(context.Background())
	if err != nil {
		t.Fatalf("SIPRedeems: %v", err)
	}
	if len(redeems) != 2 || redeems[0].ID != "headpat" || !redeems[0].Available || !redeems[0].Enabled {
		t.Fatalf("redeems = %+v", redeems)
	}

	err = app.SIPSetRedeems(context.Background(), []sip.UpdateRedeemRequest{{ID: "headpat", Enabled: false}})
	if err != nil {
		t.Fatalf("SIPSetRedeems: %v", err)
	}
	if app.cfg.Profiles[1].RewardMappings[0].Enabled {
		t.Fatalf("profile reward was not updated: %+v", app.cfg.Profiles[1].RewardMappings)
	}
	if app.cfg.RewardMappings[0].Enabled {
		t.Fatalf("active reward snapshot was not updated: %+v", app.cfg.RewardMappings)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Profiles[1].RewardMappings[0].Enabled {
		t.Fatalf("persisted reward was not updated: %+v", loaded.Profiles[1].RewardMappings)
	}
}

func TestSIPRedeemsExposeUnavailableRewardsWhenTwitchDisconnected(t *testing.T) {
	app := &App{
		logger: log.Default(),
		cfg: config.Config{
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.ModePNG,
			ActiveProfileID: config.DefaultProfileID,
			Profiles: []config.Profile{
				{
					ID:   config.DefaultProfileID,
					Name: "Default",
					Mode: config.ModePNG,
					RewardMappings: []config.RewardMapping{
						{RewardID: "headpat", RewardName: "Headpat", Enabled: true, Manageable: true},
						{RewardID: "hydrate", RewardName: "Hydrate", Enabled: true, Manageable: false},
					},
				},
			},
		},
	}

	redeems, err := app.SIPRedeems(context.Background())
	if err != nil {
		t.Fatalf("SIPRedeems: %v", err)
	}
	if len(redeems) != 2 {
		t.Fatalf("redeems = %+v", redeems)
	}
	for _, redeem := range redeems {
		if !redeem.Enabled {
			t.Fatalf("enabled intent should remain visible: %+v", redeems)
		}
		if redeem.Available {
			t.Fatalf("redeem should be unavailable without Twitch connection: %+v", redeems)
		}
	}
}

func TestSIPSetRedeemsRejectsUnknownAndUnmanageableRewards(t *testing.T) {
	app := &App{
		logger: log.Default(),
		cfg: config.Config{
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.ModePNG,
			ActiveProfileID: config.DefaultProfileID,
			Profiles: []config.Profile{
				{
					ID:   config.DefaultProfileID,
					Name: "Default",
					Mode: config.ModePNG,
					RewardMappings: []config.RewardMapping{
						{RewardID: "readonly", RewardName: "Hydrate", Enabled: false, Manageable: false},
					},
				},
			},
		},
	}

	if err := app.SIPSetRedeems(context.Background(), []sip.UpdateRedeemRequest{{ID: "missing", Enabled: true}}); err != sip.ErrRedeemNotFound {
		t.Fatalf("missing err = %v", err)
	}
	if err := app.SIPSetRedeems(context.Background(), []sip.UpdateRedeemRequest{{ID: "readonly", Enabled: true}}); err != sip.ErrInvalidRequest {
		t.Fatalf("unmanageable err = %v", err)
	}
}

func TestSIPApplyRedeemsManualAppliesThroughTwitchWithoutSavingProfileIntent(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	fakeTwitch := &fakeTwitchService{}
	secretStore := &fakeSecretStore{}
	app := &App{
		store:       store,
		secretStore: secretStore,
		logger:      log.Default(),
		twitch:      fakeTwitch,
		cfg: config.Config{
			ModeProfiles:    config.DefaultProfiles(),
			Twitch:          config.TwitchConfig{ClientID: "client", AccessToken: "old-token"},
			CurrentMode:     config.ModePNG,
			ActiveProfileID: "gaming",
			Profiles: []config.Profile{
				{ID: config.DefaultProfileID, Name: "Default", Mode: config.ModePNG},
				{
					ID:   "gaming",
					Name: "Gaming Stream",
					Mode: config.ModePNG,
					RewardMappings: []config.RewardMapping{
						{RewardID: "headpat", RewardName: "Headpat", Enabled: true, Manageable: true},
					},
				},
			},
		},
	}

	if err := store.Save(app.cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := app.SIPApplyRedeemsManual(context.Background(), []sip.UpdateRedeemRequest{{ID: "headpat", Enabled: false}}); err != nil {
		t.Fatalf("SIPApplyRedeemsManual: %v", err)
	}

	if len(fakeTwitch.rewardCalls) != 1 || fakeTwitch.rewardCalls[0].rewardID != "headpat" || fakeTwitch.rewardCalls[0].enabled {
		t.Fatalf("reward calls = %#v", fakeTwitch.rewardCalls)
	}
	if !app.cfg.Profiles[1].RewardMappings[0].Enabled {
		t.Fatalf("profile reward intent was mutated: %+v", app.cfg.Profiles[1].RewardMappings)
	}
	if app.cfg.RewardMappings != nil {
		t.Fatalf("active reward snapshot was unexpectedly mutated: %+v", app.cfg.RewardMappings)
	}
	if secretStore.twitchTokens.AccessToken != "token" {
		t.Fatalf("refreshed token was not saved: %+v", secretStore.twitchTokens)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.Profiles[1].RewardMappings[0].Enabled {
		t.Fatalf("manual redeem update persisted profile intent: %+v", loaded.Profiles[1].RewardMappings)
	}
}

func TestSIPApplyRedeemsManualValidatesAllUpdatesBeforeTwitchChanges(t *testing.T) {
	fakeTwitch := &fakeTwitchService{}
	app := &App{
		secretStore: &fakeSecretStore{},
		logger:      log.Default(),
		twitch:      fakeTwitch,
		cfg: config.Config{
			ModeProfiles:    config.DefaultProfiles(),
			Twitch:          config.TwitchConfig{AccessToken: "token"},
			CurrentMode:     config.ModePNG,
			ActiveProfileID: config.DefaultProfileID,
			Profiles: []config.Profile{
				{
					ID:   config.DefaultProfileID,
					Name: "Default",
					Mode: config.ModePNG,
					RewardMappings: []config.RewardMapping{
						{RewardID: "headpat", RewardName: "Headpat", Enabled: true, Manageable: true},
						{RewardID: "readonly", RewardName: "Hydrate", Enabled: true, Manageable: false},
					},
				},
			},
		},
	}

	if err := app.SIPApplyRedeemsManual(context.Background(), []sip.UpdateRedeemRequest{{ID: "headpat", Enabled: false}, {ID: "missing", Enabled: true}}); err != sip.ErrRedeemNotFound {
		t.Fatalf("missing err = %v", err)
	}
	if len(fakeTwitch.rewardCalls) != 0 {
		t.Fatalf("unexpected calls after missing redeem: %#v", fakeTwitch.rewardCalls)
	}
	if err := app.SIPApplyRedeemsManual(context.Background(), []sip.UpdateRedeemRequest{{ID: "headpat", Enabled: false}, {ID: "readonly", Enabled: false}}); err != sip.ErrInvalidRequest {
		t.Fatalf("readonly err = %v", err)
	}
	if len(fakeTwitch.rewardCalls) != 0 {
		t.Fatalf("unexpected calls after readonly redeem: %#v", fakeTwitch.rewardCalls)
	}
}

func TestSIPApplyRedeemsManualRequiresTwitchAuthentication(t *testing.T) {
	fakeTwitch := &fakeTwitchService{}
	app := &App{
		secretStore: &fakeSecretStore{},
		logger:      log.Default(),
		twitch:      fakeTwitch,
		cfg: config.Config{
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.ModePNG,
			ActiveProfileID: config.DefaultProfileID,
			Profiles: []config.Profile{
				{
					ID:   config.DefaultProfileID,
					Name: "Default",
					Mode: config.ModePNG,
					RewardMappings: []config.RewardMapping{
						{RewardID: "headpat", RewardName: "Headpat", Enabled: true, Manageable: true},
					},
				},
			},
		},
	}

	if err := app.SIPApplyRedeemsManual(context.Background(), []sip.UpdateRedeemRequest{{ID: "headpat", Enabled: false}}); err == nil {
		t.Fatalf("expected Twitch auth error")
	}
	if len(fakeTwitch.rewardCalls) != 0 {
		t.Fatalf("unexpected calls without auth: %#v", fakeTwitch.rewardCalls)
	}
}

func TestSIPStatusDetailsReportUnavailableConfiguration(t *testing.T) {
	app := &App{
		logger: log.Default(),
		obs:    &fakeOBSService{},
		cfg: config.Config{
			OBS:             config.OBSConfig{},
			Twitch:          config.TwitchConfig{AccessToken: "token"},
			ModeProfiles:    config.DefaultProfiles(),
			CurrentMode:     config.ModePNG,
			ActiveProfileID: config.DefaultProfileID,
			Profiles: []config.Profile{
				{
					ID:   config.DefaultProfileID,
					Name: "Default",
					Mode: config.ModePNG,
					RewardMappings: []config.RewardMapping{
						{RewardID: "hydrate", RewardName: "Hydrate", Manageable: false},
					},
				},
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
	if details.RedeemsEnabled || details.RedeemCount != 1 || details.ManageableRedeemCount != 0 || details.UnmanageableRedeemCount != 1 {
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
