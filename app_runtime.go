package main

import (
	"fmt"
	"strconv"
	"strings"
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

func (a *App) SaveProfile(input appdto.SettingsInput) appdto.ActionResult {
	a.mu.Lock()
	next := a.updatedConfigLocked(input.Config)
	validationErrors, validationWarnings := validateAppDetectionConfig(next.AppDetection)
	if len(validationErrors) > 0 {
		defer a.mu.Unlock()
		return a.resultLocked(false, "Profile settings are invalid", validationWarnings, validationErrors)
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
	next.UpsertActiveProfileFromCurrent(time.Now().UTC().Format(time.RFC3339))
	if err := a.store.Save(next); err != nil {
		if input.UpdateOBSPassword {
			_ = a.secretStore.SaveOBSPassword(oldPassword)
		}
		defer a.mu.Unlock()
		return a.resultLocked(false, "Profile save failed", nil, []string{err.Error()})
	}
	a.cfg = next
	a.lastAction = "Profile saved"
	appDetection := a.cfg.AppDetection
	a.mu.Unlock()

	if a.detector != nil {
		a.detector.Start(appDetection)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.resultLocked(true, "Profile saved", validationWarnings, nil)
}

func (a *App) SaveProfileAs(name string, input appdto.SettingsInput) appdto.ActionResult {
	a.mu.Lock()
	next := a.updatedConfigLocked(input.Config)
	name = strings.TrimSpace(name)
	if name == "" {
		defer a.mu.Unlock()
		return a.resultLocked(false, "Profile name is required", nil, []string{"Profile name is required"})
	}
	if profileNameExists(next.Profiles, name, "") {
		defer a.mu.Unlock()
		return a.resultLocked(false, "Profile name must be unique", nil, []string{"A profile with that name already exists."})
	}
	validationErrors, validationWarnings := validateAppDetectionConfig(next.AppDetection)
	if len(validationErrors) > 0 {
		defer a.mu.Unlock()
		return a.resultLocked(false, "Profile settings are invalid", validationWarnings, validationErrors)
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
	profile := profileFromConfig(next, name)
	profile.ID = uniqueProfileID(next.Profiles, name)
	profile.LastUsed = time.Now().UTC().Format(time.RFC3339)
	next.Profiles = append(next.Profiles, profile)
	next.ActiveProfileID = profile.ID
	if err := a.store.Save(next); err != nil {
		if input.UpdateOBSPassword {
			_ = a.secretStore.SaveOBSPassword(oldPassword)
		}
		defer a.mu.Unlock()
		return a.resultLocked(false, "Profile save failed", nil, []string{err.Error()})
	}
	a.cfg = next
	a.lastAction = "Profile saved as " + profile.Name
	appDetection := a.cfg.AppDetection
	a.mu.Unlock()

	if a.detector != nil {
		a.detector.Start(appDetection)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.resultLocked(true, a.lastAction, validationWarnings, nil)
}

func (a *App) DuplicateProfile() appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Normalize()
	active, ok := a.cfg.ActiveProfile()
	if !ok {
		return a.resultLocked(false, "Active profile was not found", nil, []string{"Active profile was not found"})
	}
	active.Name = uniqueCopyName(active.Name, a.cfg.Profiles)
	active.ID = uniqueProfileID(a.cfg.Profiles, active.Name)
	active.LastUsed = time.Now().UTC().Format(time.RFC3339)
	a.cfg.Profiles = append(a.cfg.Profiles, active)
	a.cfg.ActiveProfileID = active.ID
	a.cfg.ApplyStreamProfile(active)
	if err := a.store.Save(a.cfg); err != nil {
		return a.resultLocked(false, "Profile duplicate failed", nil, []string{err.Error()})
	}
	a.lastAction = "Duplicated profile: " + active.Name
	return a.resultLocked(true, a.lastAction, nil, nil)
}

func (a *App) DeleteProfile(profileID string) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	if profileID == config.DefaultProfileID {
		return a.resultLocked(false, "Default profile cannot be deleted", nil, []string{"Default profile cannot be deleted"})
	}
	index := -1
	for i, profile := range a.cfg.Profiles {
		if profile.ID == profileID {
			index = i
			break
		}
	}
	if index < 0 {
		return a.resultLocked(false, "Profile was not found", nil, []string{"Profile was not found"})
	}
	deletedName := a.cfg.Profiles[index].Name
	activeDeleted := a.cfg.ActiveProfileID == profileID
	a.cfg.Profiles = append(a.cfg.Profiles[:index], a.cfg.Profiles[index+1:]...)
	if activeDeleted {
		a.cfg.ActiveProfileID = config.DefaultProfileID
		if profile, ok := a.cfg.ActiveProfile(); ok {
			a.cfg.ApplyStreamProfile(profile)
		}
	}
	a.cfg.Normalize()
	if activeDeleted {
		result := a.applyModeLocked(a.cfg.CurrentMode, applyModeOptions{
			applyTwitchChanges: true,
			source:             "profile",
			recordManualSwitch: true,
		})
		if result.OK {
			a.lastAction = "Deleted profile: " + deletedName
			result.Message = a.lastAction
			result.NewStatus = a.statusLocked()
		}
		return result
	}
	if err := a.store.Save(a.cfg); err != nil {
		return a.resultLocked(false, "Profile delete failed", nil, []string{err.Error()})
	}
	a.lastAction = "Deleted profile: " + deletedName
	return a.resultLocked(true, a.lastAction, nil, nil)
}

func (a *App) SelectProfile(profileID string) appdto.ActionResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Normalize()
	selected, ok := a.profileByIDLocked(profileID)
	if !ok {
		return a.resultLocked(false, "Profile was not found", nil, []string{"Profile was not found"})
	}
	return a.applyStreamProfileLocked(selected, applyModeOptions{
		applyTwitchChanges: true,
		source:             "profile",
		recordManualSwitch: true,
	})
}

func (a *App) applyStreamProfileLocked(selected config.Profile, options applyModeOptions) appdto.ActionResult {
	selected.LastUsed = time.Now().UTC().Format(time.RFC3339)
	for i := range a.cfg.Profiles {
		if a.cfg.Profiles[i].ID == selected.ID {
			a.cfg.Profiles[i] = selected
			break
		}
	}
	a.cfg.ApplyStreamProfile(selected)
	return a.applyModeLocked(selected.Mode, options)
}

func (a *App) profileByIDLocked(profileID string) (config.Profile, bool) {
	for _, profile := range a.cfg.Profiles {
		if profile.ID == profileID {
			return profile, true
		}
	}
	return config.Profile{}, false
}

func (a *App) profileForModeLocked(mode config.Mode) (config.Profile, bool) {
	var selected config.Profile
	found := false
	selectedTime := int64(-1)
	for _, profile := range a.cfg.Profiles {
		if profile.Mode != mode {
			continue
		}
		profileTime := int64(0)
		if parsed, err := time.Parse(time.RFC3339, profile.LastUsed); err == nil {
			profileTime = parsed.Unix()
		}
		if !found || profileTime > selectedTime || (profileTime == selectedTime && profile.ID == config.DefaultProfileID) {
			selected = profile
			selectedTime = profileTime
			found = true
		}
	}
	return selected, found
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
		RewardMappings:          append([]config.RewardMapping(nil), a.cfg.RewardMappings...),
		Profiles:                append([]config.Profile(nil), a.cfg.Profiles...),
		ActiveProfileID:         a.cfg.ActiveProfileID,
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
	next.RewardMappings = append([]config.RewardMapping(nil), settings.RewardMappings...)
	next.Profiles = append([]config.Profile(nil), settings.Profiles...)
	next.ActiveProfileID = settings.ActiveProfileID
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
		} else if options.source == "profile" {
			a.lastAction = fmt.Sprintf("Profile applied with errors")
		} else {
			a.lastAction = fmt.Sprintf("Switched to %s mode with errors", mode)
		}
		return a.resultLocked(false, a.lastAction, warnings, errors)
	case len(warnings) > 0:
		if options.source == "profile" {
			a.lastAction = "Profile applied with warnings"
		} else {
			a.lastAction = fmt.Sprintf("Auto-switched to %s mode with warnings", mode)
		}
		return a.resultLocked(true, a.lastAction, warnings, nil)
	default:
		if options.source == "auto" {
			a.lastAction = fmt.Sprintf("Auto-switched to %s Mode", mode)
		} else if options.source == "profile" {
			a.lastAction = "Profile applied"
		} else {
			a.lastAction = fmt.Sprintf("Switched to %s Mode successfully", mode)
		}
		return a.resultLocked(true, a.lastAction, nil, nil)
	}
}

