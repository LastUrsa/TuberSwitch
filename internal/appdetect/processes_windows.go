//go:build windows

package appdetect

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type WindowsProcessProvider struct{}

var (
	user32DLL                    = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32DLL.NewProc("EnumWindows")
	procGetWindowThreadProcessID = user32DLL.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32DLL.NewProc("IsWindowVisible")
	procGetWindow                = user32DLL.NewProc("GetWindow")
)

const gwOwner = 4

func (WindowsProcessProvider) ListProcesses() ([]ProcessSummary, error) {
	visibleWindowPIDs := visibleWindowProcessIDs()
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}

	processes := []ProcessSummary{}
	for {
		name := strings.TrimSpace(windows.UTF16ToString(entry.ExeFile[:]))
		processInfo := queryProcessInfo(entry.ProcessID)
		processes = append(processes, ProcessSummary{
			ProcessName:      name,
			PID:              int(entry.ProcessID),
			ExecutablePath:   processInfo.executablePath,
			IsSystemProcess:  looksLikeSystemProcess(name, processInfo),
			HasVisibleWindow: visibleWindowPIDs[entry.ProcessID],
		})

		err = windows.Process32Next(snapshot, &entry)
		if err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return nil, err
		}
	}

	sortProcessSummaries(processes)
	return processes, nil
}

func visibleWindowProcessIDs() map[uint32]bool {
	visiblePIDs := map[uint32]bool{}
	callback := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if !isWindowVisible(hwnd) || hasOwnerWindow(hwnd) {
			return 1
		}
		var pid uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != 0 {
			visiblePIDs[pid] = true
		}
		return 1
	})
	procEnumWindows.Call(callback, 0)
	return visiblePIDs
}

func isWindowVisible(hwnd uintptr) bool {
	result, _, _ := procIsWindowVisible.Call(hwnd)
	return result != 0
}

func hasOwnerWindow(hwnd uintptr) bool {
	owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
	return owner != 0
}

type processInfo struct {
	executablePath string
	sessionID      uint32
	hasSessionID   bool
	ownerName      string
}

func queryProcessInfo(pid uint32) processInfo {
	info := processInfo{}
	if sessionID, ok := querySessionID(pid); ok {
		info.sessionID = sessionID
		info.hasSessionID = true
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return info
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err == nil && size > 0 {
		info.executablePath = windows.UTF16ToString(buffer[:size])
	}

	if ownerName, ok := queryProcessOwner(handle); ok {
		info.ownerName = ownerName
	}

	return info
}

func querySessionID(pid uint32) (uint32, bool) {
	var sessionID uint32
	if err := windows.ProcessIdToSessionId(pid, &sessionID); err != nil {
		return 0, false
	}
	return sessionID, true
}

func queryProcessOwner(handle windows.Handle) (string, bool) {
	var token windows.Token
	if err := windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token); err != nil {
		return "", false
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return "", false
	}

	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return "", false
	}
	if domain == "" {
		return strings.ToLower(strings.TrimSpace(account)), true
	}
	return strings.ToLower(strings.TrimSpace(domain + `\` + account)), true
}

func looksLikeSystemProcess(processName string, info processInfo) bool {
	if isSessionZeroProcess(info) {
		return true
	}

	if isServiceOwner(info.ownerName) {
		return true
	}

	if isWindowsSystemPath(info.executablePath) {
		return true
	}

	return isKnownSystemProcessName(processName)
}

func isSessionZeroProcess(info processInfo) bool {
	return info.hasSessionID && info.sessionID == 0
}

func isServiceOwner(ownerName string) bool {
	switch normalizeProcessToken(ownerName) {
	case "nt authority\\system", "nt authority\\local service", "nt authority\\network service":
		return true
	default:
		return false
	}
}

func isWindowsSystemPath(executablePath string) bool {
	normalizedPath := normalizeProcessPath(executablePath)
	return normalizedPath != "" && strings.HasPrefix(normalizedPath, `c:\windows\`)
}

func isKnownSystemProcessName(processName string) bool {
	switch normalizeProcessToken(processName) {
	case "system", "registry", "smss.exe", "csrss.exe", "wininit.exe", "services.exe", "lsass.exe", "svchost.exe", "fontdrvhost.exe", "winlogon.exe", "taskhostw.exe", "dwm.exe":
		return true
	default:
		return false
	}
}
