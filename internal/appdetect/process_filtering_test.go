package appdetect

import "testing"

func TestFilterProcessesAppliesDefaultStyleFiltersAndDedupes(t *testing.T) {
	processes := []ProcessSummary{
		{ProcessName: "AvatarApp.exe", PID: 400, ExecutablePath: `C:\Apps\AvatarApp.exe`, HasVisibleWindow: true},
		{ProcessName: "AvatarApp.exe", PID: 123, ExecutablePath: `D:\Apps\AvatarApp.exe`, HasVisibleWindow: true},
		{ProcessName: "chrome.exe", PID: 200, ExecutablePath: `C:\Program Files\Google\Chrome\Application\chrome.exe`, HasVisibleWindow: true},
		{ProcessName: "Bitwarden.exe", PID: 300, ExecutablePath: `C:\Users\Don\AppData\Local\Programs\Bitwarden\Bitwarden.exe`, HasVisibleWindow: true},
		{ProcessName: "BackgroundAvatarHelper.exe", PID: 500, ExecutablePath: `C:\Apps\BackgroundAvatarHelper.exe`, HasVisibleWindow: false},
		{ProcessName: "svchost.exe", PID: 50, ExecutablePath: `C:\Windows\System32\svchost.exe`, IsSystemProcess: true},
	}

	filtered := FilterProcesses(processes, ProcessFilterOptions{
		ShowOnlyVisibleApps:     true,
		HideSystemProcesses:     true,
		HideCommonDesktopApps:   true,
		HideHelpersAndUtilities: true,
		LikelyAvatarAppsOnly:    true,
	})

	if len(filtered) != 1 {
		t.Fatalf("filtered = %#v", filtered)
	}
	if filtered[0].ProcessName != "AvatarApp.exe" || filtered[0].PID != 123 {
		t.Fatalf("unexpected filtered result: %#v", filtered[0])
	}
}

func TestFilterProcessesCanExposeHelperWhenFiltersRelaxed(t *testing.T) {
	processes := []ProcessSummary{
		{ProcessName: "Bitwarden.exe", PID: 300, ExecutablePath: `C:\Users\Don\AppData\Local\Programs\Bitwarden\Bitwarden.exe`, HasVisibleWindow: true},
	}

	filtered := FilterProcesses(processes, ProcessFilterOptions{
		ShowOnlyVisibleApps:     true,
		HideSystemProcesses:     true,
		HideCommonDesktopApps:   true,
		HideHelpersAndUtilities: false,
		LikelyAvatarAppsOnly:    false,
	})

	if len(filtered) != 1 || filtered[0].ProcessName != "Bitwarden.exe" {
		t.Fatalf("filtered = %#v", filtered)
	}
}

func TestFilterProcessesSearchMatchesExecutablePath(t *testing.T) {
	processes := []ProcessSummary{
		{ProcessName: "mystery.exe", PID: 22, ExecutablePath: `C:\Apps\AvatarSuite\Mystery.exe`, HasVisibleWindow: true},
	}

	filtered := FilterProcesses(processes, ProcessFilterOptions{
		Search:               "avatarsuite",
		ShowOnlyVisibleApps:  true,
		LikelyAvatarAppsOnly: false,
	})

	if len(filtered) != 1 {
		t.Fatalf("filtered = %#v", filtered)
	}
}
