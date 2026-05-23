package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appdto "TuberSwitch/internal/app"
	"TuberSwitch/internal/config"
	"TuberSwitch/internal/obs"
	"TuberSwitch/internal/secrets"
	"TuberSwitch/internal/twitch"
)

func TestMergeSceneMappingsPreservesExistingSelections(t *testing.T) {
	app := &App{
		cfg: config.Config{
			SceneMappings: []config.SceneMapping{
				{Scene: "Main", Enabled: true, VTuberSource: "VTuber", PNGTuberSource: "PNG"},
			},
		},
	}
	app.mergeSceneMappingsLocked([]obs.Scene{{Name: "Main"}, {Name: "BRB"}})

	if len(app.cfg.SceneMappings) != 2 {
		t.Fatalf("mappings = %#v", app.cfg.SceneMappings)
	}
	if app.cfg.SceneMappings[0].VTuberSource != "VTuber" {
		t.Fatalf("existing mapping not preserved: %#v", app.cfg.SceneMappings[0])
	}
	if app.cfg.SceneMappings[1].Scene != "BRB" {
		t.Fatalf("new mapping = %#v", app.cfg.SceneMappings[1])
	}
}

func TestFirstConfiguredScene(t *testing.T) {
	cfg := config.Config{SceneMappings: []config.SceneMapping{{Scene: ""}, {Scene: "Main"}}, Sources: config.SourcesConfig{Scene: "Legacy"}}
	if got := firstConfiguredScene(cfg); got != "Main" {
		t.Fatalf("firstConfiguredScene = %q", got)
	}
	cfg.SceneMappings = nil
	if got := firstConfiguredScene(cfg); got != "Legacy" {
		t.Fatalf("legacy firstConfiguredScene = %q", got)
	}
}

func TestFindSourceID(t *testing.T) {
	sources := []obs.Source{{Name: "VTuber", SceneItemID: 42}}
	if got := findSourceID(sources, "VTuber"); got != 42 {
		t.Fatalf("findSourceID = %d", got)
	}
	if got := findSourceID(sources, "Missing"); got != 0 {
		t.Fatalf("missing findSourceID = %d", got)
	}
}

func TestUnmanageableRewardErrorDetection(t *testing.T) {
	err := fakeError("The ID in the Client-Id header must match the client ID used to create the custom reward")
	if !isUnmanageableRewardError(err) {
		t.Fatalf("expected unmanageable reward error")
	}
	if isUnmanageableRewardError(fakeError("network failed")) {
		t.Fatalf("unexpected unmanageable reward match")
	}
}

func TestUpsertRewardMapping(t *testing.T) {
	app := &App{logger: log.Default()}
	app.upsertRewardMappingLocked(twitch.Reward{ID: "1", Title: "Dance"}, true)
	app.upsertRewardMappingLocked(twitch.Reward{ID: "1", Title: "Dance Updated"}, false)

	if len(app.cfg.RewardMappings) != 1 {
		t.Fatalf("mappings = %#v", app.cfg.RewardMappings)
	}
	mapping := app.cfg.RewardMappings[0]
	if mapping.RewardName != "Dance Updated" || mapping.Manageable {
		t.Fatalf("mapping = %#v", mapping)
	}
}

func TestApplyOBSModeTogglesSelectedSourcesAcrossScenes(t *testing.T) {
	fakeOBS := &fakeOBSService{
		sources: map[string][]obs.Source{
			"Main": {{Name: "VTuber", SceneItemID: 10}, {Name: "PNG", SceneItemID: 11}},
			"BRB":  {{Name: "PNG", SceneItemID: 20}},
		},
	}
	app := &App{
		obs:    fakeOBS,
		logger: log.Default(),
		cfg: config.Config{
			OBS:          config.OBSConfig{Host: "127.0.0.1", Port: 4455},
			ModeProfiles: config.DefaultProfiles(),
			SceneMappings: []config.SceneMapping{
				{Scene: "Main", Enabled: true, VTuberSource: "VTuber", PNGTuberSource: "PNG"},
				{Scene: "BRB", Enabled: true, PNGTuberSource: "PNG"},
				{Scene: "Disabled", Enabled: false, VTuberSource: "Ignored"},
			},
		},
	}

	if err := app.applyOBSMode(config.Mode3D); err != nil {
		t.Fatalf("applyOBSMode: %v", err)
	}

	want := []visibilityCall{
		{scene: "Main", source: "VTuber", id: 10, enabled: true},
		{scene: "Main", source: "PNG", id: 11, enabled: false},
		{scene: "BRB", source: "PNG", id: 20, enabled: false},
	}
	if fmt.Sprint(fakeOBS.visibilityCalls) != fmt.Sprint(want) {
		t.Fatalf("calls = %#v, want %#v", fakeOBS.visibilityCalls, want)
	}
}

