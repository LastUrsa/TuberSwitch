//go:build windows

package appdetect

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type WindowsProcessProvider struct{}

func (WindowsProcessProvider) ListProcessNames() ([]string, error) {
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

	names := []string{}
	for {
		names = append(names, windows.UTF16ToString(entry.ExeFile[:]))
		err = windows.Process32Next(snapshot, &entry)
		if err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return nil, err
		}
	}

	return names, nil
}
