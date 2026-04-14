package daemon

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		// The canonical packets.go:58 signature from GH#663.
		{"packets.go:58: unexpected EOF", true},
		{"unexpected EOF", true},
		{"EOF", true},
		{"dial tcp 127.0.0.1:3307: connect: connection refused", true},
		{"write tcp 127.0.0.1:3307->127.0.0.1:54321: broken pipe", true},
		{"read tcp 127.0.0.1:3307: connection reset by peer", true},
		{"i/o timeout", true},
		{"invalid connection", true},
		{"driver: bad connection", true},
		{"dial tcp: lookup dolt.example.com: no such host", true},
		// Mixed case is fine — we lower-case internally.
		{"Unexpected EOF", true},

		// Non-transient — real problems that require intervention.
		{"", false},
		{"syntax error at position 42", false},
		{"access denied for user 'root'", false},
		{"unknown database 'nope'", false},
		{"table not found", false},

		// Read-only state must NOT be treated as transient: it is a persistent
		// condition that requires a server restart. Guards against the driver
		// surfacing read-only-ish errors with EOF flavor text.
		{"database is read only", false},
		{"cannot update manifest: database is read only", false},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			if got := isTransientError(tt.msg); got != tt.want {
				t.Errorf("isTransientError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

// TestCheckHealthWithRetry_TransientThenSuccess verifies the core GH#663 fix:
// a transient EOF that clears on retry must NOT be treated as unhealthy.
func TestCheckHealthWithRetry_TransientThenSuccess(t *testing.T) {
	var calls atomic.Int32
	m := newTestManager(t)
	m.healthCheckFn = func() error {
		n := calls.Add(1)
		if n == 1 {
			return errors.New("packets.go:58: unexpected EOF")
		}
		return nil
	}
	m.sleepFn = func(d time.Duration) {} // instant retries

	m.mu.Lock()
	err := m.checkHealthWithRetryLocked()
	m.mu.Unlock()

	if err != nil {
		t.Fatalf("expected nil error (retry should succeed), got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("expected 2 health check calls (1 fail + 1 recover), got %d", got)
	}
}

// TestCheckHealthWithRetry_FatalErrorReturnsImmediately verifies that a
// non-transient error skips retry and surfaces immediately so the caller can
// restart the server promptly.
func TestCheckHealthWithRetry_FatalErrorReturnsImmediately(t *testing.T) {
	var calls atomic.Int32
	m := newTestManager(t)
	m.healthCheckFn = func() error {
		calls.Add(1)
		return errors.New("access denied for user 'root'")
	}
	m.sleepFn = func(d time.Duration) {} // instant

	m.mu.Lock()
	err := m.checkHealthWithRetryLocked()
	m.mu.Unlock()

	if err == nil {
		t.Fatal("expected error for fatal condition")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected exactly 1 health check call (no retry on fatal), got %d", got)
	}
}

// TestCheckHealthWithRetry_PersistentTransientExhausts verifies that a server
// that keeps returning transient errors eventually gives up after
// healthCheckMaxAttempts and returns the last error, triggering a restart.
func TestCheckHealthWithRetry_PersistentTransientExhausts(t *testing.T) {
	var calls atomic.Int32
	m := newTestManager(t)
	m.healthCheckFn = func() error {
		calls.Add(1)
		return errors.New("connection refused")
	}
	m.sleepFn = func(d time.Duration) {} // instant

	m.mu.Lock()
	err := m.checkHealthWithRetryLocked()
	m.mu.Unlock()

	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := calls.Load(); got != int32(healthCheckMaxAttempts) {
		t.Errorf("expected %d health check calls, got %d", healthCheckMaxAttempts, got)
	}
}

// TestCheckHealthWithRetry_FirstCallSucceeds verifies the happy path: a
// healthy server is not retried and the retry logic adds zero overhead.
func TestCheckHealthWithRetry_FirstCallSucceeds(t *testing.T) {
	var calls atomic.Int32
	m := newTestManager(t)
	m.healthCheckFn = func() error {
		calls.Add(1)
		return nil
	}
	slept := false
	m.sleepFn = func(d time.Duration) { slept = true }

	m.mu.Lock()
	err := m.checkHealthWithRetryLocked()
	m.mu.Unlock()

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 health check call, got %d", got)
	}
	if slept {
		t.Error("did not expect sleep on successful first call")
	}
}

// TestEnsureRunning_TransientEOFDoesNotRestart verifies the end-to-end GH#663
// scenario: a running Dolt server emits one transient EOF during a health
// check, but the retry succeeds and NO restart is triggered.
//
// Before the fix, this same sequence would call stop+start and sever every
// agent's connection for ~65s.
func TestEnsureRunning_TransientEOFDoesNotRestart(t *testing.T) {
	var stopCount, startCount, healthCalls atomic.Int32

	m := newTestManager(t)
	m.runningFn = func() (int, bool) { return 1234, true }
	m.healthCheckFn = func() error {
		n := healthCalls.Add(1)
		if n == 1 {
			return fmt.Errorf("packets.go:58: unexpected EOF")
		}
		return nil
	}
	m.stopFn = func() { stopCount.Add(1) }
	m.startFn = func() error {
		startCount.Add(1)
		return nil
	}
	m.sleepFn = func(d time.Duration) {} // instant

	if err := m.EnsureRunning(); err != nil {
		t.Fatalf("EnsureRunning returned error: %v", err)
	}

	if got := stopCount.Load(); got != 0 {
		t.Errorf("expected 0 stops (transient EOF should be retried), got %d", got)
	}
	if got := startCount.Load(); got != 0 {
		t.Errorf("expected 0 starts (no restart on retryable error), got %d", got)
	}
	if got := healthCalls.Load(); got != 2 {
		t.Errorf("expected 2 health checks (1 fail + 1 recover), got %d", got)
	}
}

// TestEnsureRunning_PersistentEOFStillRestarts verifies that when retries are
// exhausted (the server really is down), the full restart flow still runs —
// the retry budget is a blip-absorber, not a way to mask a dead server.
func TestEnsureRunning_PersistentEOFStillRestarts(t *testing.T) {
	var stopCount, startCount atomic.Int32
	var running atomic.Bool
	running.Store(true)

	m := newTestManager(t)
	m.runningFn = func() (int, bool) {
		if running.Load() {
			return 1234, true
		}
		return 0, false
	}
	m.healthCheckFn = func() error { return fmt.Errorf("unexpected EOF") }
	m.stopFn = func() {
		stopCount.Add(1)
		running.Store(false)
	}
	m.startFn = func() error {
		startCount.Add(1)
		running.Store(true)
		return nil
	}
	m.sleepFn = func(d time.Duration) {} // instant

	if err := m.EnsureRunning(); err != nil {
		t.Fatalf("EnsureRunning returned error: %v", err)
	}

	if got := stopCount.Load(); got != 1 {
		t.Errorf("expected 1 stop after retries exhausted, got %d", got)
	}
	if got := startCount.Load(); got != 1 {
		t.Errorf("expected 1 start after retries exhausted, got %d", got)
	}
}
