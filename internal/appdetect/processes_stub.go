//go:build !windows

package appdetect

type WindowsProcessProvider struct{}

func (WindowsProcessProvider) ListProcessNames() ([]string, error) {
	return nil, nil
}
