package app

import "TuberSwitch/internal/config"

type Status struct {
	Config              Settings    `json:"config"`
	CurrentMode         config.Mode `json:"currentMode"`
	CurrentModeLabel    string      `json:"currentModeLabel"`
	OBSConnected        bool        `json:"obsConnected"`
	TwitchConnected     bool        `json:"twitchConnected"`
	LastAction          string      `json:"lastAction"`
	AppDetectionStatus  string      `json:"appDetectionStatus"`
	AppDetectionEnabled bool        `json:"appDetectionEnabled"`
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

type ProcessSummary struct {
	ProcessName string `json:"processName"`
	PID         int    `json:"pid"`
}

type ProcessListOptions struct {
	Search                  string `json:"search"`
	ShowOnlyVisibleApps     bool   `json:"showOnlyVisibleApps"`
	HideSystemProcesses     bool   `json:"hideSystemProcesses"`
	HideCommonDesktopApps   bool   `json:"hideCommonDesktopApps"`
	HideHelpersAndUtilities bool   `json:"hideHelpersAndUtilities"`
	LikelyAvatarAppsOnly    bool   `json:"likelyAvatarAppsOnly"`
}

type TwitchReward struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Enabled    bool   `json:"enabled"`
	Is3DOnly   bool   `json:"is3DOnly"`
	Manageable bool   `json:"manageable"`
}

type Settings struct {
	OBS                     OBSSettings               `json:"obs"`
	Sources                 config.SourcesConfig      `json:"sources"`
	SceneMappings           []config.SceneMapping     `json:"sceneMappings"`
	Twitch                  TwitchSettings            `json:"twitch"`
	ModeProfiles            []config.ModeProfile      `json:"modeProfiles"`
	StartupMode             config.StartupMode        `json:"startupMode"`
	CurrentMode             config.Mode               `json:"currentMode"`
	RefreshRewardsOnStartup bool                      `json:"refreshRewardsOnStartup"`
	AppDetection            config.AppDetectionConfig `json:"appDetection"`
}

type OBSSettings struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	AllowRemote        bool   `json:"allowRemote"`
	PasswordConfigured bool   `json:"passwordConfigured"`
}

type TwitchSettings struct {
	ClientID    string `json:"clientId"`
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
}

type SettingsInput struct {
	Config            Settings `json:"config"`
	OBSPassword       string   `json:"obsPassword"`
	UpdateOBSPassword bool     `json:"updateObsPassword"`
}

type UpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	Message         string `json:"message"`
}
