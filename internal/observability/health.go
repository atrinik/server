package observability

import (
	"sync"
	"time"
)

// Health keeps readiness resources distinct from simulation liveness.
type Health struct {
	mu       sync.RWMutex
	ready    bool
	reason   string
	lastTick time.Time
}

// HealthSnapshot is immutable and safe for local diagnostics/export.
type HealthSnapshot struct {
	Ready       bool          `json:"ready"`
	ReadyReason string        `json:"ready_reason"`
	Live        bool          `json:"live"`
	LastTickAge time.Duration `json:"last_tick_age"`
}

// SetReady records whether startup resources and listeners are usable.
func (health *Health) SetReady(ready bool, reason string) {
	health.mu.Lock()
	defer health.mu.Unlock()
	health.ready = ready
	health.reason = truncate(reason, 160)
}

// Tick records forward progress from the simulation owner.
func (health *Health) Tick(now time.Time) {
	health.mu.Lock()
	defer health.mu.Unlock()
	health.lastTick = now
}

// Snapshot declares liveness only when a recent tick proves loop progress.
func (health *Health) Snapshot(now time.Time, stallThreshold time.Duration) HealthSnapshot {
	health.mu.RLock()
	defer health.mu.RUnlock()
	age := time.Duration(0)
	if !health.lastTick.IsZero() {
		age = now.Sub(health.lastTick)
	}
	return HealthSnapshot{
		Ready:       health.ready,
		ReadyReason: health.reason,
		Live:        !health.lastTick.IsZero() && age >= 0 && age <= stallThreshold,
		LastTickAge: age,
	}
}
