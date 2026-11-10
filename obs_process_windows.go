//go:build windows

package main

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

// Inspect executable names only; no window enumeration, paths, or owner queries.
func localOBSRunning() (bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false, err
	}
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if isOBSProcessName(name) {
			return true, nil
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return false, nil
			}
			return false, err
		}
	}
}
