package config

import "strings"

type Mode string

const (
	Mode3D  Mode = "3D"
	ModePNG Mode = "PNG"
)

type StartupMode string

const (
	StartupRestoreLast StartupMode = "restore-last"
	StartupAlways3D    StartupMode = "always-3d"
	StartupAlwaysPNG   StartupMode = "always-png"
)

type AppDetectionConflictBehavior string

const (
	AppDetectionConflictDoNothing AppDetectionConflictBehavior = "do-nothing"
	AppDetectionConflictPrefer3D  AppDetectionConflictBehavior = "prefer-3d"
	AppDetectionConflictPreferPNG AppDetectionConflictBehavior = "prefer-png"
)

type Config struct {
	OBS                     OBSConfig          `json:"obs"`
	Sources                 SourcesConfig      `json:"sources"`
	SceneMappings           []SceneMapping     `json:"sceneMappings"`
	Twitch                  TwitchConfig       `json:"twitch"`
	RewardMappings          []RewardMapping    `json:"rewardMappings"`
	ModeProfiles            []ModeProfile      `json:"modeProfiles"`
	StartupMode             StartupMode        `json:"startupMode"`
	CurrentMode             Mode               `json:"currentMode"`
	RefreshRewardsOnStartup bool               `json:"refreshRewardsOnStartup"`
	AppDetection            AppDetectionConfig `json:"appDetection"`
}

type OBSConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Password    string `json:"password"`
	AllowRemote bool   `json:"allowRemote"`
}

type SourcesConfig struct {
	Scene          string `json:"scene"`
	VTuberSource   string `json:"vtuberSource"`
	VTuberItemID   int    `json:"vtuberItemId"`
	PNGTuberSource string `json:"pngTuberSource"`
	PNGTuberItemID int    `json:"pngTuberItemId"`
}

type SceneMapping struct {
	Scene          string `json:"scene"`
	Enabled        bool   `json:"enabled"`
	VTuberSource   string `json:"vtuberSource"`
	VTuberItemID   int    `json:"vtuberItemId"`
	PNGTuberSource string `json:"pngTuberSource"`
	PNGTuberItemID int    `json:"pngTuberItemId"`
}

type TwitchConfig struct {
	ClientID     string `json:"clientId"`
	ChannelID    string `json:"channelId"`
	ChannelName  string `json:"channelName"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenExpiry  string `json:"tokenExpiry"`
}

type RewardMapping struct {
	RewardID   string `json:"rewardId"`
	RewardName string `json:"rewardName"`
	Is3DOnly   bool   `json:"is3DOnly"`
	Manageable bool   `json:"manageable"`
}

type ModeProfile struct {
	ID              Mode   `json:"id"`
	DisplayName     string `json:"displayName"`
	VTuberVisible   bool   `json:"vtuberVisible"`
	PNGTuberVisible bool   `json:"pngTuberVisible"`
	Enable3DRewards bool   `json:"enable3DRewards"`
}

type AppDetectionConfig struct {
	Enabled                       bool                         `json:"enabled"`
	ThreeDProcessName             string                       `json:"threeDProcessName"`
	PNGProcessName                string                       `json:"pngProcessName"`
	IntervalSeconds               int                          `json:"intervalSeconds"`
	ConflictBehavior              AppDetectionConflictBehavior `json:"conflictBehavior"`
	ApplyTwitchChanges            bool                         `json:"applyTwitchChanges"`
	ManualOverrideCooldownSeconds int                          `json:"manualOverrideCooldownSeconds"`
}

func Default() Config {
	return Config{
		OBS: OBSConfig{
			Host: "127.0.0.1",
			Port: 4455,
		},
		SceneMappings:           []SceneMapping{},
		RewardMappings:          []RewardMapping{},
		ModeProfiles:            DefaultProfiles(),
		StartupMode:             StartupRestoreLast,
		CurrentMode:             ModePNG,
		RefreshRewardsOnStartup: false,
		AppDetection:            DefaultAppDetection(),
	}
}

func DefaultProfiles() []ModeProfile {
	return []ModeProfile{
		{
			ID:              Mode3D,
			DisplayName:     "3D VTuber Mode",
			VTuberVisible:   true,
			PNGTuberVisible: false,
			Enable3DRewards: true,
		},
		{
			ID:              ModePNG,
			DisplayName:     "PNG VTuber Mode",
			VTuberVisible:   false,
			PNGTuberVisible: true,
			Enable3DRewards: false,
		},
	}
}

func (c *Config) Normalize() {
	if c.OBS.Host == "" || c.OBS.Host == "localhost" {
		c.OBS.Host = "127.0.0.1"
	}
	if c.OBS.Port == 0 {
		c.OBS.Port = 4455
	}
	if c.StartupMode == "" {
		c.StartupMode = StartupRestoreLast
	}
	if c.CurrentMode == "" {
		c.CurrentMode = ModePNG
	}
	if len(c.ModeProfiles) == 0 {
		c.ModeProfiles = DefaultProfiles()
	}
	if len(c.SceneMappings) == 0 && c.Sources.Scene != "" {
		c.SceneMappings = []SceneMapping{
			{
				Scene:          c.Sources.Scene,
				Enabled:        true,
				VTuberSource:   c.Sources.VTuberSource,
				VTuberItemID:   c.Sources.VTuberItemID,
				PNGTuberSource: c.Sources.PNGTuberSource,
				PNGTuberItemID: c.Sources.PNGTuberItemID,
			},
		}
	}
	if c.AppDetection == (AppDetectionConfig{}) {
		c.AppDetection = DefaultAppDetection()
	} else {
		c.AppDetection.Normalize()
	}
}

func (c Config) Profile(mode Mode) (ModeProfile, bool) {
	for _, profile := range c.ModeProfiles {
		if profile.ID == mode {
			return profile, true
		}
	}
	return ModeProfile{}, false
}

func DefaultAppDetection() AppDetectionConfig {
	return AppDetectionConfig{
		Enabled:                       false,
		ThreeDProcessName:             "",
		PNGProcessName:                "",
		IntervalSeconds:               5,
		ConflictBehavior:              AppDetectionConflictDoNothing,
		ApplyTwitchChanges:            true,
		ManualOverrideCooldownSeconds: 15,
	}
}

func (c *AppDetectionConfig) Normalize() {
	defaults := DefaultAppDetection()
	c.ThreeDProcessName = normalizeExecutableName(c.ThreeDProcessName)
	c.PNGProcessName = normalizeExecutableName(c.PNGProcessName)
	if c.IntervalSeconds == 0 {
		c.IntervalSeconds = defaults.IntervalSeconds
	}
	if c.IntervalSeconds < 2 {
		c.IntervalSeconds = 2
	}
	if c.ConflictBehavior == "" {
		c.ConflictBehavior = defaults.ConflictBehavior
	}
	switch c.ConflictBehavior {
	case AppDetectionConflictDoNothing, AppDetectionConflictPrefer3D, AppDetectionConflictPreferPNG:
	default:
		c.ConflictBehavior = defaults.ConflictBehavior
	}
	if c.ManualOverrideCooldownSeconds == 0 {
		c.ManualOverrideCooldownSeconds = defaults.ManualOverrideCooldownSeconds
	}
	if c.ManualOverrideCooldownSeconds < 0 {
		c.ManualOverrideCooldownSeconds = 0
	}
}

func normalizeExecutableName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lastSeparator := strings.LastIndexAny(value, `/\`)
	if lastSeparator >= 0 && lastSeparator < len(value)-1 {
		value = value[lastSeparator+1:]
	}
	return strings.TrimSpace(value)
}
