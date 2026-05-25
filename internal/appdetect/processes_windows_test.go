//go:build windows

package appdetect

import "testing"

func TestLooksLikeSystemProcess(t *testing.T) {
	t.Run("session zero", func(t *testing.T) {
		if !looksLikeSystemProcess("avatarapp.exe", processInfo{sessionID: 0, hasSessionID: true}) {
			t.Fatalf("expected session-zero process to be classified as system")
		}
	})

	t.Run("service owner", func(t *testing.T) {
		if !looksLikeSystemProcess("avatarapp.exe", processInfo{ownerName: "NT AUTHORITY\\SYSTEM"}) {
			t.Fatalf("expected SYSTEM-owned process to be classified as system")
		}
	})

	t.Run("windows path", func(t *testing.T) {
		if !looksLikeSystemProcess("avatarapp.exe", processInfo{executablePath: ` C:/Windows/System32/notepad.exe `}) {
			t.Fatalf("expected Windows path process to be classified as system")
		}
	})

	t.Run("known system name", func(t *testing.T) {
		if !looksLikeSystemProcess(" svchost.exe ", processInfo{}) {
			t.Fatalf("expected known system process name to be classified as system")
		}
	})

	t.Run("normal desktop app", func(t *testing.T) {
		if looksLikeSystemProcess("AvatarApp.exe", processInfo{
			sessionID:      1,
			hasSessionID:   true,
			ownerName:      `don-pc\don`,
			executablePath: `C:\Program Files\Avatar App\AvatarApp.exe`,
		}) {
			t.Fatalf("did not expect normal desktop app to be classified as system")
		}
	})
}

func TestNormalizeProcessHelpers(t *testing.T) {
	if got := normalizeProcessPath(` C:/Program Files/App/AvatarApp.exe `); got != `c:\program files\app\avatarapp.exe` {
		t.Fatalf("normalizeProcessPath = %q", got)
	}
	if got := normalizeProcessToken(` NT AUTHORITY\LOCAL SERVICE `); got != `nt authority\local service` {
		t.Fatalf("normalizeProcessToken = %q", got)
	}
	if !isServiceOwner(`NT AUTHORITY\NETWORK SERVICE`) {
		t.Fatalf("expected service owner classification")
	}
	if isServiceOwner(`don-pc\don`) {
		t.Fatalf("did not expect normal user owner classification")
	}
	if !isKnownSystemProcessName(` DWM.EXE `) {
		t.Fatalf("expected known system process name match")
	}
	if isKnownSystemProcessName(`avatarapp.exe`) {
		t.Fatalf("did not expect avatar app to match known system process names")
	}
}
