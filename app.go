package main

import (
	"context"
	stderrors "errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"

	appdto "TuberSwitch/internal/app"
	"TuberSwitch/internal/config"
	"TuberSwitch/internal/logging"
	"TuberSwitch/internal/obs"
	"TuberSwitch/internal/secrets"
	"TuberSwitch/internal/twitch"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	store       *config.Store
	secretStore secretStore
	logger      *log.Logger
	closeLog    func()
	obs         obsService
	twitch      fullTwitchService

	mu         sync.Mutex
	cfg        config.Config
	lastAction string
}

type secretStore interface {
	LoadOBSPassword() (string, error)
	SaveOBSPassword(string) error
	LoadTwitchTokens() (secrets.TwitchTokens, error)
	SaveTwitchTokens(secrets.TwitchTokens) error
}

type obsService interface {
	Connected() bool
	Close()
	Connect(config.OBSConfig) error
	GetScenes() ([]obs.Scene, error)
	GetSources(string) ([]obs.Source, error)
	FindSceneItemID(string, string) (int, error)
	SetSourceVisibility(string, string, int, bool) error
}

type twitchService interface {
	EnsureToken(context.Context, config.TwitchConfig) (config.TwitchConfig, error)
	SetRewardEnabled(context.Context, config.TwitchConfig, string, bool) error
}

type fullTwitchService interface {
	twitchService
	StartDeviceFlow(context.Context, config.TwitchConfig) (twitch.DeviceAuthorization, error)
	WaitForDeviceToken(context.Context, config.TwitchConfig, twitch.DeviceAuthorization) (config.TwitchConfig, error)
	FetchRewards(context.Context, config.TwitchConfig) ([]twitch.Reward, error)
	FetchManageableRewards(context.Context, config.TwitchConfig) ([]twitch.Reward, error)
	CreateReward(context.Context, config.TwitchConfig, string, int, string) (twitch.Reward, error)
}

func NewApp() *App {
	cfgPath, _ := config.ConfigPath()
	logPath, _ := config.LogPath()
	logger, closeLog, err := logging.New(logPath)
	if err != nil {
		logger = log.Default()
		closeLog = func() {}
		logger.Printf("logger setup failed: %v", err)
	}
	store := config.NewStore(cfgPath)
	cfg, err := store.Load()
	if err != nil {
		logger.Printf("config load failed: %v", err)
		cfg = config.Default()
	}
	app := &App{
		store:       store,
		secretStore: secrets.NewStore(),
		logger:      logger,
		closeLog:    closeLog,
		obs:         obs.New(logger),
		twitch:      twitch.New(logger),
		cfg:         cfg,
		lastAction:  "Ready",
	}
	return app.initSecrets()
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.logger.Printf("startup")
	_ = a.connectOBSLocked()
	a.refreshTwitchTokenLocked()
	if a.cfg.RefreshRewardsOnStartup && a.cfg.Twitch.AccessToken != "" {
		if _, err := a.refreshRewards(ctx); err != nil {
			a.logger.Printf("startup reward refresh failed: %v", err)
			a.lastAction = "Startup reward refresh failed: " + err.Error()
		}
	}
	mode := a.cfg.CurrentMode
	switch a.cfg.StartupMode {
	case config.StartupAlways3D:
		mode = config.Mode3D
	case config.StartupAlwaysPNG:
		mode = config.ModePNG
	}
	if err := a.applyOBSMode(mode); err != nil {
		a.logger.Printf("startup OBS mode apply failed: %v", err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.logger.Printf("shutdown")
	a.obs.Close()
	a.closeLog()
}

func (a *App) GetStatus() appdto.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) SaveConfig(input appdto.SettingsInput) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.updatedConfigLocked(input.Config)
	oldPassword := a.cfg.OBS.Password
	if input.UpdateOBSPassword {
		if err := a.secretStore.SaveOBSPassword(input.OBSPassword); err != nil {
			return a.resultLocked(false, "OBS password save failed", nil, []string{err.Error()})
		}
		next.OBS.Password = input.OBSPassword
	} else {
		next.OBS.Password = oldPassword
	}
	if err := a.store.Save(next); err != nil {
		if input.UpdateOBSPassword {
			_ = a.secretStore.SaveOBSPassword(oldPassword)
		}
		return a.resultLocked(false, "Config save failed", nil, []string{err.Error()})
	}
	a.cfg = next
	a.lastAction = "Settings saved"
	return a.resultLocked(true, "Settings saved", nil, nil)
}

