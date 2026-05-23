package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"TuberSwitch/internal/config"
	"TuberSwitch/internal/obs"
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
		twitch: fakeTwitch,
		logger: log.Default(),
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
		obs:    &fakeOBSService{},
		twitch: fakeTwitch,
		store:  config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		logger: log.Default(),
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
		obs:    &fakeOBSService{},
		twitch: fakeTwitch,
		store:  config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		logger: log.Default(),
		cfg:    config.Config{Twitch: config.TwitchConfig{ClientID: "client", AccessToken: "token"}},
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
		obs:    fakeOBS,
		twitch: fakeTwitch,
		store:  config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		logger: log.Default(),
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
		obs:    fakeOBS,
		twitch: &fakeTwitchService{},
		store:  config.NewStore(filepath.Join(t.TempDir(), "config.json")),
		logger: log.Default(),
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
