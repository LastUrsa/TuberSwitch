package main

import (
	"context"
	"testing"

	"TuberSwitch/internal/sip"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestParseLaunchConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want launchConfig
	}{
		{name: "standalone by default", want: launchConfig{Mode: sip.StandaloneMode}},
		{name: "service mode starts hidden", args: []string{"--service"}, want: launchConfig{Mode: sip.ServiceMode, ServiceMode: true, StartHidden: true}},
		{name: "show launches standalone", args: []string{"--show"}, want: launchConfig{Mode: sip.StandaloneMode, ShowRequested: true}},
		{name: "show wins over service", args: []string{"--service", "--show"}, want: launchConfig{Mode: sip.StandaloneMode, ServiceMode: true, ShowRequested: true}},
		{name: "flags are case insensitive", args: []string{"--SERVICE"}, want: launchConfig{Mode: sip.ServiceMode, ServiceMode: true, StartHidden: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseLaunchConfig(test.args); got != test.want {
				t.Fatalf("parseLaunchConfig() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestShouldActivateExistingInstance(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "standalone duplicate activates", want: true},
		{name: "show duplicate activates", args: []string{"--show"}, want: true},
		{name: "service duplicate is blocked silently", args: []string{"--service"}, want: false},
		{name: "service show duplicate activates", args: []string{"--service", "--show"}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldActivateExistingInstance(test.args); got != test.want {
				t.Fatalf("shouldActivateExistingInstance() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestBuildWailsOptionsConfiguresServiceModeAndSingleInstance(t *testing.T) {
	got := buildWailsOptions(&App{}, parseLaunchConfig([]string{"--service"}))

	if !got.StartHidden {
		t.Fatalf("expected service mode to start hidden")
	}
	if got.SingleInstanceLock == nil {
		t.Fatalf("expected single instance lock")
	}
	if got.SingleInstanceLock.UniqueId != productionSingleInstanceID {
		t.Fatalf("single instance id = %q", got.SingleInstanceLock.UniqueId)
	}
}

func TestSecondInstanceLaunchActivatesExistingWindowWhenRequested(t *testing.T) {
	app := &App{ctx: context.Background()}
	activationCount := 0
	app.windowActivator = func(ctx context.Context) {
		if ctx == nil {
			t.Fatalf("expected app context")
		}
		activationCount++
	}

	optionsApp := buildWailsOptions(app, parseLaunchConfig(nil))
	optionsApp.SingleInstanceLock.OnSecondInstanceLaunch(options.SecondInstanceData{Args: []string{"--show"}})
	optionsApp.SingleInstanceLock.OnSecondInstanceLaunch(options.SecondInstanceData{Args: []string{"--service"}})
	optionsApp.SingleInstanceLock.OnSecondInstanceLaunch(options.SecondInstanceData{})

	if activationCount != 2 {
		t.Fatalf("activation count = %d, want 2", activationCount)
	}
}
