package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"TuberSwitch/internal/config"
)

const (
	obsStateNotConfigured = "not_configured"
	obsStateDisconnected  = "disconnected"
	obsStateReconnecting  = "reconnecting"
	obsStateConnected     = "connected"
)

type obsReconnectManager struct {
	obs         obsService
	config      func() config.OBSConfig
	baseDelay   time.Duration
	maxDelay    time.Duration
	healthEvery time.Duration
	onConnected func()

	attemptMu sync.Mutex
	mu        sync.RWMutex
	state     string
	wake      chan struct{}
	cancel    context.CancelFunc
	done      chan struct{}
}

func newOBSReconnectManager(obs obsService, configFn func() config.OBSConfig) *obsReconnectManager {
	return &obsReconnectManager{
		obs: obs, config: configFn,
		baseDelay: time.Second, maxDelay: 30 * time.Second, healthEvery: 3 * time.Second,
		state: obsStateDisconnected, wake: make(chan struct{}, 1),
	}
}

func (m *obsReconnectManager) Start(parent context.Context) {
	if m == nil || m.obs == nil || m.config == nil {
		return
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.done = make(chan struct{})
	done := m.done
	m.mu.Unlock()
	go func() {
		defer close(done)
		m.run(ctx)
	}()
}

func (m *obsReconnectManager) Stop() {
	if m == nil {
		return
	}
	m.mu.Lock()
	cancel, done := m.cancel, m.done
	m.cancel, m.done = nil, nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	m.attemptMu.Lock()
	if m.obs != nil {
		m.obs.Close()
	}
	m.attemptMu.Unlock()
	m.setState(obsStateDisconnected)
}

func (m *obsReconnectManager) Wake() {
	if m == nil {
		return
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *obsReconnectManager) State() string {
	if m == nil {
		return obsStateDisconnected
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *obsReconnectManager) ConnectNow(cfg config.OBSConfig, reset bool) error {
	if m == nil || m.obs == nil {
		return fmt.Errorf("OBS service unavailable")
	}
	if !obsConfigured(cfg) {
		m.setState(obsStateNotConfigured)
		return fmt.Errorf("OBS is not configured")
	}
	m.attemptMu.Lock()
	defer m.attemptMu.Unlock()
	if reset {
		m.obs.Close()
	}
	if m.obs.Connected() {
		m.setState(obsStateConnected)
		return nil
	}
	m.setState(obsStateReconnecting)
	if err := m.obs.Connect(cfg); err != nil {
		m.setState(obsStateDisconnected)
		m.Wake()
		return err
	}
	m.setState(obsStateConnected)
	m.Wake()
	return nil
}

func (m *obsReconnectManager) run(ctx context.Context) {
	delay := m.baseDelay
	for {
		cfg := m.config()
		if !obsConfigured(cfg) {
			m.setState(obsStateNotConfigured)
			delay = m.baseDelay
			if !m.wait(ctx, m.maxDelay) {
				return
			}
			continue
		}
		if m.obs.Connected() {
			m.setState(obsStateConnected)
			delay = m.baseDelay
			if !m.wait(ctx, m.healthEvery) {
				return
			}
			if err := m.obs.CheckConnection(); err != nil {
				m.setState(obsStateReconnecting)
			}
			continue
		}
		m.setState(obsStateReconnecting)
		m.attemptMu.Lock()
		err := error(nil)
		if !m.obs.Connected() {
			err = m.obs.Connect(cfg)
		}
		m.attemptMu.Unlock()
		if err == nil && m.obs.Connected() {
			m.setState(obsStateConnected)
			delay = m.baseDelay
			if m.onConnected != nil {
				m.onConnected()
			}
			continue
		}
		m.setState(obsStateReconnecting)
		if !m.wait(ctx, delay) {
			return
		}
		delay *= 2
		if delay > m.maxDelay {
			delay = m.maxDelay
		}
	}
}

func (m *obsReconnectManager) wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-m.wake:
		return true
	case <-timer.C:
		return true
	}
}

func (m *obsReconnectManager) setState(state string) {
	m.mu.Lock()
	m.state = state
	m.mu.Unlock()
}

func obsConfigured(cfg config.OBSConfig) bool {
	return strings.TrimSpace(cfg.Host) != "" && cfg.Port > 0 && cfg.Port <= 65535
}
