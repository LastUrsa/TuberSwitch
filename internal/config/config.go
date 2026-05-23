package config

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

type Config struct {
	OBS                     OBSConfig       `json:"obs"`
	Sources                 SourcesConfig   `json:"sources"`
	SceneMappings           []SceneMapping  `json:"sceneMappings"`
	Twitch                  TwitchConfig    `json:"twitch"`
	RewardMappings          []RewardMapping `json:"rewardMappings"`
	ModeProfiles            []ModeProfile   `json:"modeProfiles"`
	StartupMode             StartupMode     `json:"startupMode"`
	CurrentMode             Mode            `json:"currentMode"`
	RefreshRewardsOnStartup bool            `json:"refreshRewardsOnStartup"`
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
}

func (c Config) Profile(mode Mode) (ModeProfile, bool) {
	for _, profile := range c.ModeProfiles {
		if profile.ID == mode {
			return profile, true
		}
	}
	return ModeProfile{}, false
}