func (a *App) TestOBSConnection() appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.obs.Close()
	if err := a.connectOBSLocked(); err != nil {
		a.lastAction = "OBS connection failed"
		return a.resultLocked(false, "OBS connection failed", nil, []string{err.Error()})
	}
	a.lastAction = "OBS connected"
	return a.resultLocked(true, "OBS connected", nil, nil)
}

func (a *App) SyncOBS() appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.connectOBSLocked(); err != nil {
		return a.resultLocked(false, "OBS sync failed", nil, []string{err.Error()})
	}
	scenes, err := a.obs.GetScenes()
	if err != nil {
		return a.resultLocked(false, "OBS sync failed", nil, []string{err.Error()})
	}
	a.mergeSceneMappingsLocked(scenes)
	a.resolveSceneItemIDsBestEffortLocked()
	_ = a.store.Save(a.cfg)
	a.lastAction = "OBS scenes and sources synced"
	return a.resultLocked(true, "OBS scenes and sources synced", nil, nil)
}

func (a *App) GetOBSInventory(sceneName string) (appdto.OBSInventory, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.connectOBSLocked(); err != nil {
		return appdto.OBSInventory{}, err
	}
	scenes, err := a.obs.GetScenes()
	if err != nil {
		return appdto.OBSInventory{}, err
	}
	if sceneName == "" {
		sceneName = firstConfiguredScene(a.cfg)
	}
	sourcesByScene := map[string][]obs.Source{}
	for _, scene := range scenes {
		sceneSources, err := a.obs.GetSources(scene.Name)
		if err != nil {
			a.logger.Printf("OBS inventory skipped scene %q: %v", scene.Name, err)
			continue
		}
		sourcesByScene[scene.Name] = sceneSources
	}
	return toInventory(scenes, sourcesByScene, sceneName), nil
}

func (a *App) Test3DMode() appdto.ActionResult {
	return a.testOBSMode(config.Mode3D)
}

func (a *App) TestPNGMode() appdto.ActionResult {
	return a.testOBSMode(config.ModePNG)
}

func (a *App) ApplyMode(mode config.Mode) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	warnings := []string{}
	errors := []string{}
	if err := a.applyOBSMode(mode); err != nil {
		errors = append(errors, err.Error())
	}
	if errList := a.applyTwitchModeLocked(mode); len(errList) > 0 {
		errors = append(errors, errList...)
	}
	a.cfg.CurrentMode = mode
	if err := a.store.Save(a.cfg); err != nil {
		errors = append(errors, err.Error())
	}
	if len(errors) > 0 {
		a.lastAction = fmt.Sprintf("Switched to %s mode with errors", mode)
		return a.resultLocked(false, a.lastAction, warnings, errors)
	}
	a.lastAction = fmt.Sprintf("Switched to %s Mode successfully", mode)
	return a.resultLocked(true, a.lastAction, warnings, nil)
}

func (a *App) StartTwitchLogin() appdto.ActionResult {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	if cfg.Twitch.ClientID == "" {
		return a.withError("Twitch login requires a client ID")
	}
	device, err := a.twitch.StartDeviceFlow(context.Background(), cfg.Twitch)
	if err != nil {
		return a.withError(err.Error())
	}
	a.mu.Lock()
	a.lastAction = "Twitch login opened. Enter code " + device.UserCode + " if prompted."
	a.mu.Unlock()
	a.logger.Printf("Twitch device login URL: %s user_code=%s", device.VerificationURI, device.UserCode)
	openURL, err := trustedBrowserURL(device.VerificationURI)
	if err != nil {
		return a.withError(err.Error())
	}
	runtime.BrowserOpenURL(a.ctx, openURL)

	updated, err := a.twitch.WaitForDeviceToken(context.Background(), cfg.Twitch, device)
	if err != nil {
		return a.withError(err.Error())
	}
	if err := a.secretStore.SaveTwitchTokens(twitchTokensFromConfig(updated)); err != nil {
		return a.withError("Twitch secure token save failed: " + err.Error())
	}
	a.mu.Lock()
	a.cfg.Twitch = updated
	a.lastAction = "Twitch connected as " + updated.ChannelName
	_ = a.store.Save(a.cfg)
	result := a.resultLocked(true, a.lastAction, nil, nil)
	a.mu.Unlock()
	return result
}