func TestApplyTwitchModeOnlyUpdatesManageable3DRewards(t *testing.T) {
	fakeTwitch := &fakeTwitchService{}
	app := &App{
		twitch:      fakeTwitch,
		secretStore: &fakeSecretStore{},
		logger:      log.Default(),
		cfg: config.Config{
			Twitch:       config.TwitchConfig{ClientID: "client", AccessToken: "token"},
			ModeProfiles: config.DefaultProfiles(),
			RewardMappings: []config.RewardMapping{
				{RewardID: "manageable", RewardName: "Dance", Is3DOnly: true, Manageable: true},
				{RewardID: "readonly", RewardName: "Hydrate", Is3DOnly: true, Manageable: false},
				{RewardID: "not-3d", RewardName: "Hello", Is3DOnly: false, Manageable: true},
			},
		},
	}

	errors := app.applyTwitchModeLocked(config.ModePNG)
	if len(errors) != 0 {
		t.Fatalf("errors = %#v", errors)
	}
	if len(fakeTwitch.rewardCalls) != 1 {
		t.Fatalf("reward calls = %#v", fakeTwitch.rewardCalls)
	}
	call := fakeTwitch.rewardCalls[0]
	if call.rewardID != "manageable" || call.enabled {
		t.Fatalf("call = %#v", call)
	}
}

func TestSetReward3DOnlyBlocksUnmanageableReward(t *testing.T) {
	app := &App{
		obs:    &fakeOBSService{},
		store:  config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		logger: log.Default(),
		cfg: config.Config{
			RewardMappings: []config.RewardMapping{
				{RewardID: "readonly", RewardName: "Hydrate", Manageable: false},
			},
		},
	}

	result := app.SetReward3DOnly("readonly", true)
	if result.OK {
		t.Fatalf("expected failure")
	}
	if app.cfg.RewardMappings[0].Is3DOnly {
		t.Fatalf("unmanageable reward was marked 3D-only")
	}
}

func TestRefreshRewardsMarksManageableAndClearsReadonly3DOnly(t *testing.T) {
	fakeTwitch := &fakeTwitchService{
		allRewards: []twitch.Reward{
			{ID: "manageable", Title: "Dance"},
			{ID: "readonly", Title: "Hydrate"},
		},
		manageableRewards: []twitch.Reward{
			{ID: "manageable", Title: "Dance", Manageable: true},
		},
	}
	app := &App{
		obs:         &fakeOBSService{},
		twitch:      fakeTwitch,
		store:       config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		secretStore: &fakeSecretStore{},
		logger:      log.Default(),
		cfg: config.Config{
			Twitch: config.TwitchConfig{ClientID: "client", AccessToken: "token"},
			RewardMappings: []config.RewardMapping{
				{RewardID: "readonly", RewardName: "Hydrate", Is3DOnly: true, Manageable: true},
			},
		},
	}

	rewards, err := app.refreshRewards(context.Background())
	if err != nil {
		t.Fatalf("refreshRewards: %v", err)
	}
	if len(rewards) != 2 {
		t.Fatalf("rewards = %#v", rewards)
	}
	byID := map[string]config.RewardMapping{}
	for _, mapping := range app.cfg.RewardMappings {
		byID[mapping.RewardID] = mapping
	}
	if !byID["manageable"].Manageable {
		t.Fatalf("manageable reward not marked manageable: %#v", byID["manageable"])
	}
	if byID["readonly"].Manageable || byID["readonly"].Is3DOnly {
		t.Fatalf("readonly reward should be unmanageable and not 3D-only: %#v", byID["readonly"])
	}
}

