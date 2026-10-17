package game

import (
	"testing"
	"time"
)

func TestSessionsStartIsIdempotent(t *testing.T) {
	s := NewSessions(time.Minute)
	t.Cleanup(s.StopAll)

	first := s.Start("a")
	if second := s.Start("a"); first != second {
		t.Fatal("Start returned a different sim for the same room")
	}
	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1", s.Len())
	}
	if s.Get("a") != first {
		t.Fatal("Get did not return the started sim")
	}
	if s.Get("missing") != nil {
		t.Fatal("Get returned a sim for an unknown room")
	}
}

func TestSessionsStop(t *testing.T) {
	s := NewSessions(time.Minute)
	sim := s.Start("a")

	s.Stop("a")
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0 after Stop", s.Len())
	}
	// The loop really stopped: intents are refused.
	if err := sim.SubmitIntent(RolePilot, Intent{Seq: 1}); err != ErrStopped {
		t.Fatalf("stopped sim err = %v, want %v", err, ErrStopped)
	}
}

func TestSessionsSweepReclaimsIdle(t *testing.T) {
	s := NewSessions(time.Minute)
	now := time.Now()
	s.now = func() time.Time { return now }
	sim := s.Start("a")

	// Within the grace the sim survives.
	now = now.Add(time.Minute - time.Second)
	s.Sweep()
	if s.Len() != 1 {
		t.Fatal("sim reclaimed before the idle grace elapsed")
	}

	// Past the grace it is stopped and forgotten.
	now = now.Add(2 * time.Second)
	s.Sweep()
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0 after the grace", s.Len())
	}
	if err := sim.SubmitIntent(RolePilot, Intent{Seq: 1}); err != ErrStopped {
		t.Fatalf("reclaimed sim err = %v, want %v", err, ErrStopped)
	}
}

func TestSessionsSweepKeepsSubscribed(t *testing.T) {
	s := NewSessions(time.Minute)
	now := time.Now()
	s.now = func() time.Time { return now }
	sim := s.Start("a")

	ch, cancel := sim.Subscribe()
	defer cancel()
	<-ch // the subscription is live

	now = now.Add(time.Hour)
	s.Sweep()
	if s.Len() != 1 {
		t.Fatal("sim with a live subscriber was reclaimed")
	}

	// Once the subscriber leaves, the sim becomes reclaimable again.
	// Unsubscribe is asynchronous, so wait for the loop to record it.
	cancel()
	deadline := time.Now().Add(time.Second)
	for sim.Subscribers() > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if sim.Subscribers() != 0 {
		t.Fatal("subscriber was not removed")
	}

	now = now.Add(time.Hour)
	s.Sweep()
	if s.Len() != 0 {
		t.Fatal("sim was not reclaimed after its last subscriber left")
	}
}

func TestSessionsStopAll(t *testing.T) {
	s := NewSessions(time.Minute)
	a := s.Start("a")
	b := s.Start("b")

	s.StopAll()
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0 after StopAll", s.Len())
	}
	for _, sim := range []*Sim{a, b} {
		if err := sim.SubmitIntent(RolePilot, Intent{Seq: 1}); err != ErrStopped {
			t.Fatalf("sim err = %v, want %v", err, ErrStopped)
		}
	}
}