func (a *App) RefreshTwitchRewards() appdto.ActionResult {
	rewards, err := a.refreshRewards(context.Background())
	if err != nil {
		return a.withError(err.Error())
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAction = "Twitch rewards refreshed: " + strconv.Itoa(len(rewards))
	return a.resultLocked(true, a.lastAction, nil, nil)
}

func (a *App) SetReward3DOnly(rewardID string, is3DOnly bool) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	found := false
	for i := range a.cfg.RewardMappings {
		if a.cfg.RewardMappings[i].RewardID == rewardID {
			if is3DOnly && !a.cfg.RewardMappings[i].Manageable {
				return a.resultLocked(false, "Reward cannot be managed", nil, []string{"Twitch only allows this app to toggle rewards created by this app."})
			}
			a.cfg.RewardMappings[i].Is3DOnly = is3DOnly
			found = true
			break
		}
	}
	if !found {
		return a.resultLocked(false, "Reward mapping not found", nil, []string{"Reward mapping was not found"})
	}
	if err := a.store.Save(a.cfg); err != nil {
		return a.resultLocked(false, "Reward mapping save failed", nil, []string{err.Error()})
	}
	a.lastAction = "Reward mapping updated"
	return a.resultLocked(true, a.lastAction, nil, nil)
}

func (a *App) CreateTwitchReward(title string, cost int, prompt string) appdto.ActionResult {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	updated, err := a.twitch.EnsureToken(context.Background(), cfg.Twitch)
	if err != nil {
		return a.withError(err.Error())
	}
	cfg.Twitch = updated
	if err := a.secretStore.SaveTwitchTokens(twitchTokensFromConfig(updated)); err != nil {
		return a.withError("Twitch secure token save failed: " + err.Error())
	}
	reward, err := a.twitch.CreateReward(context.Background(), cfg.Twitch, title, cost, prompt)
	if err != nil {
		return a.withError(err.Error())
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Twitch = cfg.Twitch
	a.upsertRewardMappingLocked(reward, true)
	if err := a.store.Save(a.cfg); err != nil {
		return a.resultLocked(false, "Reward created but config save failed", nil, []string{err.Error()})
	}
	a.lastAction = "Created Twitch reward: " + reward.Title
	return a.resultLocked(true, a.lastAction, nil, nil)
}

func (a *App) GetTwitchRewards() []appdto.TwitchReward {
	a.mu.Lock()
	defer a.mu.Unlock()
	rewards := make([]appdto.TwitchReward, 0, len(a.cfg.RewardMappings))
	for _, mapping := range a.cfg.RewardMappings {
		rewards = append(rewards, appdto.TwitchReward{
			ID:         mapping.RewardID,
			Title:      mapping.RewardName,
			Is3DOnly:   mapping.Is3DOnly,
			Manageable: mapping.Manageable,
		})
	}
	return rewards
}

func (a *App) testOBSMode(mode config.Mode) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.applyOBSMode(mode); err != nil {
		a.lastAction = fmt.Sprintf("OBS test %s failed", mode)
		return a.resultLocked(false, a.lastAction, nil, []string{err.Error()})
	}
	a.lastAction = fmt.Sprintf("OBS test %s mode applied", mode)
	return a.resultLocked(true, a.lastAction, nil, nil)
}

func (a *App) connectOBSLocked() error {
	if a.obs.Connected() {
		return nil
	}
	return a.obs.Connect(a.cfg.OBS)
}

func (a *App) refreshTwitchTokenLocked() {
	if a.cfg.Twitch.AccessToken == "" {
		return
	}
	updated, err := a.twitch.EnsureToken(context.Background(), a.cfg.Twitch)
	if err != nil {
		a.logger.Printf("Twitch token refresh failed: %v", err)
		return
	}
	a.cfg.Twitch = updated
	if err := a.secretStore.SaveTwitchTokens(twitchTokensFromConfig(updated)); err != nil {
		a.logger.Printf("Twitch secure token save failed: %v", err)
	}
	_ = a.store.Save(a.cfg)
}

