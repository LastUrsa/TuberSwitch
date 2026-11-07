package appdetect

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"TuberSwitch/internal/config"
)

type ApplyModeFunc func(mode config.Mode, applyTwitch bool) error
type CurrentModeFunc func() config.Mode

type Service struct {
	logger        *log.Logger
	provider      ProcessProvider
	applyMode     ApplyModeFunc
	currentMode   CurrentModeFunc
	now           func() time.Time
	resolveTarget ResolveTargetFunc
	lastFailure   string
	lastDecision  string

	mu                  sync.Mutex
	snapshot            Snapshot
	manualOverrideUntil time.Time
	lastLogState        logState
	cancel              context.CancelFunc
	done                chan struct{}
}

type logState struct {
	status       string
	desiredMode  config.Mode
	shouldSwitch bool
	matches      []string
}

func New(logger *log.Logger, provider ProcessProvider, applyMode ApplyModeFunc, currentMode CurrentModeFunc, resolvers ...ResolveTargetFunc) *Service {
	var resolver ResolveTargetFunc
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	return &Service{
		resolveTarget: resolver,
		logger:        logger,
		provider:      provider,
		applyMode:     applyMode,
		currentMode:   currentMode,
		now:           time.Now,
		snapshot:      Snapshot{Status: StatusDisabled, Diagnostics: Diagnostics{Reason: "disabled"}},
	}
}

func (s *Service) Start(cfg config.AppDetectionConfig) {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()

	stopLoop(cancel, done)

	s.mu.Lock()
	defer s.mu.Unlock()
	if !cfg.Enabled {
		s.snapshot = Snapshot{Status: StatusDisabled, Diagnostics: Diagnostics{Reason: "disabled"}}
		s.lastLogState = logState{}
		return
	}
	s.lastFailure, s.lastDecision = "", ""
	s.snapshot = Snapshot{Status: StatusEnabled, ApplyTwitch: cfg.ApplyTwitchChanges, Diagnostics: Diagnostics{Reason: "waiting"}}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	go s.loop(ctx, cfg, s.done)
}

func (s *Service) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel = nil
	s.done = nil
	s.snapshot = Snapshot{Status: StatusDisabled, Diagnostics: Diagnostics{Reason: "disabled"}}
	s.lastLogState = logState{}
	s.mu.Unlock()
	stopLoop(cancel, done)
}

func (s *Service) RecordManualOverride(cooldown time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.manualOverrideUntil = s.now().Add(cooldown)
	s.lastFailure = ""
}

func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneSnapshot(s.snapshot)
}

func (s *Service) loop(ctx context.Context, cfg config.AppDetectionConfig, done chan struct{}) {
	defer close(done)
	s.tick(cfg)
	ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(cfg)
		}
	}
}