func TestCreateTwitchRewardPersistsManageableMapping(t *testing.T) {
	fakeTwitch := &fakeTwitchService{
		createdReward: twitch.Reward{ID: "new", Title: "Throw Tomato", Manageable: true},
	}
	app := &App{
		obs:         &fakeOBSService{},
		twitch:      fakeTwitch,
		store:       config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		secretStore: &fakeSecretStore{},
		logger:      log.Default(),
		cfg:         config.Config{Twitch: config.TwitchConfig{ClientID: "client", AccessToken: "token"}},
	}

	result := app.CreateTwitchReward("Throw Tomato", 500, "")
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if len(app.cfg.RewardMappings) != 1 || app.cfg.RewardMappings[0].RewardID != "new" || !app.cfg.RewardMappings[0].Manageable {
		t.Fatalf("mappings = %#v", app.cfg.RewardMappings)
	}
}

func TestApplyModeReportsTwitchFailureAndPersistsMode(t *testing.T) {
	fakeOBS := &fakeOBSService{
		sources: map[string][]obs.Source{
			"Main": {{Name: "VTuber", SceneItemID: 10}},
		},
	}
	fakeTwitch := &fakeTwitchService{rewardErrors: map[string]error{"fail": fakeError("boom")}}
	app := &App{
		obs:         fakeOBS,
		twitch:      fakeTwitch,
		store:       config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		secretStore: &fakeSecretStore{},
		logger:      log.Default(),
		cfg: config.Config{
			OBS:          config.OBSConfig{Host: "127.0.0.1", Port: 4455},
			Twitch:       config.TwitchConfig{ClientID: "client", AccessToken: "token"},
			ModeProfiles: config.DefaultProfiles(),
			CurrentMode:  config.ModePNG,
			SceneMappings: []config.SceneMapping{
				{Scene: "Main", Enabled: true, VTuberSource: "VTuber"},
			},
			RewardMappings: []config.RewardMapping{
				{RewardID: "ok", RewardName: "OK", Is3DOnly: true, Manageable: true},
				{RewardID: "fail", RewardName: "Fail", Is3DOnly: true, Manageable: true},
			},
		},
	}

	result := app.ApplyMode(config.Mode3D)
	if result.OK {
		t.Fatalf("expected partial failure result")
	}
	if app.cfg.CurrentMode != config.Mode3D {
		t.Fatalf("current mode = %q", app.cfg.CurrentMode)
	}
	if len(fakeTwitch.rewardCalls) != 2 {
		t.Fatalf("reward calls = %#v", fakeTwitch.rewardCalls)
	}
}

func TestApplyModeReportsOBSFailureAndPersistsMode(t *testing.T) {
	fakeOBS := &fakeOBSService{
		sources: map[string][]obs.Source{
			"Main": {{Name: "VTuber", SceneItemID: 10}},
		},
		visibilityErrors: map[string]error{"Main/VTuber": fakeError("obs failed")},
	}
	app := &App{
		obs:         fakeOBS,
		twitch:      &fakeTwitchService{},
		store:       config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		secretStore: &fakeSecretStore{},
		logger:      log.Default(),
		cfg: config.Config{
			OBS:          config.OBSConfig{Host: "127.0.0.1", Port: 4455},
			ModeProfiles: config.DefaultProfiles(),
			CurrentMode:  config.ModePNG,
			SceneMappings: []config.SceneMapping{
				{Scene: "Main", Enabled: true, VTuberSource: "VTuber"},
			},
		},
	}

	result := app.ApplyMode(config.Mode3D)
	if result.OK {
		t.Fatalf("expected OBS failure result")
	}
	if app.cfg.CurrentMode != config.Mode3D {
		t.Fatalf("current mode = %q", app.cfg.CurrentMode)
	}
}

func TestStatusLockedRedactsSecrets(t *testing.T) {
	app := &App{
		obs:    &fakeOBSService{},
		logger: log.Default(),
		cfg: config.Config{
			OBS: config.OBSConfig{Host: "127.0.0.1", Port: 4455, Password: "obs-secret"},
			Twitch: config.TwitchConfig{
				ClientID:     "client",
				ChannelID:    "channel",
				ChannelName:  "Streamer",
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				TokenExpiry:  "2026-01-01T00:00:00Z",
			},
			ModeProfiles: config.DefaultProfiles(),
			CurrentMode:  config.ModePNG,
		},
	}

	status := app.statusLocked()
	if !status.Config.OBS.PasswordConfigured {
		t.Fatalf("expected OBS passwordConfigured to be true")
	}
	if status.Config.Twitch.ChannelName != "Streamer" {
		t.Fatalf("unexpected channel name: %#v", status.Config.Twitch)
	}
}