func (a *App) applyOBSMode(mode config.Mode) error {
	if err := a.connectOBSLocked(); err != nil {
		return err
	}
	profile, ok := a.cfg.Profile(mode)
	if !ok {
		return fmt.Errorf("mode profile %q was not found", mode)
	}
	a.resolveSceneItemIDsBestEffortLocked()
	errMessages := []string{}
	applied := 0
	for _, mapping := range a.cfg.SceneMappings {
		if !mapping.Enabled || mapping.Scene == "" {
			continue
		}
		if mapping.VTuberSource != "" {
			applied++
			if err := a.obs.SetSourceVisibility(mapping.Scene, mapping.VTuberSource, mapping.VTuberItemID, profile.VTuberVisible); err != nil {
				errMessages = append(errMessages, fmt.Sprintf("%s / %s: %v", mapping.Scene, mapping.VTuberSource, err))
			}
		}
		if mapping.PNGTuberSource != "" {
			applied++
			if err := a.obs.SetSourceVisibility(mapping.Scene, mapping.PNGTuberSource, mapping.PNGTuberItemID, profile.PNGTuberVisible); err != nil {
				errMessages = append(errMessages, fmt.Sprintf("%s / %s: %v", mapping.Scene, mapping.PNGTuberSource, err))
			}
		}
		if mapping.VTuberSource == "" || mapping.PNGTuberSource == "" {
			a.logger.Printf("OBS scene %q partially configured: missing sources are ignored", mapping.Scene)
		}
		a.logger.Printf("OBS scene %q switched to %s", mapping.Scene, mode)
	}
	if applied == 0 && len(errMessages) == 0 {
		return fmt.Errorf("no OBS scene mappings are configured")
	}
	if len(errMessages) > 0 {
		return stderrors.New(strings.Join(errMessages, "; "))
	}
	return nil
}

func (a *App) applyTwitchModeLocked(mode config.Mode) []string {
	errors := []string{}
	if a.cfg.Twitch.AccessToken == "" {
		return errors
	}
	profile, ok := a.cfg.Profile(mode)
	if !ok {
		return []string{fmt.Sprintf("mode profile %q was not found", mode)}
	}
	updated, err := a.twitch.EnsureToken(context.Background(), a.cfg.Twitch)
	if err != nil {
		return []string{err.Error()}
	}
	a.cfg.Twitch = updated
	if err := a.secretStore.SaveTwitchTokens(twitchTokensFromConfig(updated)); err != nil {
		return []string{"Twitch secure token save failed: " + err.Error()}
	}
	for _, mapping := range a.cfg.RewardMappings {
		if !mapping.Is3DOnly || !mapping.Manageable {
			continue
		}
		if err := a.twitch.SetRewardEnabled(context.Background(), a.cfg.Twitch, mapping.RewardID, profile.Enable3DRewards); err != nil {
			msg := fmt.Sprintf("%s: %v", mapping.RewardName, err)
			if isUnmanageableRewardError(err) {
				a.logger.Printf("reward skipped: %s", msg)
				continue
			}
			a.logger.Printf("reward update failed: %s", msg)
			errors = append(errors, msg)
			continue
		}
		a.logger.Printf("reward %q enabled=%v", mapping.RewardName, profile.Enable3DRewards)
	}
	return errors
}

func (a *App) resolveSceneItemIDsLocked() error {
	for i := range a.cfg.SceneMappings {
		mapping := &a.cfg.SceneMappings[i]
		if !mapping.Enabled || mapping.Scene == "" {
			continue
		}
		if mapping.VTuberSource != "" {
			id, err := a.obs.FindSceneItemID(mapping.Scene, mapping.VTuberSource)
			if err != nil {
				return err
			}
			mapping.VTuberItemID = id
		}
		if mapping.PNGTuberSource != "" {
			id, err := a.obs.FindSceneItemID(mapping.Scene, mapping.PNGTuberSource)
			if err != nil {
				return err
			}
			mapping.PNGTuberItemID = id
		}
	}
	return nil
}

func (a *App) resolveSceneItemIDsBestEffortLocked() {
	for i := range a.cfg.SceneMappings {
		mapping := &a.cfg.SceneMappings[i]
		if mapping.Scene == "" {
			continue
		}
		sources, err := a.obs.GetSources(mapping.Scene)
		if err != nil {
			a.logger.Printf("scene item ID sync skipped for %q: %v", mapping.Scene, err)
			continue
		}
		mapping.VTuberItemID = findSourceID(sources, mapping.VTuberSource)
		mapping.PNGTuberItemID = findSourceID(sources, mapping.PNGTuberSource)
	}
}

