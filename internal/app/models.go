package app

import "TuberSwitch/internal/config"

type Status struct {
	Config           config.Config `json:"config"`
	CurrentMode      config.Mode   `json:"currentMode"`
	CurrentModeLabel string        `json:"currentModeLabel"`
	OBSConnected     bool          `json:"obsConnected"`
	TwitchConnected  bool          `json:"twitchConnected"`
	LastAction       string        `json:"lastAction"`
	ConfigPath       string        `json:"configPath"`
	LogPath          string        `json:"logPath"`
}

type ActionResult struct {
	OK        bool     `json:"ok"`
	Message   string   `json:"message"`
	Warnings  []string `json:"warnings"`
	Errors    []string `json:"errors"`
	NewStatus Status   `json:"newStatus"`
}

type OBSScene struct {
	Name string `json:"name"`
}

type OBSSource struct {
	Name        string `json:"name"`
	SceneItemID int    `json:"sceneItemId"`
}

type OBSInventory struct {
	Scenes         []OBSScene             `json:"scenes"`
	Sources        []OBSSource            `json:"sources"`
	SourcesByScene map[string][]OBSSource `json:"sourcesByScene"`
}

type TwitchReward struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Enabled    bool   `json:"enabled"`
	Is3DOnly   bool   `json:"is3DOnly"`
	Manageable bool   `json:"manageable"`
}
