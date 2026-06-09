package main

import (
	"context"
	"fmt"
	"strings"

	"TuberSwitch/internal/sip"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const productionSingleInstanceID = "com.starsong.tuberswitch"

type launchConfig struct {
	Mode          string
	ServiceMode   bool
	ShowRequested bool
	StartHidden   bool
}

func parseLaunchConfig(args []string) launchConfig {
	config := launchConfig{Mode: sip.StandaloneMode}
	for _, arg := range args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "--service":
			config.ServiceMode = true
		case "--show":
			config.ShowRequested = true
		}
	}
	if config.ServiceMode && !config.ShowRequested {
		config.Mode = sip.ServiceMode
		config.StartHidden = true
	}
	return config
}

func singleInstanceID() string {
	return productionSingleInstanceID
}

func shouldActivateExistingInstance(args []string) bool {
	config := parseLaunchConfig(args)
	return !config.ServiceMode || config.ShowRequested
}

func runtimeModeDisplayName(mode string) string {
	if mode == sip.ServiceMode {
		return "Service"
	}
	return "Standalone"
}

func startupModeMessage(mode string) string {
	return fmt.Sprintf("Starting TuberSwitch in %s Mode", runtimeModeDisplayName(mode))
}

func defaultWindowActivator(ctx context.Context) {
	if ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(ctx)
	wailsruntime.WindowShow(ctx)
	wailsruntime.Show(ctx)
}
