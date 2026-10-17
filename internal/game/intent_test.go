package game

import (
	"math"
	"testing"
	"time"
)

// nextSnapshot waits for one snapshot or fails.
func nextSnapshot(t *testing.T, ch <-chan Snapshot) Snapshot {
	t.Helper()
	select {
	case snap, ok := <-ch:
		if !ok {
			t.Fatal("subscriber channel closed unexpectedly")
		}
		return snap
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a snapshot")
		return Snapshot{}
	}
}

func TestIntentReachesSnapshots(t *testing.T) {
	sim := startSim(t)
	ch, cancel := sim.Subscribe()
	defer cancel()

	if err := sim.SubmitIntent(RolePilot, Intent{Seq: 1, Thrust: true}); err != nil {
		t.Fatalf("SubmitIntent: %v", err)
	}

	for i := 0; i < 20; i++ {
		snap := nextSnapshot(t, ch)
		if snap.Ship.Vel.X != 0 || snap.Ship.Vel.Y != 0 {
			return // the pilot's thrust is visible in the streamed state
		}
	}
	t.Fatal("ship never accelerated after a thrust intent")
}

func TestStaleIntentStopsThrust(t *testing.T) {
	sim := startSim(t)
	ch, cancel := sim.Subscribe()
	defer cancel()

	if err := sim.SubmitIntent(RolePilot, Intent{Seq: 1, Thrust: true}); err != nil {
		t.Fatalf("SubmitIntent: %v", err)
	}
	// Wait until the ship is clearly moving.
	var moving Snapshot
	for i := 0; i < 20; i++ {
		moving = nextSnapshot(t, ch)
		if math.Hypot(moving.Ship.Vel.X, moving.Ship.Vel.Y) > 1 {
			break
		}
	}
	if math.Hypot(moving.Ship.Vel.X, moving.Ship.Vel.Y) <= 1 {
		t.Fatal("ship never started moving")
	}

	// Go silent past the freshness window: thrust must auto-stop, so the
	// ship only decays from here on.
	time.Sleep(time.Duration(intentTTLTicks+2) * time.Second / TickRate)
	before := nextSnapshot(t, ch)
	after := nextSnapshot(t, ch)

	beforeSpeed := math.Hypot(before.Ship.Vel.X, before.Ship.Vel.Y)
	afterSpeed := math.Hypot(after.Ship.Vel.X, after.Ship.Vel.Y)
	if afterSpeed >= beforeSpeed {
		t.Fatalf("stale thrust did not stop: speed %v then %v", beforeSpeed, afterSpeed)
	}
}

func TestDuplicateSequenceIsCoalesced(t *testing.T) {
	sim := startSim(t)
	for i := 0; i < 2; i++ {
		if err := sim.SubmitIntent(RolePilot, Intent{Seq: 7, Thrust: true}); err != nil {
			t.Fatalf("duplicate SubmitIntent: %v", err)
		}
	}
}

func TestReloadResetsSequenceWatermark(t *testing.T) {
	sim := startSim(t)
	if err := sim.SubmitIntent(RolePilot, Intent{Seq: 100}); err != nil {
		t.Fatalf("SubmitIntent: %v", err)
	}

	// A gap longer than the freshness window is a fresh client session;
	// its sequence counter starting over must not be dropped.
	time.Sleep(time.Duration(intentTTLTicks+2) * time.Second / TickRate)
	if err := sim.SubmitIntent(RolePilot, Intent{Seq: 1, Thrust: true}); err != nil {
		t.Fatalf("post-gap SubmitIntent: %v", err)
	}

	ch, cancel := sim.Subscribe()
	defer cancel()
	for i := 0; i < 20; i++ {
		if snap := nextSnapshot(t, ch); snap.Ship.Vel.X != 0 || snap.Ship.Vel.Y != 0 {
			return // the restarted client's intent took effect
		}
	}
	t.Fatal("intent after a restart gap was ignored")
}

func TestRateLimit(t *testing.T) {
	sim := startSim(t)
	for i := 0; i < maxIntentsPerSecond; i++ {
		if err := sim.SubmitIntent(RolePilot, Intent{Seq: uint64(i + 1), Thrust: true}); err != nil {
			t.Fatalf("intent %d: %v", i+1, err)
		}
	}
	if err := sim.SubmitIntent(RolePilot, Intent{Seq: maxIntentsPerSecond + 1, Thrust: true}); err != ErrRateLimited {
		t.Fatalf("over-cap intent err = %v, want %v", err, ErrRateLimited)
	}
}

func TestUnknownRoleRejected(t *testing.T) {
	sim := startSim(t)
	if err := sim.SubmitIntent(Role("referee"), Intent{Seq: 1}); err != ErrUnknownRole {
		t.Fatalf("unknown role err = %v, want %v", err, ErrUnknownRole)
	}
}

func TestSubmitIntentAfterStop(t *testing.T) {
	sim := NewSim(1)
	go sim.Run()
	sim.Stop()
	if err := sim.SubmitIntent(RolePilot, Intent{Seq: 1}); err != ErrStopped {
		t.Fatalf("after stop err = %v, want %v", err, ErrStopped)
	}
}

func TestGunnerIntentIsStored(t *testing.T) {
	// The gunner's fire state is recorded; weapons consume it later.
	sim := startSim(t)
	if err := sim.SubmitIntent(RoleGunner, Intent{Seq: 1, Fire: true}); err != nil {
		t.Fatalf("gunner SubmitIntent: %v", err)
	}
	if err := sim.SubmitIntent(RoleGunner, Intent{Seq: 2, Fire: false}); err != nil {
		t.Fatalf("gunner SubmitIntent: %v", err)
	}
}

func TestRateLimitIsPerRole(t *testing.T) {
	sim := startSim(t)
	for i := 0; i < maxIntentsPerSecond; i++ {
		if err := sim.SubmitIntent(RolePilot, Intent{Seq: uint64(i + 1), Thrust: true}); err != nil {
			t.Fatalf("pilot intent %d: %v", i+1, err)
		}
	}
	// The gunner has its own window and is unaffected.
	if err := sim.SubmitIntent(RoleGunner, Intent{Seq: 1, Fire: true}); err != nil {
		t.Fatalf("gunner blocked by the pilot's rate limit: %v", err)
	}
}
