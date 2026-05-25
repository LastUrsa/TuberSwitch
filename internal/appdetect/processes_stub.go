//go:build !windows

package appdetect

type WindowsProcessProvider struct{}

func (WindowsProcessProvider) ListProcesses() ([]ProcessSummary, error) {
	return nil, nil
}
