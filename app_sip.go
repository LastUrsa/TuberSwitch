package main

import (
	"context"
	"errors"
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

func sipProfileFromConfig(profile config.Profile) sip.Profile {
	return sip.Profile{
		ID:   profile.ID,
		Name: profile.Name,
		Mode: strings.ToLower(string(profile.Mode)),
	}
}
