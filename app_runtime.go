package main

import (
	"fmt"
	"time"

	appdto "TuberSwitch/internal/app"
	"TuberSwitch/internal/config"
)

func (a *App) GetStatus() appdto.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) SaveConfig(input appdto.SettingsInput) appdto.ActionResult {
	a.mu.Lock()
	next := a.updatedConfigLocked(input.Config)
	validationErrors, validationWarnings := validateAppDetectionConfig(next.AppDetection)
	if len(validationErrors) > 0 {
		defer a.mu.Unlock()
		return a.resultLocked(false, "App Detection settings are invalid", validationWarnings, validationErrors)
	}
	oldPassword := a.cfg.OBS.Password
	if input.UpdateOBSPassword {
		if err := a.secretStore.SaveOBSPassword(input.OBSPassword); err != nil {
			defer a.mu.Unlock()
			return a.resultLocked(false, "OBS password save failed", nil, []string{err.Error()})
		}
		next.OBS.Password = input.OBSPassword
	} else {
		next.OBS.Password = oldPassword
	}
	if err := a.store.Save(next); err != nil {
		if input.UpdateOBSPassword {
			_ = a.secretStore.SaveOBSPassword(oldPassword)
		}
		defer a.mu.Unlock()
		return a.resultLocked(false, "Config save failed", nil, []string{err.Error()})
	}
	a.cfg = next
	a.lastAction = "Settings saved"
	appDetection := a.cfg.AppDetection
	a.mu.Unlock()

	if a.detector != nil {
		a.detector.Start(appDetection)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.resultLocked(true, "Settings saved", validationWarnings, nil)
}

func (a *App) ApplyMode(mode config.Mode) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.applyModeLocked(mode, applyModeOptions{
		applyTwitchChanges: true,
		source:             "manual",
		recordManualSwitch: true,
	})
}

func (a *App) statusLocked() appdto.Status {
	label := string(a.cfg.CurrentMode)
	if profile, ok := a.cfg.Profile(a.cfg.CurrentMode); ok {
		label = profile.DisplayName
	}
	return appdto.Status{
		Config:              a.settingsLocked(),
		CurrentMode:         a.cfg.CurrentMode,
		CurrentModeLabel:    label,
		OBSConnected:        a.obs.Connected(),
		TwitchConnected:     a.cfg.Twitch.AccessToken != "",
		LastAction:          a.lastAction,
		AppDetectionStatus:  a.appDetectionStatusLocked(),
		AppDetectionEnabled: a.cfg.AppDetection.Enabled,
	}
}

func (a *App) resultLocked(ok bool, message string, warnings []string, errors []string) appdto.ActionResult {
	return appdto.ActionResult{OK: ok, Message: message, Warnings: warnings, Errors: errors, NewStatus: a.statusLocked()}
}

func (a *App) withError(message string) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAction = message
	return a.resultLocked(false, message, nil, []string{message})
}

func (a *App) settingsLocked() appdto.Settings {
	return appdto.Settings{
		OBS: appdto.OBSSettings{
			Host:               a.cfg.OBS.Host,
			Port:               a.cfg.OBS.Port,
			AllowRemote:        a.cfg.OBS.AllowRemote,
			PasswordConfigured: a.cfg.OBS.Password != "",
		},
		Sources:                 a.cfg.Sources,
		SceneMappings:           append([]config.SceneMapping(nil), a.cfg.SceneMappings...),
		Twitch:                  appdto.TwitchSettings{ClientID: a.cfg.Twitch.ClientID, ChannelID: a.cfg.Twitch.ChannelID, ChannelName: a.cfg.Twitch.ChannelName},
		ModeProfiles:            append([]config.ModeProfile(nil), a.cfg.ModeProfiles...),
		StartupMode:             a.cfg.StartupMode,
		CurrentMode:             a.cfg.CurrentMode,
		RefreshRewardsOnStartup: a.cfg.RefreshRewardsOnStartup,
		AppDetection:            a.cfg.AppDetection,
	}
}

func (a *App) updatedConfigLocked(settings appdto.Settings) config.Config {
	next := a.cfg
	next.OBS.Host = settings.OBS.Host
	next.OBS.Port = settings.OBS.Port
	next.OBS.AllowRemote = settings.OBS.AllowRemote
	next.Sources = settings.Sources
	next.SceneMappings = append([]config.SceneMapping(nil), settings.SceneMappings...)
	next.Twitch.ClientID = settings.Twitch.ClientID
	next.Twitch.ChannelID = settings.Twitch.ChannelID
	next.Twitch.ChannelName = settings.Twitch.ChannelName
	next.ModeProfiles = append([]config.ModeProfile(nil), settings.ModeProfiles...)
	next.StartupMode = settings.StartupMode
	next.CurrentMode = settings.CurrentMode
	next.RefreshRewardsOnStartup = settings.RefreshRewardsOnStartup
	next.AppDetection = settings.AppDetection
	next.Normalize()
	return next
}

func (a *App) applyModeLocked(mode config.Mode, options applyModeOptions) appdto.ActionResult {
	warnings := []string{}
	errors := []string{}
	if err := a.applyOBSMode(mode); err != nil {
		errors = append(errors, err.Error())
	}
	if options.applyTwitchChanges {
		if errList := a.applyTwitchModeLocked(mode); len(errList) > 0 {
			if options.source == "auto" {
				warnings = append(warnings, errList...)
			} else {
				errors = append(errors, errList...)
			}
		}
	}
	a.cfg.CurrentMode = mode
	if err := a.store.Save(a.cfg); err != nil {
		errors = append(errors, err.Error())
	}
	if options.recordManualSwitch && a.detector != nil {
		a.detector.RecordManualOverride(time.Duration(a.cfg.AppDetection.ManualOverrideCooldownSeconds) * time.Second)
	}

	switch {
	case len(errors) > 0:
		if options.source == "auto" {
			a.lastAction = fmt.Sprintf("Auto-switch to %s mode failed", mode)
		} else {
			a.lastAction = fmt.Sprintf("Switched to %s mode with errors", mode)
		}
		return a.resultLocked(false, a.lastAction, warnings, errors)
	case len(warnings) > 0:
		a.lastAction = fmt.Sprintf("Auto-switched to %s mode with warnings", mode)
		return a.resultLocked(true, a.lastAction, warnings, nil)
	default:
		if options.source == "auto" {
			a.lastAction = fmt.Sprintf("Auto-switched to %s Mode", mode)
		} else {
			a.lastAction = fmt.Sprintf("Switched to %s Mode successfully", mode)
		}
		return a.resultLocked(true, a.lastAction, nil, nil)
	}
}
