package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"TuberSwitch/internal/config"
	"TuberSwitch/internal/obs"
)

type reconnectFakeOBS struct {
	mu           sync.Mutex
	available    bool
	connected    bool
	failures     int
	drop         bool
	connectDelay time.Duration
	attempts     []time.Time
	active       int
	maxActive    int
}

func (f *reconnectFakeOBS) Connected() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.connected }
func (f *reconnectFakeOBS) Close()          { f.mu.Lock(); f.connected = false; f.mu.Unlock() }
func (f *reconnectFakeOBS) Connect(config.OBSConfig) error {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.attempts = append(f.attempts, time.Now())
	delay := f.connectDelay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active--
	if !f.available || f.failures > 0 {
		if f.failures > 0 {
			f.failures--
		}
		return fmt.Errorf("connection unavailable")
	}
	f.connected = true
	return nil
}
func (f *reconnectFakeOBS) CheckConnection() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.drop {
		f.drop, f.connected = false, false
		return fmt.Errorf("connection dropped")
	}
	if !f.connected {
		return fmt.Errorf("disconnected")
	}
	return nil
}
func (f *reconnectFakeOBS) GetScenes() ([]obs.Scene, error)                     { return nil, nil }
func (f *reconnectFakeOBS) GetSources(string) ([]obs.Source, error)             { return nil, nil }
func (f *reconnectFakeOBS) FindSceneItemID(string, string) (int, error)         { return 0, nil }
func (f *reconnectFakeOBS) SetSourceVisibility(string, string, int, bool) error { return nil }

func testReconnectManager(fake *reconnectFakeOBS, cfg config.OBSConfig) *obsReconnectManager {
	m := newOBSReconnectManager(fake, func() config.OBSConfig { return cfg })
	m.baseDelay, m.maxDelay, m.healthEvery = 5*time.Millisecond, 20*time.Millisecond, 5*time.Millisecond
	return m
}

func waitForOBS(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for OBS condition")
}

func TestOBSReconnectStartsDisconnectedThenConnects(t *testing.T) {
	fake := &reconnectFakeOBS{}
	m := testReconnectManager(fake, config.OBSConfig{Host: "127.0.0.1", Port: 4455, Password: "secret"})
	m.Start(context.Background())
	defer m.Stop()
	waitForOBS(t, 100*time.Millisecond, func() bool { fake.mu.Lock(); defer fake.mu.Unlock(); return len(fake.attempts) >= 2 })
	fake.mu.Lock()
	fake.available = true
	fake.mu.Unlock()
	m.Wake()
	waitForOBS(t, 100*time.Millisecond, fake.Connected)
	if m.State() != obsStateConnected {
		t.Fatalf("state = %q", m.State())
	}
}

func TestOBSReconnectBackoffIncreasesAndResetsAfterSuccess(t *testing.T) {
	fake := &reconnectFakeOBS{available: true, failures: 2}
	m := testReconnectManager(fake, config.OBSConfig{Host: "127.0.0.1", Port: 4455})
	m.Start(context.Background())
	defer m.Stop()
	waitForOBS(t, 150*time.Millisecond, fake.Connected)
	fake.mu.Lock()
	first := append([]time.Time(nil), fake.attempts...)
	fake.failures, fake.drop = 1, true
	fake.mu.Unlock()
	waitForOBS(t, 100*time.Millisecond, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.attempts) >= len(first)+2 && fake.connected
	})
	fake.mu.Lock()
	all := append([]time.Time(nil), fake.attempts...)
	fake.mu.Unlock()
	if first[2].Sub(first[1]) < 8*time.Millisecond {
		t.Fatalf("backoff did not increase: %v", first)
	}
	resetGap := all[len(first)+1].Sub(all[len(first)])
	if resetGap > 15*time.Millisecond {
		t.Fatalf("backoff did not reset: %v", resetGap)
	}
}

func TestOBSReconnectSerializesConnectionAttempts(t *testing.T) {
	fake := &reconnectFakeOBS{available: true, connectDelay: 20 * time.Millisecond}
	cfg := config.OBSConfig{Host: "127.0.0.1", Port: 4455}
	m := testReconnectManager(fake, cfg)
	m.Start(context.Background())
	done := make(chan struct{})
	go func() { _ = m.ConnectNow(cfg, true); close(done) }()
	<-done
	m.Stop()
	fake.mu.Lock()
	maxActive := fake.maxActive
	fake.mu.Unlock()
	if maxActive != 1 {
		t.Fatalf("overlapping attempts = %d", maxActive)
	}
}

func TestOBSReconnectDetectsDroppedConnection(t *testing.T) {
	fake := &reconnectFakeOBS{available: true}
	m := testReconnectManager(fake, config.OBSConfig{Host: "127.0.0.1", Port: 4455})
	m.Start(context.Background())
	defer m.Stop()
	waitForOBS(t, 100*time.Millisecond, fake.Connected)
	fake.mu.Lock()
	attempts := len(fake.attempts)
	fake.drop = true
	fake.mu.Unlock()
	waitForOBS(t, 100*time.Millisecond, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.attempts) > attempts && fake.connected
	})
}

func TestOBSReconnectInvalidConfigurationAndShutdown(t *testing.T) {
	fake := &reconnectFakeOBS{}
	m := testReconnectManager(fake, config.OBSConfig{Host: "", Port: 0, Password: "never-log-this"})
	m.Start(context.Background())
	time.Sleep(30 * time.Millisecond)
	if m.State() != obsStateNotConfigured {
		t.Fatalf("state = %q", m.State())
	}
	fake.mu.Lock()
	attempts := len(fake.attempts)
	fake.mu.Unlock()
	if attempts != 0 {
		t.Fatalf("invalid config attempts = %d", attempts)
	}
	start := time.Now()
	m.Stop()
	if time.Since(start) > 30*time.Millisecond {
		t.Fatalf("shutdown did not cancel pending retry")
	}
	if fake.Connected() {
		t.Fatal("OBS remained connected after shutdown")
	}
}
