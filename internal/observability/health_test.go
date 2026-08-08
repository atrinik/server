package observability

import (
	"testing"
	"time"
)

func TestReadinessAndLivenessHaveSeparateSemantics(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_000, 0)
	var health Health
	health.SetReady(true, "resources and listeners ready")
	if snapshot := health.Snapshot(now, time.Second); !snapshot.Ready || snapshot.Live {
		t.Fatal("readiness incorrectly implied liveness")
	}
	health.Tick(now)
	if snapshot := health.Snapshot(now.Add(500*time.Millisecond), time.Second); !snapshot.Ready || !snapshot.Live {
		t.Fatal("recent tick was not live")
	}
	if snapshot := health.Snapshot(now.Add(2*time.Second), time.Second); snapshot.Live {
		t.Fatal("stalled tick remained live")
	}
	if snapshot := health.Snapshot(now.Add(-time.Second), time.Second); snapshot.Live {
		t.Fatal("backward wall-clock jump reported liveness")
	}
}