func (a *App) refreshRewards(ctx context.Context) ([]twitch.Reward, error) {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	updated, err := a.twitch.EnsureToken(ctx, cfg.Twitch)
	if err != nil {
		return nil, err
	}
	cfg.Twitch = updated
	if err := a.secretStore.SaveTwitchTokens(twitchTokensFromConfig(updated)); err != nil {
		return nil, err
	}
	rewards, err := a.twitch.FetchRewards(ctx, cfg.Twitch)
	if err != nil {
		return nil, err
	}
	manageableRewards, err := a.twitch.FetchManageableRewards(ctx, cfg.Twitch)
	if err != nil {
		return nil, err
	}
	manageableIDs := map[string]bool{}
	for _, reward := range manageableRewards {
		manageableIDs[reward.ID] = true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Twitch = cfg.Twitch
	existing := map[string]config.RewardMapping{}
	for _, mapping := range a.cfg.RewardMappings {
		existing[mapping.RewardID] = mapping
	}
	next := make([]config.RewardMapping, 0, len(rewards))
	for _, reward := range rewards {
		mapping := existing[reward.ID]
		mapping.RewardID = reward.ID
		mapping.RewardName = reward.Title
		mapping.Manageable = manageableIDs[reward.ID]
		if !mapping.Manageable {
			mapping.Is3DOnly = false
		}
		next = append(next, mapping)
	}
	a.cfg.RewardMappings = next
	if err := a.store.Save(a.cfg); err != nil {
		return nil, err
	}
	return rewards, nil
}

func (a *App) upsertRewardMappingLocked(reward twitch.Reward, manageable bool) {
	for i := range a.cfg.RewardMappings {
		if a.cfg.RewardMappings[i].RewardID == reward.ID {
			a.cfg.RewardMappings[i].RewardName = reward.Title
			a.cfg.RewardMappings[i].Manageable = manageable
			return
		}
	}
	a.cfg.RewardMappings = append(a.cfg.RewardMappings, config.RewardMapping{
		RewardID:   reward.ID,
		RewardName: reward.Title,
		Manageable: manageable,
	})
}

func (a *App) statusLocked() appdto.Status {
	label := string(a.cfg.CurrentMode)
	if profile, ok := a.cfg.Profile(a.cfg.CurrentMode); ok {
		label = profile.DisplayName
	}
	return appdto.Status{
		Config:           a.settingsLocked(),
		CurrentMode:      a.cfg.CurrentMode,
		CurrentModeLabel: label,
		OBSConnected:     a.obs.Connected(),
		TwitchConnected:  a.cfg.Twitch.AccessToken != "",
		LastAction:       a.lastAction,
	}
}

func (a *App) resultLocked(ok bool, message string, warnings []string, errors []string) appdto.ActionResult {
	return appdto.ActionResult{OK: ok, Message: message, Warnings: warnings, Errors: errors, NewStatus: a.statusLocked()}
}

func (a *App) withError(message string) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAction = message
	return a.resultLocked(false, message, nil, []string{message})
}

func toInventory(scenes []obs.Scene, sourcesByScene map[string][]obs.Source, selectedScene string) appdto.OBSInventory {
	inventory := appdto.OBSInventory{SourcesByScene: map[string][]appdto.OBSSource{}}
	for _, scene := range scenes {
		inventory.Scenes = append(inventory.Scenes, appdto.OBSScene{Name: scene.Name})
	}
	for sceneName, sources := range sourcesByScene {
		for _, source := range sources {
			dto := appdto.OBSSource{Name: source.Name, SceneItemID: source.SceneItemID}
			inventory.SourcesByScene[sceneName] = append(inventory.SourcesByScene[sceneName], dto)
			if sceneName == selectedScene {
				inventory.Sources = append(inventory.Sources, dto)
			}
		}
	}
	return inventory
}

func sceneExists(scenes []obs.Scene, name string) bool {
	if name == "" {
		return false
	}
	for _, scene := range scenes {
		if scene.Name == name {
			return true
		}
	}
	return false
}

func (a *App) mergeSceneMappingsLocked(scenes []obs.Scene) {
	existing := map[string]config.SceneMapping{}
	for _, mapping := range a.cfg.SceneMappings {
		if mapping.Scene != "" {
			existing[mapping.Scene] = mapping
		}
	}
	next := make([]config.SceneMapping, 0, len(scenes))
	for _, scene := range scenes {
		mapping := existing[scene.Name]
		mapping.Scene = scene.Name
		next = append(next, mapping)
	}
	a.cfg.SceneMappings = next
}

func firstConfiguredScene(cfg config.Config) string {
	for _, mapping := range cfg.SceneMappings {
		if mapping.Scene != "" {
			return mapping.Scene
		}
	}
	return cfg.Sources.Scene
}

func findSourceID(sources []obs.Source, name string) int {
	if name == "" {
		return 0
	}
	for _, source := range sources {
		if source.Name == name {
			return source.SceneItemID
		}
	}
	return 0
}

