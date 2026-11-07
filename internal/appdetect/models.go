package appdetect

import (
	"errors"
	"sort"
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

var ErrTargetMissing = errors.New("detection target profile is missing")
var ErrTargetModeMismatch = errors.New("detection target profile has the wrong mode")

// Diagnostics contains only operational state; never the process inventory,
// profile names, paths, credentials, or raw integration errors.
type Diagnostics struct {
	DesiredMode     config.Mode `json:"desiredMode,omitempty"`
	TargetProfileID string      `json:"targetProfileId,omitempty"`
	Reason          string      `json:"reason"`
	ErrorCode       string      `json:"errorCode,omitempty"`
	ObservedAt      string      `json:"observedAt,omitempty"`
	CooldownUntil   string      `json:"cooldownUntil,omitempty"`
	ThreeDRunning   bool        `json:"threeDRunning"`
	PNGRunning      bool        `json:"pngRunning"`
}
type Target struct {
	ProfileID string
	Active    bool
}
type ResolveTargetFunc func(config.Mode) (Target, error)

type Snapshot struct {
	Diagnostics      Diagnostics
	Status           string
	ThreeDRunning    bool
	PNGRunning       bool
	DetectedNames    []string
	DesiredMode      config.Mode
	ShouldSwitch     bool
	ApplyTwitch      bool
	ConflictResolved bool
}

type ProcessSummary struct {
	ProcessName      string
	PID              int
	ExecutablePath   string
	IsSystemProcess  bool
	HasVisibleWindow bool
}

type ProcessProvider interface {
	ListProcesses() ([]ProcessSummary, error)
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

func ProcessNames(processes []ProcessSummary) []string {
	names := make([]string, 0, len(processes))
	for _, process := range processes {
		names = append(names, process.ProcessName)
	}
	return names
}

func sortProcessSummaries(processes []ProcessSummary) {
	sort.Slice(processes, func(i, j int) bool {
		left := strings.ToLower(processes[i].ProcessName)
		right := strings.ToLower(processes[j].ProcessName)
		if left == right {
			return processes[i].PID < processes[j].PID
		}
		return left < right
	})
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
