package daemon

import (
	"strings"
	"time"
)

// Health-check retry tuning for transient Dolt errors.
//
// Under concurrent load Dolt occasionally returns a single "packets.go:58
// unexpected EOF" or a momentary "connection refused" that clears within
// milliseconds. Before GH#663, any such blip in the 30s health-check tick
// triggered a full server stop+restart — severing every agent's connection
// for ~65s. Retrying the probe a few times absorbs the blip.
const (
	healthCheckMaxAttempts   = 3
	healthCheckRetryBase     = 100 * time.Millisecond
	healthCheckRetryMaxDelay = 1 * time.Second
)

// transientErrorSubstrings are lower-cased substrings that mark a Dolt error
// as transient. A transient error is retryable — not a reason to restart.
//
// "unexpected eof" / "eof":   the packets.go:58 signature under concurrent load.
// "broken pipe":              the driver's peer closed the socket mid-read.
// "connection reset":         TCP RST, usually Dolt briefly paused.
// "connection refused":       the listener is momentarily gone (restart window).
// "i/o timeout":              slow query or stalled socket.
// "bad connection"/"invalid connection": go-sql-driver's retry signals.
// "no such host":             transient DNS blip for remote hosts.
var transientErrorSubstrings = []string{
	"unexpected eof",
	"eof",
	"broken pipe",
	"connection reset",
	"connection refused",
	"i/o timeout",
	"bad connection",
	"invalid connection",
	"no such host",
}

// isTransientError reports whether msg looks like a transient Dolt/driver
// failure that should be retried before declaring the server unhealthy.
func isTransientError(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	// Read-only state is a persistent condition, not transient — even though
	// the driver may surface it with an EOF-flavored message. Handled
	// separately by checkWriteHealthLocked.
	if isReadOnlyError(lower) {
		return false
	}
	for _, p := range transientErrorSubstrings {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// checkHealthWithRetryLocked runs checkHealthLocked, retrying on transient
// errors before declaring the server unhealthy. Must be called with m.mu
// held. The mutex is released during retry sleeps so other callers can make
// progress (mirrors the pattern in restartWithBackoff).
//
// Returns nil on success, the first non-transient error immediately, or the
// last transient error after exhausting retries.
func (m *DoltServerManager) checkHealthWithRetryLocked() error {
	delay := healthCheckRetryBase
	var lastErr error
	for attempt := 1; attempt <= healthCheckMaxAttempts; attempt++ {
		err := m.checkHealthLocked()
		if err == nil {
			if attempt > 1 {
				m.logger("Dolt health check recovered on attempt %d/%d", attempt, healthCheckMaxAttempts)
			}
			return nil
		}
		lastErr = err
		if !isTransientError(err.Error()) {
			return err
		}
		if attempt == healthCheckMaxAttempts {
			break
		}
		m.logger("Dolt health check transient failure (attempt %d/%d): %v — retrying in %v",
			attempt, healthCheckMaxAttempts, err, delay)
		// Release the lock during sleep so concurrent EnsureRunning callers
		// can proceed through the restarting-check fast-path.
		m.mu.Unlock()
		m.doSleep(delay)
		m.mu.Lock()

		delay *= 2
		if delay > healthCheckRetryMaxDelay {
			delay = healthCheckRetryMaxDelay
		}
	}
	return lastErr
}
