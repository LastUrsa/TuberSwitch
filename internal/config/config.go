package config

import (
	"strconv"
	"strings"
)

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
	Profiles                []Profile          `json:"profiles"`
	ActiveProfileID         string             `json:"activeProfileId"`
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

const DefaultProfileID = "default"

type Profile struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Mode           Mode            `json:"mode"`
	Sources        SourcesConfig   `json:"sources"`
	SceneMappings  []SceneMapping  `json:"sceneMappings"`
	RewardMappings []RewardMapping `json:"rewardMappings"`
	LastUsed       string          `json:"lastUsed"`
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
		Profiles:                []Profile{},
		ActiveProfileID:         DefaultProfileID,
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
			DisplayName:     "PNGTuber Mode",
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
	for i := range c.ModeProfiles {
		if c.ModeProfiles[i].ID == ModePNG {
			c.ModeProfiles[i].DisplayName = strings.ReplaceAll(c.ModeProfiles[i].DisplayName, "PNG "+"VTuber", "PNGTuber")
			c.ModeProfiles[i].DisplayName = strings.ReplaceAll(c.ModeProfiles[i].DisplayName, "PNG "+"Tuber", "PNGTuber")
			if c.ModeProfiles[i].DisplayName == "" {
				c.ModeProfiles[i].DisplayName = "PNGTuber Mode"
			}
		}
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
	c.normalizeProfiles()
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

func (c Config) ActiveProfile() (Profile, bool) {
	for _, profile := range c.Profiles {
		if profile.ID == c.ActiveProfileID {
			return profile, true
		}
	}
	return Profile{}, false
}

func (c *Config) ApplyStreamProfile(profile Profile) {
	c.ActiveProfileID = profile.ID
	c.CurrentMode = profile.Mode
	c.Sources = profile.Sources
	c.SceneMappings = cloneSceneMappings(profile.SceneMappings)
	c.RewardMappings = cloneRewardMappings(profile.RewardMappings)
}

func (c *Config) UpsertActiveProfileFromCurrent(lastUsed string) {
	c.Normalize()
	profile, index := c.activeProfileWithIndex()
	profile.Mode = c.CurrentMode
	profile.Sources = c.Sources
	profile.SceneMappings = cloneSceneMappings(c.SceneMappings)
	profile.RewardMappings = cloneRewardMappings(c.RewardMappings)
	if lastUsed != "" {
		profile.LastUsed = lastUsed
	}
	c.Profiles[index] = profile
}

func (c *Config) activeProfileWithIndex() (Profile, int) {
	for i, profile := range c.Profiles {
		if profile.ID == c.ActiveProfileID {
			return profile, i
		}
	}
	return c.Profiles[0], 0
}

func (c *Config) normalizeProfiles() {
	if c.ActiveProfileID == "" {
		c.ActiveProfileID = DefaultProfileID
	}
	if len(c.Profiles) == 0 {
		c.Profiles = []Profile{c.defaultProfileFromCurrent()}
	}

	seenNames := map[string]bool{}
	defaultFound := false
	activeFound := false
	for i := range c.Profiles {
		profile := &c.Profiles[i]
		profile.ID = strings.TrimSpace(profile.ID)
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.ID == "" {
			profile.ID = profileIDFromName(profile.Name, i)
		}
		if profile.Name == "" {
			profile.Name = "Profile"
		}
		if profile.ID == DefaultProfileID {
			profile.Name = "Default"
			defaultFound = true
		}
		if profile.Mode == "" {
			profile.Mode = c.CurrentMode
		}
		if profile.Mode == "" {
			profile.Mode = ModePNG
		}
		if len(profile.SceneMappings) == 0 && len(c.SceneMappings) > 0 {
			profile.SceneMappings = cloneSceneMappings(c.SceneMappings)
		}
		if len(profile.RewardMappings) == 0 && len(c.RewardMappings) > 0 {
			profile.RewardMappings = cloneRewardMappings(c.RewardMappings)
		}
		normalizedNameKey := strings.ToLower(profile.Name)
		if seenNames[normalizedNameKey] && profile.ID != DefaultProfileID {
			profile.Name = uniqueProfileName(profile.Name, seenNames)
			normalizedNameKey = strings.ToLower(profile.Name)
		}
		seenNames[normalizedNameKey] = true
		if profile.ID == c.ActiveProfileID {
			activeFound = true
		}
	}
	if !defaultFound {
		c.Profiles = append([]Profile{c.defaultProfileFromCurrent()}, c.Profiles...)
	}
	if !activeFound {
		c.ActiveProfileID = DefaultProfileID
	}
}

func (c Config) defaultProfileFromCurrent() Profile {
	mode := c.CurrentMode
	if mode == "" {
		mode = ModePNG
	}
	return Profile{
		ID:             DefaultProfileID,
		Name:           "Default",
		Mode:           mode,
		Sources:        c.Sources,
		SceneMappings:  cloneSceneMappings(c.SceneMappings),
		RewardMappings: cloneRewardMappings(c.RewardMappings),
	}
}

func profileIDFromName(name string, index int) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "profile"
	}
	var builder strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case builder.Len() > 0 && builder.String()[builder.Len()-1] != '-':
			builder.WriteRune('-')
		}
	}
	id := strings.Trim(builder.String(), "-")
	if id == "" {
		id = "profile"
	}
	if index > 0 {
		return id + "-" + strconv.Itoa(index+1)
	}
	return id
}

func uniqueProfileName(base string, seen map[string]bool) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "Profile"
	}
	for i := 2; ; i++ {
		next := base + " " + strconv.Itoa(i)
		if !seen[strings.ToLower(next)] {
			return next
		}
	}
}

func cloneSceneMappings(mappings []SceneMapping) []SceneMapping {
	return append([]SceneMapping(nil), mappings...)
}

func cloneRewardMappings(mappings []RewardMapping) []RewardMapping {
	return append([]RewardMapping(nil), mappings...)
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
