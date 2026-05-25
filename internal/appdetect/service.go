package appdetect

import (
	"context"
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
	logger      *log.Logger
	provider    ProcessProvider
	applyMode   ApplyModeFunc
	currentMode CurrentModeFunc
	now         func() time.Time

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

func New(logger *log.Logger, provider ProcessProvider, applyMode ApplyModeFunc, currentMode CurrentModeFunc) *Service {
	return &Service{
		logger:      logger,
		provider:    provider,
		applyMode:   applyMode,
		currentMode: currentMode,
		now:         time.Now,
		snapshot:    Snapshot{Status: StatusDisabled},
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
		s.snapshot = Snapshot{Status: StatusDisabled}
		s.lastLogState = logState{}
		return
	}
	s.snapshot = Snapshot{Status: StatusEnabled, ApplyTwitch: cfg.ApplyTwitchChanges}
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
	s.snapshot = Snapshot{Status: StatusDisabled}
	s.lastLogState = logState{}
	s.mu.Unlock()
	stopLoop(cancel, done)
}

func (s *Service) RecordManualOverride(cooldown time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.manualOverrideUntil = s.now().Add(cooldown)
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
		s.logger.Printf("app detection process enumeration failed: %v", err)
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
		return
	}
	if s.currentMode() == evaluation.DesiredMode {
		return
	}
	if s.inManualCooldown() {
		s.logger.Printf("app detection suppressed by manual cooldown until %s", s.manualOverrideDeadline().Format(time.RFC3339))
		return
	}
	if err := s.applyMode(evaluation.DesiredMode, cfg.ApplyTwitchChanges); err != nil {
		s.logger.Printf("app detection switch failed: mode=%s err=%v", evaluation.DesiredMode, err)
		return
	}
	s.logger.Printf("app detection switched mode=%s", evaluation.DesiredMode)
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
