package appdetect

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"TuberSwitch/internal/config"
)

func TestExplicitTargetsSwitchProfilesWithinSameModeAndOnAppTransition(t *testing.T) {
	provider := &stubProcessProvider{names: []string{"vnyan.exe"}}
	active := "default"
	mode := config.Mode3D
	calls := 0
	targets := map[config.Mode]string{config.Mode3D: "avatar", config.ModePNG: "png"}
	s := New(log.Default(), provider, func(m config.Mode, _ bool) error { calls++; active = targets[m]; mode = m; return nil }, func() config.Mode { return mode }, func(m config.Mode) (Target, error) { return Target{targets[m], active == targets[m] && mode == m}, nil })
	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	cfg.PNGProcessName = "veadotube-mini.exe"
	s.tick(cfg)
	if active != "avatar" || calls != 1 || s.Snapshot().Diagnostics.Reason != "applied" {
		t.Fatalf("first tick: active=%s calls=%d diagnostics=%+v", active, calls, s.Snapshot().Diagnostics)
	}
	s.tick(cfg)
	if calls != 1 || s.Snapshot().Diagnostics.Reason != "already_active" {
		t.Fatal("already applied target was reapplied")
	}
	provider.names = []string{"veadotube-mini.exe"}
	s.tick(cfg)
	if active != "png" || calls != 2 {
		t.Fatalf("transition: active=%s calls=%d", active, calls)
	}
	s.tick(cfg)
	if calls != 2 {
		t.Fatal("PNG target was reapplied")
	}
}

func TestExplicitTargetBlocksWithSafeDiagnostic(t *testing.T) {
	for _, test := range []struct {
		err    error
		reason string
	}{{ErrTargetMissing, "target_missing"}, {ErrTargetModeMismatch, "target_mode_mismatch"}} {
		t.Run(test.reason, func(t *testing.T) {
			calls := 0
			s := New(log.Default(), &stubProcessProvider{names: []string{"vnyan.exe"}}, func(config.Mode, bool) error { calls++; return nil }, func() config.Mode { return config.Mode3D }, func(config.Mode) (Target, error) { return Target{ProfileID: "target"}, test.err })
			cfg := config.DefaultAppDetection()
			cfg.ThreeDProcessName = "vnyan.exe"
			s.tick(cfg)
			d := s.Snapshot().Diagnostics
			if calls != 0 || d.Reason != test.reason || d.TargetProfileID != "target" || d.ErrorCode != "INVALID_REQUEST" {
				t.Fatalf("calls=%d diagnostics=%+v", calls, d)
			}
		})
	}
}

func TestExplicitTargetCooldownAndConflictPreventActivation(t *testing.T) {
	calls := 0
	active := false
	now := time.Unix(1000, 0)
	provider := &stubProcessProvider{names: []string{"vnyan.exe"}}
	s := New(log.Default(), provider, func(config.Mode, bool) error { calls++; active = true; return nil }, func() config.Mode { return config.Mode3D }, func(config.Mode) (Target, error) { return Target{"avatar", active}, nil })
	s.now = func() time.Time { return now }
	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	cfg.PNGProcessName = "png.exe"
	s.RecordManualOverride(15 * time.Second)
	s.tick(cfg)
	if calls != 0 || s.Snapshot().Diagnostics.Reason != "manual_cooldown" || s.Snapshot().Diagnostics.CooldownUntil == "" {
		t.Fatal("manual cooldown ignored")
	}
	now = now.Add(16 * time.Second)
	provider.names = append(provider.names, "png.exe")
	s.tick(cfg)
	if calls != 0 || s.Snapshot().Diagnostics.Reason != "conflict" {
		t.Fatal("conflict ignored")
	}
	cfg.ConflictBehavior = config.AppDetectionConflictPrefer3D
	s.tick(cfg)
	if calls != 1 {
		t.Fatal("configured conflict preference ignored")
	}
	provider.names = nil
	s.tick(cfg)
	if s.Snapshot().Diagnostics.Reason != "no_app" {
		t.Fatal("missing no-app observation")
	}
}

func TestFailedIntegrationRemainsVisibleWithoutReapplyingActiveTarget(t *testing.T) {
	var output bytes.Buffer
	active := false
	calls := 0
	s := New(log.New(&output, "", 0), &stubProcessProvider{names: []string{"vnyan.exe"}}, func(config.Mode, bool) error { calls++; active = true; return errors.New("private-token-and-path") }, func() config.Mode { return config.Mode3D }, func(config.Mode) (Target, error) { return Target{"private-profile-id", active}, nil })
	cfg := config.DefaultAppDetection()
	cfg.ThreeDProcessName = "vnyan.exe"
	s.tick(cfg)
	s.tick(cfg)
	if calls != 1 || s.Snapshot().Diagnostics.Reason != "integration_failed" {
		t.Fatal("failure was erased or action repeated")
	}
	if strings.Contains(output.String(), "private-") {
		t.Fatalf("private metadata in log: %s", output.String())
	}
	if strings.Count(output.String(), "app detection decision:") != 1 {
		t.Fatal("unchanged decisions must not flood logs")
	}
}

func TestProcessFailureDiagnosticDoesNotExposeRawError(t *testing.T) {
	var output bytes.Buffer
	s := New(log.New(&output, "", 0), &stubProcessProvider{err: errors.New("secret")}, nil, nil)
	s.tick(config.DefaultAppDetection())
	if s.Snapshot().Diagnostics.Reason != "process_unavailable" || strings.Contains(output.String(), "secret") {
		t.Fatal("unsafe process error diagnostic")
	}
}