func isUnmanageableRewardError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "client ID used to create the custom reward") ||
		strings.Contains(msg, "broadcaster doesn't have partner or affiliate status")
}

func (a *App) initSecrets() *App {
	if err := a.migrateLegacySecrets(); err != nil {
		a.logger.Printf("secret migration failed: %v", err)
	}
	if err := a.loadSecrets(); err != nil {
		a.logger.Printf("secret load failed: %v", err)
	}
	return a
}

func (a *App) migrateLegacySecrets() error {
	changed := false
	if a.cfg.OBS.Password != "" {
		if err := a.secretStore.SaveOBSPassword(a.cfg.OBS.Password); err != nil {
			return err
		}
		a.cfg.OBS.Password = ""
		changed = true
	}
	if a.cfg.Twitch.AccessToken != "" || a.cfg.Twitch.RefreshToken != "" || a.cfg.Twitch.TokenExpiry != "" {
		if err := a.secretStore.SaveTwitchTokens(twitchTokensFromConfig(a.cfg.Twitch)); err != nil {
			return err
		}
		a.cfg.Twitch.AccessToken = ""
		a.cfg.Twitch.RefreshToken = ""
		a.cfg.Twitch.TokenExpiry = ""
		changed = true
	}
	if changed {
		return a.store.Save(a.cfg)
	}
	return nil
}

func (a *App) loadSecrets() error {
	password, err := a.secretStore.LoadOBSPassword()
	if err != nil {
		return err
	}
	a.cfg.OBS.Password = password
	tokens, err := a.secretStore.LoadTwitchTokens()
	if err != nil {
		return err
	}
	a.cfg.Twitch.AccessToken = tokens.AccessToken
	a.cfg.Twitch.RefreshToken = tokens.RefreshToken
	a.cfg.Twitch.TokenExpiry = tokens.TokenExpiry
	return nil
}

func (a *App) settingsLocked() appdto.Settings {
	return appdto.Settings{
		OBS: appdto.OBSSettings{
			Host:               a.cfg.OBS.Host,
			Port:               a.cfg.OBS.Port,
			AllowRemote:        a.cfg.OBS.AllowRemote,
			PasswordConfigured: a.cfg.OBS.Password != "",
		},
		Sources:                 a.cfg.Sources,
		SceneMappings:           append([]config.SceneMapping(nil), a.cfg.SceneMappings...),
		Twitch:                  appdto.TwitchSettings{ClientID: a.cfg.Twitch.ClientID, ChannelID: a.cfg.Twitch.ChannelID, ChannelName: a.cfg.Twitch.ChannelName},
		ModeProfiles:            append([]config.ModeProfile(nil), a.cfg.ModeProfiles...),
		StartupMode:             a.cfg.StartupMode,
		CurrentMode:             a.cfg.CurrentMode,
		RefreshRewardsOnStartup: a.cfg.RefreshRewardsOnStartup,
	}
}

func (a *App) updatedConfigLocked(settings appdto.Settings) config.Config {
	next := a.cfg
	next.OBS.Host = settings.OBS.Host
	next.OBS.Port = settings.OBS.Port
	next.OBS.AllowRemote = settings.OBS.AllowRemote
	next.Sources = settings.Sources
	next.SceneMappings = append([]config.SceneMapping(nil), settings.SceneMappings...)
	next.Twitch.ClientID = settings.Twitch.ClientID
	next.Twitch.ChannelID = settings.Twitch.ChannelID
	next.Twitch.ChannelName = settings.Twitch.ChannelName
	next.ModeProfiles = append([]config.ModeProfile(nil), settings.ModeProfiles...)
	next.StartupMode = settings.StartupMode
	next.CurrentMode = settings.CurrentMode
	next.RefreshRewardsOnStartup = settings.RefreshRewardsOnStartup
	next.Normalize()
	return next
}

func twitchTokensFromConfig(cfg config.TwitchConfig) secrets.TwitchTokens {
	return secrets.TwitchTokens{
		AccessToken:  cfg.AccessToken,
		RefreshToken: cfg.RefreshToken,
		TokenExpiry:  cfg.TokenExpiry,
	}
}

func trustedBrowserURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid browser URL from Twitch")
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("refusing to open non-HTTPS Twitch login URL")
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "twitch.tv", "www.twitch.tv", "id.twitch.tv":
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("refusing to open unexpected Twitch login host %q", host)
	}
}
