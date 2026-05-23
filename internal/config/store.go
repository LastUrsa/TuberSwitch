package config

import (
	"encoding/json"
	"errors"
	"os"
)

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, s.Save(cfg)
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.Normalize()
	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	cfg.Normalize()
	data, err := json.MarshalIndent(toPersistedConfig(cfg), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

type persistedConfig struct {
	OBS                     persistedOBSConfig `json:"obs"`
	Sources                 SourcesConfig      `json:"sources"`
	SceneMappings           []SceneMapping     `json:"sceneMappings"`
	Twitch                  persistedTwitch    `json:"twitch"`
	RewardMappings          []RewardMapping    `json:"rewardMappings"`
	ModeProfiles            []ModeProfile      `json:"modeProfiles"`
	StartupMode             StartupMode        `json:"startupMode"`
	CurrentMode             Mode               `json:"currentMode"`
	RefreshRewardsOnStartup bool               `json:"refreshRewardsOnStartup"`
}

type persistedOBSConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	AllowRemote bool   `json:"allowRemote"`
}

type persistedTwitch struct {
	ClientID    string `json:"clientId"`
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
}

func toPersistedConfig(cfg Config) persistedConfig {
	return persistedConfig{
		OBS: persistedOBSConfig{
			Host:        cfg.OBS.Host,
			Port:        cfg.OBS.Port,
			AllowRemote: cfg.OBS.AllowRemote,
		},
		Sources:       cfg.Sources,
		SceneMappings: cfg.SceneMappings,
		Twitch: persistedTwitch{
			ClientID:    cfg.Twitch.ClientID,
			ChannelID:   cfg.Twitch.ChannelID,
			ChannelName: cfg.Twitch.ChannelName,
		},
		RewardMappings:          cfg.RewardMappings,
		ModeProfiles:            cfg.ModeProfiles,
		StartupMode:             cfg.StartupMode,
		CurrentMode:             cfg.CurrentMode,
		RefreshRewardsOnStartup: cfg.RefreshRewardsOnStartup,
	}
}