func TestTrustedBrowserURLRejectsUnexpectedHosts(t *testing.T) {
	if _, err := trustedBrowserURL("https://www.twitch.tv/activate"); err != nil {
		t.Fatalf("trusted twitch host rejected: %v", err)
	}
	if _, err := trustedBrowserURL("http://www.twitch.tv/activate"); err == nil {
		t.Fatalf("expected non-https URL rejection")
	}
	if _, err := trustedBrowserURL("https://example.com/activate"); err == nil {
		t.Fatalf("expected unexpected host rejection")
	}
}

func TestInitSecretsMigratesLegacyConfigSecretsAndKeepsThemInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store := config.NewStore(path)
	secretStore := &fakeSecretStore{}
	app := &App{
		store:       store,
		secretStore: secretStore,
		logger:      log.Default(),
		cfg: config.Config{
			OBS: config.OBSConfig{Host: "127.0.0.1", Port: 4455, Password: "obs-secret"},
			Twitch: config.TwitchConfig{
				ClientID:     "client",
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				TokenExpiry:  "2026-01-01T00:00:00Z",
			},
		},
	}

	app.initSecrets()

	if secretStore.obsPassword != "obs-secret" {
		t.Fatalf("obs password not migrated: %#v", secretStore)
	}
	if secretStore.twitchTokens.AccessToken != "access-token" || secretStore.twitchTokens.RefreshToken != "refresh-token" {
		t.Fatalf("twitch tokens not migrated: %#v", secretStore.twitchTokens)
	}
	if app.cfg.OBS.Password != "obs-secret" {
		t.Fatalf("obs password not reloaded into memory: %q", app.cfg.OBS.Password)
	}
	if app.cfg.Twitch.AccessToken != "access-token" || app.cfg.Twitch.RefreshToken != "refresh-token" {
		t.Fatalf("twitch tokens not reloaded into memory: %#v", app.cfg.Twitch)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "obs-secret") || strings.Contains(text, "access-token") || strings.Contains(text, "refresh-token") {
		t.Fatalf("legacy secrets remained in persisted config: %s", text)
	}
}

func TestSaveConfigReturnsErrorWhenSecureOBSSaveFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	app := &App{
		store:       config.NewStore(path),
		secretStore: &fakeSecretStore{saveOBSPasswordErr: fakeError("keyring offline")},
		logger:      log.Default(),
		obs:         &fakeOBSService{},
		cfg: config.Config{
			OBS:          config.OBSConfig{Host: "127.0.0.1", Port: 4455, Password: "old-secret"},
			ModeProfiles: config.DefaultProfiles(),
			CurrentMode:  config.ModePNG,
		},
	}

	result := app.SaveConfig(appdto.SettingsInput{
		Config:            app.settingsLocked(),
		OBSPassword:       "new-secret",
		UpdateOBSPassword: true,
	})

	if result.OK {
		t.Fatalf("expected failure result")
	}
	if got := result.Errors[0]; got != "keyring offline" {
		t.Fatalf("unexpected error: %#v", result.Errors)
	}
	if app.cfg.OBS.Password != "old-secret" {
		t.Fatalf("obs password changed despite failure: %q", app.cfg.OBS.Password)
	}
}

func TestRefreshRewardsReturnsErrorWhenSecureTokenSaveFails(t *testing.T) {
	app := &App{
		store:       config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		secretStore: &fakeSecretStore{saveTwitchTokensErr: fakeError("keyring write failed")},
		logger:      log.Default(),
		obs:         &fakeOBSService{},
		twitch: &fakeTwitchService{
			allRewards:        []twitch.Reward{{ID: "reward", Title: "Dance"}},
			manageableRewards: []twitch.Reward{{ID: "reward", Title: "Dance", Manageable: true}},
		},
		cfg: config.Config{
			Twitch: config.TwitchConfig{ClientID: "client", AccessToken: "token"},
		},
	}

	_, err := app.refreshRewards(context.Background())
	if err == nil || !strings.Contains(err.Error(), "keyring write failed") {
		t.Fatalf("expected secure token save error, got %v", err)
	}
}

