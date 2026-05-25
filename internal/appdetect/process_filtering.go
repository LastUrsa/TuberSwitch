package appdetect

import "strings"

type ProcessFilterOptions struct {
	Search                  string
	ShowOnlyVisibleApps     bool
	HideSystemProcesses     bool
	HideCommonDesktopApps   bool
	HideHelpersAndUtilities bool
	LikelyAvatarAppsOnly    bool
}

var commonDesktopAppNames = map[string]struct{}{
	"acrocef.exe": {}, "acrobat.exe": {}, "acrotray.exe": {}, "adobe cef helper.exe": {}, "adobe desktop service.exe": {},
	"adobeipcbroker.exe": {}, "adobearm.exe": {}, "afterfx.exe": {}, "agent.exe": {}, "amazon music.exe": {},
	"android studio.exe": {}, "anydesk.exe": {}, "arc.exe": {}, "armsvc.exe": {}, "audacity.exe": {},
	"battle.net.exe": {}, "battlenet.exe": {}, "brave.exe": {}, "braveupdate.exe": {}, "browser_assistant.exe": {},
	"ccxprocess.exe": {}, "cefsharp.browsersubprocess.exe": {}, "chrome.exe": {}, "chrome_proxy.exe": {}, "code.exe": {},
	"codeium.exe": {}, "creative cloud helper.exe": {}, "creative cloud.exe": {}, "cursor.exe": {}, "deepl.exe": {},
	"discordcanary.exe": {}, "discordptb.exe": {}, "discord.exe": {}, "docker desktop.exe": {}, "docker.exe": {},
	"dropbox.exe": {}, "epicgameslauncher.exe": {}, "everything.exe": {}, "excel.exe": {}, "explorer.exe": {},
	"figma.exe": {}, "figmaagent.exe": {}, "firefox.exe": {}, "foxitpdfreader.exe": {}, "galaxyclient.exe": {},
	"gitkraken.exe": {}, "gog galaxy notifications renderer.exe": {}, "googledrivefs.exe": {}, "googledrivesync.exe": {}, "heroic.exe": {},
	"idea64.exe": {}, "illustrator.exe": {}, "itunes.exe": {}, "joplin.exe": {}, "lightshot.exe": {},
	"logi_overlay.exe": {}, "logioptions.exe": {}, "logioptionsplus.exe": {}, "logioptionsplus_agent.exe": {}, "logioptionsplus_appbroker.exe": {},
	"microsoft photos.exe": {}, "microsoft.photos.exe": {}, "ms-teams.exe": {}, "msedge.exe": {}, "msedgewebview2.exe": {},
	"notion.exe": {}, "obsidian.exe": {}, "onedrivesetup.exe": {}, "onedrive.exe": {}, "opera.exe": {},
	"parsecd.exe": {}, "pdf24-editor.exe": {}, "photoshop.exe": {}, "postman.exe": {}, "powershell.exe": {},
	"powerpnt.exe": {}, "premiere pro.exe": {}, "qbittorrent.exe": {}, "raidrive.exe": {}, "razer synapse 3.exe": {},
	"razer synapse service process.exe": {}, "razercentral.exe": {}, "reader_sl.exe": {}, "riotclientservices.exe": {}, "riotclientux.exe": {},
	"rubymine64.exe": {}, "searchhost.exe": {}, "searchindexer.exe": {}, "signal.exe": {}, "skype.exe": {},
	"slack.exe": {}, "spotifylauncher.exe": {}, "spotify.exe": {}, "steamerrorreporter.exe": {}, "steamhelper.exe": {},
	"steamservice.exe": {}, "steam.exe": {}, "steamwebhelper.exe": {}, "teams.exe": {}, "telegram.exe": {},
	"thunderbird.exe": {}, "todoist.exe": {}, "ubuntu.exe": {}, "upc.exe": {}, "utweb.exe": {},
	"vivaldi.exe": {}, "vlc.exe": {}, "wacom_tabletuser.exe": {}, "wacom_touchuser.exe": {}, "wacomhost.exe": {},
	"webex.exe": {}, "webexmta.exe": {}, "whatsapp.exe": {}, "winword.exe": {}, "zoom.exe": {}, "zoomit.exe": {},
}

