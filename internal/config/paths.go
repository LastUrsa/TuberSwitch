package config

import (
	"os"
	"path/filepath"
)

const appDirName = "TuberSwitch"

func AppDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDirName)
	return dir, os.MkdirAll(dir, 0o700)
}

func ConfigPath() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LogPath() (string, error) {
	dir, err := AppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tuberswitch.log"), nil
}
