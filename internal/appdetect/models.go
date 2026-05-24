package appdetect

import (
	"strings"

	"TuberSwitch/internal/config"
)

const (
	StatusDisabled       = "Disabled"
	StatusEnabled        = "Enabled"
	StatusThreeDDetected = "3D app detected"
	StatusPNGDetected    = "PNG app detected"
	StatusBothDetected   = "Both apps detected"
	StatusNoAppsDetected = "No apps detected"
)

type Snapshot struct {
	Status           string
	ThreeDRunning    bool
	PNGRunning       bool
	DetectedNames    []string
	DesiredMode      config.Mode
	ShouldSwitch     bool
	ApplyTwitch      bool
	ConflictResolved bool
}

type ProcessProvider interface {
	ListProcessNames() ([]string, error)
}

type Evaluation struct {
	Status       string
	DesiredMode  config.Mode
	ShouldSwitch bool
}

func Evaluate(cfg config.AppDetectionConfig, names []string) Evaluation {
	threeDRunning := containsProcessName(names, cfg.ThreeDProcessName)
	pngRunning := containsProcessName(names, cfg.PNGProcessName)

	switch {
	case threeDRunning && pngRunning:
		switch cfg.ConflictBehavior {
		case config.AppDetectionConflictPrefer3D:
			return Evaluation{Status: StatusBothDetected, DesiredMode: config.Mode3D, ShouldSwitch: true}
		case config.AppDetectionConflictPreferPNG:
			return Evaluation{Status: StatusBothDetected, DesiredMode: config.ModePNG, ShouldSwitch: true}
		default:
			return Evaluation{Status: StatusBothDetected}
		}
	case threeDRunning:
		return Evaluation{Status: StatusThreeDDetected, DesiredMode: config.Mode3D, ShouldSwitch: true}
	case pngRunning:
		return Evaluation{Status: StatusPNGDetected, DesiredMode: config.ModePNG, ShouldSwitch: true}
	default:
		return Evaluation{Status: StatusNoAppsDetected}
	}
}

func containsProcessName(names []string, target string) bool {
	if strings.TrimSpace(target) == "" {
		return false
	}
	target = strings.ToLower(strings.TrimSpace(target))
	for _, name := range names {
		if strings.ToLower(strings.TrimSpace(name)) == target {
			return true
		}
	}
	return false
}

func matchedProcessNames(names []string, cfg config.AppDetectionConfig) []string {
	matches := []string{}
	if containsProcessName(names, cfg.ThreeDProcessName) {
		matches = append(matches, strings.TrimSpace(cfg.ThreeDProcessName))
	}
	if containsProcessName(names, cfg.PNGProcessName) {
		matches = append(matches, strings.TrimSpace(cfg.PNGProcessName))
	}
	return matches
}