func (s *Service) tick(cfg config.AppDetectionConfig) {
	processes, err := s.provider.ListProcesses()
	if err != nil {
		s.decision(Diagnostics{Reason: "process_unavailable", ErrorCode: "UNAVAILABLE", ObservedAt: s.now().UTC().Format(time.RFC3339Nano)})
		s.updateSnapshot(func(snapshot *Snapshot) {
			snapshot.Status = StatusEnabled
			snapshot.DetectedNames = nil
			snapshot.ShouldSwitch = false
			snapshot.DesiredMode = ""
			snapshot.ApplyTwitch = cfg.ApplyTwitchChanges
		})
		return
	}
	names := ProcessNames(processes)

	evaluation := Evaluate(cfg, names)
	diagnostics := Diagnostics{DesiredMode: evaluation.DesiredMode, ObservedAt: s.now().UTC().Format(time.RFC3339Nano), ThreeDRunning: containsProcessName(names, cfg.ThreeDProcessName), PNGRunning: containsProcessName(names, cfg.PNGProcessName)}
	matches := matchedProcessNames(names, cfg)
	s.updateSnapshot(func(snapshot *Snapshot) {
		snapshot.Status = evaluation.Status
		snapshot.ThreeDRunning = containsProcessName(names, cfg.ThreeDProcessName)
		snapshot.PNGRunning = containsProcessName(names, cfg.PNGProcessName)
		snapshot.DetectedNames = append([]string(nil), names...)
		snapshot.DesiredMode = evaluation.DesiredMode
		snapshot.ShouldSwitch = evaluation.ShouldSwitch
		snapshot.ApplyTwitch = cfg.ApplyTwitchChanges
		snapshot.ConflictResolved = snapshot.ThreeDRunning && snapshot.PNGRunning && evaluation.ShouldSwitch
	})

	s.logObservationIfChanged(logState{
		status:       evaluation.Status,
		desiredMode:  evaluation.DesiredMode,
		shouldSwitch: evaluation.ShouldSwitch,
		matches:      matches,
	})
	if !evaluation.ShouldSwitch {
		diagnostics.Reason = "no_app"
		if diagnostics.ThreeDRunning && diagnostics.PNGRunning {
			diagnostics.Reason = "conflict"
		}
		s.mu.Lock()
		s.lastFailure = ""
		s.mu.Unlock()
		s.decision(diagnostics)
		return
	}
	target := Target{}
	if s.resolveTarget != nil {
		target, err = s.resolveTarget(evaluation.DesiredMode)
	} else {
		target.Active = s.currentMode() == evaluation.DesiredMode
	}
	diagnostics.TargetProfileID = target.ProfileID
	if err != nil {
		diagnostics.Reason = "target_missing"
		if errors.Is(err, ErrTargetModeMismatch) {
			diagnostics.Reason = "target_mode_mismatch"
		}
		diagnostics.ErrorCode = "INVALID_REQUEST"
		s.decision(diagnostics)
		return
	}
	if s.inManualCooldown() {
		diagnostics.Reason = "manual_cooldown"
		diagnostics.CooldownUntil = s.manualOverrideDeadline().UTC().Format(time.RFC3339Nano)
		s.decision(diagnostics)
		return
	}
	failureKey := string(evaluation.DesiredMode) + ":" + target.ProfileID
	if target.Active {
		diagnostics.Reason = "already_active"
		s.mu.Lock()
		failed := s.lastFailure == failureKey
		s.mu.Unlock()
		if failed {
			diagnostics.Reason = "integration_failed"
			diagnostics.ErrorCode = "UNAVAILABLE"
		}
		s.decision(diagnostics)
		return
	}
	if err := s.applyMode(evaluation.DesiredMode, cfg.ApplyTwitchChanges); err != nil {
		s.mu.Lock()
		s.lastFailure = failureKey
		s.mu.Unlock()
		diagnostics.Reason = "integration_failed"
		diagnostics.ErrorCode = "UNAVAILABLE"
		s.decision(diagnostics)
		return
	}
	s.mu.Lock()
	s.lastFailure = ""
	s.mu.Unlock()
	diagnostics.Reason = "applied"
	s.decision(diagnostics)
}

func (s *Service) updateSnapshot(apply func(snapshot *Snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneSnapshot(s.snapshot)
	apply(&next)
	s.snapshot = next
}

func (s *Service) inManualCooldown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now().Before(s.manualOverrideUntil)
}

func (s *Service) manualOverrideDeadline() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.manualOverrideUntil
}

func stopLoop(cancel context.CancelFunc, done chan struct{}) {
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.DetectedNames = append([]string(nil), snapshot.DetectedNames...)
	return snapshot
}

func (s *Service) logObservationIfChanged(state logState) {
	s.mu.Lock()
	if state.status == s.lastLogState.status &&
		state.desiredMode == s.lastLogState.desiredMode &&
		state.shouldSwitch == s.lastLogState.shouldSwitch &&
		slices.Equal(state.matches, s.lastLogState.matches) {
		s.mu.Unlock()
		return
	}
	s.lastLogState = logState{
		status:       state.status,
		desiredMode:  state.desiredMode,
		shouldSwitch: state.shouldSwitch,
		matches:      append([]string(nil), state.matches...),
	}
	s.mu.Unlock()

	matchText := "none"
	if len(state.matches) > 0 {
		matchText = strings.Join(state.matches, ",")
	}
	s.logger.Printf("app detection observed: matches=%s status=%s desired_mode=%s should_switch=%t", matchText, state.status, state.desiredMode, state.shouldSwitch)
}

// decision logs transitions, not every polling heartbeat. IDs are hashed in logs.
func (s *Service) decision(d Diagnostics) {
	key := string(d.DesiredMode) + ":" + d.TargetProfileID + ":" + d.Reason + ":" + d.ErrorCode
	s.mu.Lock()
	changed := s.lastDecision != key
	s.lastDecision = key
	s.snapshot.Diagnostics = d
	s.mu.Unlock()
	if changed {
		ref := "none"
		if d.TargetProfileID != "" {
			sum := sha256.Sum256([]byte(d.TargetProfileID))
			ref = fmt.Sprintf("%x", sum[:12])
		}
		s.logger.Printf("app detection decision: mode=%s target_ref=%s reason=%s code=%s", d.DesiredMode, ref, d.Reason, d.ErrorCode)
	}
}
