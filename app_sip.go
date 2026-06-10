package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"TuberSwitch/internal/config"
	"TuberSwitch/internal/sip"
)

func (a *App) SIPProfiles(context.Context) ([]sip.Profile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Normalize()
	profiles := make([]sip.Profile, 0, len(a.cfg.Profiles))
	for _, profile := range a.cfg.Profiles {
		profiles = append(profiles, sipProfileFromConfig(profile))
	}
	return profiles, nil
}

func (a *App) SIPCurrentProfile(context.Context) (sip.Profile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Normalize()
	profile, ok := a.cfg.ActiveProfile()
	if !ok {
		return sip.Profile{}, nil
	}
	return sipProfileFromConfig(profile), nil
}

func (a *App) SIPActivateProfile(ctx context.Context, name string) (sip.Profile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return sip.Profile{}, sip.ErrInvalidRequest
	}
	a.cfg.Normalize()
	for _, profile := range a.cfg.Profiles {
		if strings.EqualFold(profile.Name, name) {
			result := a.applyStreamProfileLocked(profile, applyModeOptions{
				applyTwitchChanges: true,
				source:             "profile",
				recordManualSwitch: true,
			})
			if !result.OK {
				if len(result.Errors) > 0 {
					return sip.Profile{}, errors.New(strings.Join(result.Errors, "; "))
				}
				return sip.Profile{}, errors.New(result.Message)
			}
			selected, ok := a.profileByIDLocked(profile.ID)
			if !ok {
				return sipProfileFromConfig(profile), nil
			}
			return sipProfileFromConfig(selected), nil
		}
	}
	return sip.Profile{}, sip.ErrProfileNotFound
}

func (a *App) SIPStatusDetails(context.Context) (sip.StatusDetails, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	obsConfigForSummary := a.cfg.OBS
	a.cfg.Normalize()

	activeProfile, hasActiveProfile := a.cfg.ActiveProfile()
	mode := a.cfg.CurrentMode
	if hasActiveProfile && activeProfile.Mode != "" {
		mode = activeProfile.Mode
	}

	sceneMappings := a.cfg.SceneMappings
	rewardMappings := a.cfg.RewardMappings
	if hasActiveProfile {
		sceneMappings = activeProfile.SceneMappings
		rewardMappings = activeProfile.RewardMappings
	}

	activeScene, activeSource := primarySIPSceneSource(sceneMappings, mode)
	obsConnected := a.obs != nil && a.obs.Connected()
	manageableRedeemCount, unmanageableRedeemCount := sipRedeemCounts(rewardMappings)

	label := string(a.cfg.CurrentMode)
	if profile, ok := a.cfg.Profile(a.cfg.CurrentMode); ok {
		label = profile.DisplayName
	}

	return sip.StatusDetails{
		OBSConnected:            obsConnected,
		OBSSummary:              sipOBSSummary(obsConnected, obsConfigForSummary, activeScene, activeSource),
		ActiveScene:             activeScene,
		ActiveSource:            activeSource,
		RedeemsEnabled:          strings.TrimSpace(a.cfg.Twitch.AccessToken) != "" && manageableRedeemCount > 0,
		RedeemCount:             len(rewardMappings),
		ManageableRedeemCount:   manageableRedeemCount,
		UnmanageableRedeemCount: unmanageableRedeemCount,
		AppDetectionEnabled:     a.cfg.AppDetection.Enabled,
		AppDetectionStatus:      a.appDetectionStatusLocked(),
		CurrentModeLabel:        label,
		ActiveProfileLastUsed:   activeProfile.LastUsed,
	}, nil
}

func sipRedeemCounts(mappings []config.RewardMapping) (int, int) {
	manageable := 0
	unmanageable := 0
	for _, mapping := range mappings {
		if mapping.Manageable {
			manageable++
			continue
		}
		unmanageable++
	}
	return manageable, unmanageable
}

func sipProfileFromConfig(profile config.Profile) sip.Profile {
	return sip.Profile{
		ID:   profile.ID,
		Name: profile.Name,
		Mode: strings.ToLower(string(profile.Mode)),
	}
}

func primarySIPSceneSource(mappings []config.SceneMapping, mode config.Mode) (string, string) {
	for _, mapping := range mappings {
		if !mapping.Enabled {
			continue
		}
		source := mapping.PNGTuberSource
		if mode == config.Mode3D {
			source = mapping.VTuberSource
		}
		return mapping.Scene, source
	}
	return "", ""
}

func sipOBSSummary(connected bool, obsConfig config.OBSConfig, scene string, source string) string {
	if !connected {
		if strings.TrimSpace(obsConfig.Host) == "" || obsConfig.Port == 0 {
			return "OBS not configured"
		}
		return "OBS not connected"
	}
	if strings.TrimSpace(scene) != "" && strings.TrimSpace(source) != "" {
		return fmt.Sprintf("Connected: %s / %s", scene, source)
	}
	return "Connected: no source selected"
}
