package appdetect

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"

	"TuberSwitch/internal/config"
)

func TestEvaluatePrefersSingleDetectedApp(t *testing.T) {
	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	cfg.PNGProcessName = "veadotube-mini.exe"

	result := Evaluate(cfg, []string{"VNYAN.EXE"})
	if result.Status != StatusThreeDDetected || result.DesiredMode != config.Mode3D || !result.ShouldSwitch {
		t.Fatalf("unexpected 3D evaluation: %#v", result)
	}

	result = Evaluate(cfg, []string{"veadotube-mini.exe"})
	if result.Status != StatusPNGDetected || result.DesiredMode != config.ModePNG || !result.ShouldSwitch {
		t.Fatalf("unexpected PNG evaluation: %#v", result)
	}
}

func TestEvaluateHandlesConflictBehavior(t *testing.T) {
	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	cfg.PNGProcessName = "veadotube-mini.exe"
	names := []string{"vnyan.exe", "veadotube-mini.exe"}

	result := Evaluate(cfg, names)
	if result.Status != StatusBothDetected || result.ShouldSwitch {
		t.Fatalf("unexpected do-nothing evaluation: %#v", result)
	}

	cfg.ConflictBehavior = config.AppDetectionConflictPrefer3D
	result = Evaluate(cfg, names)
	if result.DesiredMode != config.Mode3D || !result.ShouldSwitch {
		t.Fatalf("unexpected prefer-3d evaluation: %#v", result)
	}

	cfg.ConflictBehavior = config.AppDetectionConflictPreferPNG
	result = Evaluate(cfg, names)
	if result.DesiredMode != config.ModePNG || !result.ShouldSwitch {
		t.Fatalf("unexpected prefer-png evaluation: %#v", result)
	}
}

func TestServiceRespectsManualCooldown(t *testing.T) {
	provider := &stubProcessProvider{names: []string{"vnyan.exe"}}
	switches := 0
	service := New(log.Default(), provider, func(mode config.Mode, applyTwitch bool) error {
		switches++
		if mode != config.Mode3D || !applyTwitch {
			t.Fatalf("unexpected apply: mode=%s applyTwitch=%t", mode, applyTwitch)
		}
		return nil
	}, func() config.Mode {
		return config.ModePNG
	})
	now := time.Unix(1_000, 0)
	service.now = func() time.Time { return now }

	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	service.RecordManualOverride(15 * time.Second)
	service.tick(cfg)
	if switches != 0 {
		t.Fatalf("switch executed during cooldown")
	}

	now = now.Add(16 * time.Second)
	service.tick(cfg)
	if switches != 1 {
		t.Fatalf("expected one switch after cooldown, got %d", switches)
	}
}

func TestServiceDoesNotApplyWhenDesiredModeAlreadyActive(t *testing.T) {
	provider := &stubProcessProvider{names: []string{"vnyan.exe"}}
	switches := 0
	service := New(log.Default(), provider, func(mode config.Mode, applyTwitch bool) error {
		switches++
		return nil
	}, func() config.Mode {
		return config.Mode3D
	})

	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	service.tick(cfg)
	if switches != 0 {
		t.Fatalf("unexpected switch when mode already active")
	}
}

func TestServiceLogsOnlyMatchedProcessNamesOnStateChange(t *testing.T) {
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)
	provider := &stubProcessProvider{names: []string{"obs64.exe", "vnyan.exe", "discord.exe"}}
	service := New(logger, provider, func(mode config.Mode, applyTwitch bool) error {
		return nil
	}, func() config.Mode {
		return config.ModePNG
	})

	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	service.tick(cfg)
	service.tick(cfg)

	output := logs.String()
	if strings.Contains(output, "obs64.exe") || strings.Contains(output, "discord.exe") {
		t.Fatalf("unexpected non-matching processes logged: %s", output)
	}
	if !strings.Contains(output, "matches=vnyan.exe") {
		t.Fatalf("expected matched process log, got: %s", output)
	}
	if strings.Count(output, "app detection observed:") != 1 {
		t.Fatalf("expected one observation log for unchanged state, got: %s", output)
	}
}

type stubProcessProvider struct {
	names []string
	err   error
}

func (s *stubProcessProvider) ListProcesses() ([]ProcessSummary, error) {
	processes := make([]ProcessSummary, 0, len(s.names))
	for index, name := range s.names {
		processes = append(processes, ProcessSummary{ProcessName: name, PID: index + 1})
	}
	return processes, s.err
}