func profileFromConfig(cfg config.Config, name string) config.Profile {
	return config.Profile{
		Name:           strings.TrimSpace(name),
		Mode:           cfg.CurrentMode,
		Sources:        cfg.Sources,
		SceneMappings:  append([]config.SceneMapping(nil), cfg.SceneMappings...),
		RewardMappings: append([]config.RewardMapping(nil), cfg.RewardMappings...),
	}
}

func profileNameExists(profiles []config.Profile, name string, exceptID string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, profile := range profiles {
		if profile.ID != exceptID && strings.ToLower(strings.TrimSpace(profile.Name)) == name {
			return true
		}
	}
	return false
}

func uniqueCopyName(base string, profiles []config.Profile) string {
	candidate := strings.TrimSpace(base) + " Copy"
	if !profileNameExists(profiles, candidate, "") {
		return candidate
	}
	for i := 2; ; i++ {
		next := candidate + " " + strconv.Itoa(i)
		if !profileNameExists(profiles, next, "") {
			return next
		}
	}
}

func uniqueProfileID(profiles []config.Profile, name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	var builder strings.Builder
	for _, char := range base {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case builder.Len() > 0:
			text := builder.String()
			if text[len(text)-1] != '-' {
				builder.WriteRune('-')
			}
		}
	}
	id := strings.Trim(builder.String(), "-")
	if id == "" {
		id = "profile"
	}
	exists := func(value string) bool {
		for _, profile := range profiles {
			if profile.ID == value {
				return true
			}
		}
		return false
	}
	if !exists(id) {
		return id
	}
	for i := 2; ; i++ {
		next := id + "-" + strconv.Itoa(i)
		if !exists(next) {
			return next
		}
	}
}