type fakeError string

func (e fakeError) Error() string { return string(e) }

type fakeOBSService struct {
	connected        bool
	sources          map[string][]obs.Source
	visibilityCalls  []visibilityCall
	visibilityErrors map[string]error
}

func (f *fakeOBSService) Connected() bool { return f.connected }
func (f *fakeOBSService) Close()          { f.connected = false }
func (f *fakeOBSService) Connect(config.OBSConfig) error {
	f.connected = true
	return nil
}
func (f *fakeOBSService) GetScenes() ([]obs.Scene, error) { return nil, nil }
func (f *fakeOBSService) GetSources(scene string) ([]obs.Source, error) {
	return f.sources[scene], nil
}
func (f *fakeOBSService) FindSceneItemID(scene, source string) (int, error) {
	for _, item := range f.sources[scene] {
		if item.Name == source {
			return item.SceneItemID, nil
		}
	}
	return 0, fmt.Errorf("missing")
}
func (f *fakeOBSService) SetSourceVisibility(scene, source string, id int, enabled bool) error {
	f.visibilityCalls = append(f.visibilityCalls, visibilityCall{scene: scene, source: source, id: id, enabled: enabled})
	if f.visibilityErrors != nil {
		if err := f.visibilityErrors[scene+"/"+source]; err != nil {
			return err
		}
	}
	return nil
}

type visibilityCall struct {
	scene   string
	source  string
	id      int
	enabled bool
}

type fakeTwitchService struct {
	rewardCalls       []rewardCall
	rewardErrors      map[string]error
	allRewards        []twitch.Reward
	manageableRewards []twitch.Reward
	createdReward     twitch.Reward
}

func (f *fakeTwitchService) EnsureToken(context.Context, config.TwitchConfig) (config.TwitchConfig, error) {
	return config.TwitchConfig{ClientID: "client", AccessToken: "token"}, nil
}
func (f *fakeTwitchService) SetRewardEnabled(_ context.Context, _ config.TwitchConfig, rewardID string, enabled bool) error {
	f.rewardCalls = append(f.rewardCalls, rewardCall{rewardID: rewardID, enabled: enabled})
	if f.rewardErrors != nil {
		if err := f.rewardErrors[rewardID]; err != nil {
			return err
		}
	}
	return nil
}
func (f *fakeTwitchService) StartDeviceFlow(context.Context, config.TwitchConfig) (twitch.DeviceAuthorization, error) {
	return twitch.DeviceAuthorization{}, nil
}
func (f *fakeTwitchService) WaitForDeviceToken(context.Context, config.TwitchConfig, twitch.DeviceAuthorization) (config.TwitchConfig, error) {
	return config.TwitchConfig{}, nil
}
func (f *fakeTwitchService) FetchRewards(context.Context, config.TwitchConfig) ([]twitch.Reward, error) {
	return f.allRewards, nil
}
func (f *fakeTwitchService) FetchManageableRewards(context.Context, config.TwitchConfig) ([]twitch.Reward, error) {
	return f.manageableRewards, nil
}
func (f *fakeTwitchService) CreateReward(context.Context, config.TwitchConfig, string, int, string) (twitch.Reward, error) {
	return f.createdReward, nil
}

type rewardCall struct {
	rewardID string
	enabled  bool
}

type fakeSecretStore struct {
	obsPassword         string
	twitchTokens        secrets.TwitchTokens
	loadOBSPasswordErr  error
	saveOBSPasswordErr  error
	loadTwitchTokensErr error
	saveTwitchTokensErr error
}

func (f *fakeSecretStore) LoadOBSPassword() (string, error) {
	return f.obsPassword, f.loadOBSPasswordErr
}

func (f *fakeSecretStore) SaveOBSPassword(password string) error {
	if f.saveOBSPasswordErr != nil {
		return f.saveOBSPasswordErr
	}
	f.obsPassword = password
	return nil
}

func (f *fakeSecretStore) LoadTwitchTokens() (secrets.TwitchTokens, error) {
	return f.twitchTokens, f.loadTwitchTokensErr
}

func (f *fakeSecretStore) SaveTwitchTokens(tokens secrets.TwitchTokens) error {
	if f.saveTwitchTokensErr != nil {
		return f.saveTwitchTokensErr
	}
	f.twitchTokens = tokens
	return nil
}
