package main

import (
	"context"
	"log"
	"sync"
	"time"

	"TuberSwitch/internal/appdetect"
	"TuberSwitch/internal/config"
	"TuberSwitch/internal/logging"
	"TuberSwitch/internal/obs"
	"TuberSwitch/internal/secrets"
	"TuberSwitch/internal/sip"
	"TuberSwitch/internal/twitch"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	currentAppVersion    = "0.5.0"
	githubLatestRelease  = "https://api.github.com/repos/LastUrsa/TuberSwitch/releases/latest"
	githubReleasesPage   = "https://github.com/LastUrsa/TuberSwitch/releases"
	updateRequestTimeout = 8 * time.Second
)

var updateReleaseEndpoint = githubLatestRelease

type App struct {
	ctx             context.Context
	store           *config.Store
	secretStore     secretStore
	logger          *log.Logger
	closeLog        func()
	obs             obsService
	twitch          fullTwitchService
	detector        appDetectionService
	processes       appdetect.ProcessProvider
	openFileDialog  func(context.Context, runtime.OpenDialogOptions) (string, error)
	runtimeMode     string
	windowActivator func(context.Context)
	sipServer       *sip.Server

	mu         sync.Mutex
	cfg        config.Config
	lastAction string
}

type applyModeOptions struct {
	applyTwitchChanges bool
	source             string
	recordManualSwitch bool
}

type appDetectionService interface {
	Start(config.AppDetectionConfig)
	Stop()
	RecordManualOverride(time.Duration)
	Snapshot() appdetect.Snapshot
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
	return NewAppForMode(sip.StandaloneMode)
}

func NewAppForMode(runtimeMode string) *App {
	if runtimeMode == "" {
		runtimeMode = sip.StandaloneMode
	}
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
		store:           store,
		secretStore:     secrets.NewStore(),
		logger:          logger,
		closeLog:        closeLog,
		obs:             obs.New(logger),
		twitch:          twitch.New(logger),
		cfg:             cfg,
		lastAction:      "Ready",
		processes:       appdetect.WindowsProcessProvider{},
		openFileDialog:  runtime.OpenFileDialog,
		runtimeMode:     runtimeMode,
		windowActivator: defaultWindowActivator,
	}
	app.detector = appdetect.New(logger, app.processes, app.applyModeFromDetection, app.currentMode)
	app.sipServer = sip.NewServer(sip.NewService(sip.AppInfo{
		AppID:    "tuberswitch",
		Name:     "TuberSwitch",
		Version:  currentAppVersion,
		Mode:     runtimeMode,
		Protocol: sip.ProtocolVersion,
	}, app))
	return app.initSecrets()
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.logger != nil {
		a.logger.Printf("%s", startupModeMessage(a.runtimeMode))
	}
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
	if a.detector != nil {
		a.detector.Start(a.cfg.AppDetection)
	}
	if a.sipServer != nil {
		if err := a.sipServer.Start(ctx); err != nil && a.logger != nil {
			a.logger.Printf("SIP server unavailable: %v", err)
		}
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.logger != nil {
		a.logger.Printf("shutdown")
	}
	if a.sipServer != nil {
		_ = a.sipServer.Stop(ctx)
	}
	if a.detector != nil {
		a.detector.Stop()
	}
	if a.obs != nil {
		a.obs.Close()
	}
	if a.closeLog != nil {
		a.closeLog()
	}
}

func (a *App) handleSecondInstanceLaunch(args []string) {
	if !shouldActivateExistingInstance(args) {
		return
	}
	activator := a.windowActivator
	if activator == nil {
		activator = defaultWindowActivator
	}
	activator(a.ctx)
}