var commonHelperAppNames = map[string]struct{}{
	"adobecollabsync.exe": {}, "adobenotificationclient.exe": {}, "bitwarden.exe": {}, "codex.exe": {}, "corsairgamingaudiocfgservice64.exe": {},
	"crashhelper.exe": {}, "crashpad_handler.exe": {}, "crossdeviceservice.exe": {}, "dsatray.exe": {}, "elgatoaudiocontrolserver.exe": {},
	"elgatoaudiocontrolserverwatcher.exe": {}, "fancontrol.exe": {}, "focusrite notifier.exe": {}, "focusritecontrolserver.exe": {}, "gamebarpresencewriter.exe": {},
	"gameinputredistservice.exe": {}, "gameinputsvc.exe": {}, "tidal.exe": {}, "tidalplayer.exe": {}, "uacloudhelper.exe": {},
	"ua connect.exe": {}, "video.ui.exe": {}, "virtualdesktop.service.exe": {}, "wacom_tablet.exe": {}, "wacom_updateutil.exe": {},
	"widgetservice.exe": {}, "widgets.exe": {}, "xboxpcappft.exe": {},
}

var commonHelperPathFragments = []string{
	`\\adobe\\`,
	`\\bitwarden\\`,
	`\\corsair\\`,
	`\\elgato\\`,
	`\\focusrite\\`,
	`\\intel\\driver and support assistant\\`,
	`\\openai\\codex\\`,
	`\\program files\\ua connect\\`,
	`\\program files\\tablet\\wacom\\`,
	`\\program files\\windowsapps\\`,
	`\\razer\\`,
	`\\tidal\\`,
	`\\virtualdesktop\\`,
}

var likelyAvatarAppTokens = []string{
	"animaze",
	"avatar",
	"facerig",
	"kalidoface",
	"luppet",
	"pngtuber",
	"tuber",
	"veadotube",
	"vnyan",
	"vseeface",
	"vtube",
	"warudo",
}

func FilterProcesses(processes []ProcessSummary, options ProcessFilterOptions) []ProcessSummary {
	search := normalizeProcessToken(options.Search)
	filtered := make([]ProcessSummary, 0, len(processes))
	seen := map[string]ProcessSummary{}

	for _, process := range processes {
		if options.ShowOnlyVisibleApps && !process.HasVisibleWindow {
			continue
		}
		if options.HideSystemProcesses && process.IsSystemProcess {
			continue
		}
		if options.HideCommonDesktopApps && isCommonDesktopApp(process.ProcessName) {
			continue
		}
		if options.HideHelpersAndUtilities && isCommonHelperOrUtility(process) {
			continue
		}
		if options.LikelyAvatarAppsOnly && !isLikelyAvatarApp(process) {
			continue
		}
		if search != "" && !matchesProcessSearch(process, search) {
			continue
		}

		key := normalizeProcessToken(process.ProcessName)
		existing, ok := seen[key]
		if !ok || process.PID < existing.PID {
			seen[key] = process
		}
	}

	for _, process := range seen {
		filtered = append(filtered, process)
	}
	sortProcessSummaries(filtered)
	return filtered
}

func matchesProcessSearch(process ProcessSummary, search string) bool {
	return strings.Contains(normalizeProcessToken(process.ProcessName), search) ||
		strings.Contains(normalizeProcessPath(process.ExecutablePath), search)
}

func isCommonDesktopApp(processName string) bool {
	_, ok := commonDesktopAppNames[normalizeProcessToken(processName)]
	return ok
}

func isCommonHelperOrUtility(process ProcessSummary) bool {
	if _, ok := commonHelperAppNames[normalizeProcessToken(process.ProcessName)]; ok {
		return true
	}
	normalizedPath := normalizeProcessPath(process.ExecutablePath)
	for _, fragment := range commonHelperPathFragments {
		if strings.Contains(normalizedPath, fragment) {
			return true
		}
	}
	return false
}

func isLikelyAvatarApp(process ProcessSummary) bool {
	if isCommonDesktopApp(process.ProcessName) || isCommonHelperOrUtility(process) || process.IsSystemProcess {
		return false
	}
	target := normalizeProcessToken(process.ProcessName) + " " + normalizeProcessPath(process.ExecutablePath)
	for _, token := range likelyAvatarAppTokens {
		if strings.Contains(target, token) {
			return true
		}
	}
	return false
}

func normalizeProcessPath(executablePath string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(executablePath, "/", `\`)))
}

func normalizeProcessToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
