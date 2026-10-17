package game

import (
	"runtime"
	"testing"
	"time"
)

func startSim(t *testing.T) *Sim {
	t.Helper()
	sim := NewSim(1)
	go sim.Run()
	t.Cleanup(sim.Stop)
	return sim
}

func TestTickRate(t *testing.T) {
	sim := startSim(t)
	time.Sleep(1050 * time.Millisecond)
	got := sim.Ticks()
	// ~30 ticks in ~1.05s; allow a generous window for scheduler jitter.
	if got < 25 || got > 36 {
		t.Fatalf("tick rate out of range: got %d, want ~30", got)
	}
}

func TestBroadcastRate(t *testing.T) {
	sim := startSim(t)
	ch, cancel := sim.Subscribe()
	defer cancel()

	count := 0
	deadline := time.After(1050 * time.Millisecond)
	for {
		select {
		case <-ch:
			count++
		case <-deadline:
			// ~15 Hz: 10–15 snapshots in ~1.05s.
			if count < 10 || count > 18 {
				t.Fatalf("broadcast rate out of range: got %d, want ~15", count)
			}
			return
		}
	}
}

func TestSnapshotSequenceAndTick(t *testing.T) {
	sim := startSim(t)
	ch, cancel := sim.Subscribe()
	defer cancel()

	prevSeq := uint64(0)
	for i := 0; i < 3; i++ {
		snap, ok := <-ch
		if !ok {
			t.Fatal("mailbox closed unexpectedly")
		}
		if snap.Seq != prevSeq+1 {
			t.Fatalf("sequence jumped: got %d, want %d", snap.Seq, prevSeq+1)
		}
		if snap.Tick != snap.Seq*BroadcastEvery {
			t.Fatalf("tick %d does not match sequence %d", snap.Tick, snap.Seq)
		}
		if i == 0 && (snap.Round != 1 || snap.Phase != PhaseWaiting) {
			t.Fatalf("first snapshot round/phase = %d/%q, want 1/%q",
				snap.Round, snap.Phase, PhaseWaiting)
		}
		prevSeq = snap.Seq
	}
}

// TestRoleOfReportsOppositeRoles checks the roll reaches the server's
// intent path: the two players always hold different roles.
func TestRoleOfReportsOppositeRoles(t *testing.T) {
	sim := startSim(t)

	one, err := sim.RoleOf(PlayerOne)
	if err != nil {
		t.Fatalf("RoleOf(player one) = %v", err)
	}
	two, err := sim.RoleOf(PlayerTwo)
	if err != nil {
		t.Fatalf("RoleOf(player two) = %v", err)
	}
	if one == two {
		t.Fatalf("both players hold %q", one)
	}
	for _, role := range []Role{one, two} {
		if role != RolePilot && role != RoleGunner {
			t.Fatalf("role = %q, want pilot or gunner", role)
		}
	}
}

// TestRestartDuringTheOpeningWindowIsANoOp checks the replay path's
// idempotence: the round about to start is already fresh.
func TestRestartDuringTheOpeningWindowIsANoOp(t *testing.T) {
	sim := startSim(t)
	if err := sim.Restart(); err != nil {
		t.Fatalf("Restart in the opening window = %v, want nil", err)
	}
}

func TestSlowSubscriberDoesNotStallSim(t *testing.T) {
	sim := startSim(t)
	_, cancel := sim.Subscribe() // never reads: mailbox fills, then drops
	defer cancel()

	time.Sleep(550 * time.Millisecond)
	if got := sim.Ticks(); got < 10 {
		t.Fatalf("sim stalled on slow subscriber: got %d ticks, want ~15", got)
	}
}

func TestOneBusySimDoesNotStallAnother(t *testing.T) {
	busy := startSim(t)
	_, cancel := busy.Subscribe() // never reads
	defer cancel()

	calm := startSim(t)
	time.Sleep(550 * time.Millisecond)
	if got := calm.Ticks(); got < 10 {
		t.Fatalf("calm sim stalled by busy sim: got %d ticks, want ~15", got)
	}
}

func TestStopClosesSubscribers(t *testing.T) {
	sim := NewSim(1)
	go sim.Run()
	ch, cancel := sim.Subscribe()
	defer cancel()

	sim.Stop()
	select {
	case _, ok := <-ch:
		if ok {
			// Drain any buffered snapshot, then expect close.
			_, ok = <-ch
		}
		if ok {
			t.Fatal("subscriber channel not closed after Stop")
		}
	default:
		t.Fatal("subscriber channel not closed after Stop")
	}
}

func TestStopWithoutRunDoesNotHang(t *testing.T) {
	sim := NewSim(1)
	done := make(chan struct{})
	go func() {
		sim.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop hung on a sim that was never started")
	}
}

func TestSubscribeAfterStop(t *testing.T) {
	sim := NewSim(1)
	go sim.Run()
	sim.Stop()

	ch, cancel := sim.Subscribe()
	defer cancel()
	if _, ok := <-ch; ok {
		t.Fatal("expected already-closed mailbox after Stop")
	}
}

func TestNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		sim := NewSim(1)
		go sim.Run()
		ch, cancel := sim.Subscribe()
		<-ch // receive one snapshot so we know the loop is alive
		cancel()
		sim.Stop()
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutine leak: %d before, %d after", before, runtime.NumGoroutine())
}
