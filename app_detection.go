package main

import (
	"fmt"

	"TuberSwitch/internal/appdetect"
	"TuberSwitch/internal/config"
)

// Blank targets preserve legacy most-recent-profile selection and mode checks.
// An explicit missing or mismatched target must never silently fall back.
func (a *App) detectionTargetLocked(mode config.Mode) (config.Profile, bool, error) {
	id := a.cfg.AppDetection.PNGProfileID
	if mode == config.Mode3D {
		id = a.cfg.AppDetection.ThreeDProfileID
	}
	if id == "" {
		p, ok := a.profileForModeLocked(mode)
		return p, ok, nil
	}
	p, ok := a.profileByIDLocked(id)
	if !ok {
		return config.Profile{ID: id}, false, appdetect.ErrTargetMissing
	}
	if p.Mode != mode {
		return p, false, appdetect.ErrTargetModeMismatch
	}
	return p, true, nil
}
func (a *App) resolveDetectionTarget(mode config.Mode) (appdetect.Target, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, _, err := a.detectionTargetLocked(mode)
	explicit := a.cfg.AppDetection.PNGProfileID != ""
	if mode == config.Mode3D {
		explicit = a.cfg.AppDetection.ThreeDProfileID != ""
	}
	if !explicit && a.cfg.CurrentMode == mode {
		p.ID = a.cfg.ActiveProfileID
	}
	return appdetect.Target{ProfileID: p.ID, Active: a.cfg.CurrentMode == mode && (!explicit || a.cfg.ActiveProfileID == p.ID)}, err
}
func (a *App) detectionDiagnosticsLocked() appdetect.Diagnostics {
	if a.detector != nil {
		return a.detector.Snapshot().Diagnostics
	}
	reason := "disabled"
	if a.cfg.AppDetection.Enabled {
		reason = "waiting"
	}
	return appdetect.Diagnostics{Reason: reason}
}
func validateDetectionTargets(cfg config.Config) ([]string, []string) {
	problems, warnings := validateAppDetectionConfig(cfg.AppDetection)
	for _, target := range []struct {
		id    string
		mode  config.Mode
		label string
	}{
		{cfg.AppDetection.ThreeDProfileID, config.Mode3D, "3D"}, {cfg.AppDetection.PNGProfileID, config.ModePNG, "PNG"},
	} {
		if target.id == "" {
			continue
		}
		found := false
		for _, p := range cfg.Profiles {
			if p.ID != target.id {
				continue
			}
			found = true
			if p.Mode != target.mode {
				problems = append(problems, fmt.Sprintf("%s detection target must use %s mode. Correct the profile mode or choose another target.", target.label, target.mode))
			}
			break
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s detection target profile no longer exists. Choose another target.", target.label))
		}
	}
	return problems, warnings
}
