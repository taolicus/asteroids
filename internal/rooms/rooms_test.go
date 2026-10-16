package rooms

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestManager() (*Manager, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	m := NewManager()
	m.now = clock.now
	return m, clock
}

func TestCreateReturnsUnguessableID(t *testing.T) {
	m, _ := newTestManager()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id, err := m.Create()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 12 {
			t.Fatalf("id %q is %d chars, want 12", id, len(id))
		}
		for _, r := range id {
			ok := r == '-' || r == '_' ||
				(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			if !ok {
				t.Fatalf("id %q contains non-base64url char %q", id, r)
			}
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestJoinAssignsPilotThenGunner(t *testing.T) {
	m, _ := newTestManager()
	id, err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	role, _, err := m.Join(id, "")
	if err != nil || role != RolePilot {
		t.Fatalf("first join = %q, %v; want pilot, nil", role, err)
	}
	role, _, err = m.Join(id, "")
	if err != nil || role != RoleGunner {
		t.Fatalf("second join = %q, %v; want gunner, nil", role, err)
	}
	if _, _, err = m.Join(id, ""); !errors.Is(err, ErrFull) {
		t.Fatalf("third join err = %v, want ErrFull", err)
	}
	if _, _, err = m.Join("nope", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown room err = %v, want ErrNotFound", err)
	}
}

func TestTokenReattachesFullRoom(t *testing.T) {
	m, _ := newTestManager()
	id, _ := m.Create()
	_, pilotToken, err := m.Join(id, "")
	if err != nil {
		t.Fatal(err)
	}
	m.Join(id, "")

	role, token, err := m.Join(id, pilotToken)
	if err != nil {
		t.Fatalf("token rejoin: %v", err)
	}
	if role != RolePilot || token != pilotToken {
		t.Fatalf("reattach = %q/%q, want pilot/own token", role, token)
	}
	if _, _, err := m.Join(id, "wrong-token"); !errors.Is(err, ErrFull) {
		t.Fatalf("unknown token in full room err = %v, want ErrFull", err)
	}
}

func TestStatusPresence(t *testing.T) {
	m, clock := newTestManager()
	id, _ := m.Create()
	_, pilotToken, _ := m.Join(id, "")
	m.Join(id, "")

	role, state, pilot, gunner, err := m.Status(id, pilotToken)
	if err != nil {
		t.Fatal(err)
	}
	if role != RolePilot || state != "ready" || !pilot || !gunner {
		t.Fatalf("status = %q %q %v %v", role, state, pilot, gunner)
	}

	// Gunner goes silent past the presence TTL; the pilot's poll still
	// works and reports the gunner as gone.
	clock.advance(PresenceTTL + time.Second)
	_, state, pilot, gunner, err = m.Status(id, pilotToken)
	if err != nil {
		t.Fatal(err)
	}
	if state != "ready" {
		t.Fatalf("state = %q after silent gunner; seats stay filled", state)
	}
	if !pilot || gunner {
		t.Fatalf("presence pilot=%v gunner=%v; want true,false", pilot, gunner)
	}
}

func TestStatusRejectsBadTokens(t *testing.T) {
	m, _ := newTestManager()
	id, _ := m.Create()
	_, token, _ := m.Join(id, "")

	if _, _, _, _, err := m.Status(id, ""); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("empty token err = %v, want ErrUnauthorized", err)
	}
	if _, _, _, _, err := m.Status(id, "wrong"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong token err = %v, want ErrUnauthorized", err)
	}
	if _, _, _, _, err := m.Status("nope", token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown room err = %v, want ErrNotFound", err)
	}
}

func TestEmptyRoomExpiresAfterGrace(t *testing.T) {
	m, clock := newTestManager()
	m.Create()

	clock.advance(EmptyGrace - time.Second)
	m.Sweep()
	if m.Len() != 1 {
		t.Fatalf("rooms = %d after %v, want 1", m.Len(), EmptyGrace-time.Second)
	}
	clock.advance(2 * time.Second)
	m.Sweep()
	if m.Len() != 0 {
		t.Fatalf("rooms = %d after %v, want 0", m.Len(), EmptyGrace+time.Second)
	}
}

func TestJoinedRoomGraceStartsWhenLastPlayerGoesSilent(t *testing.T) {
	m, clock := newTestManager()
	id, _ := m.Create()
	m.Join(id, "")

	clock.advance(PresenceTTL + time.Second)
	m.Sweep() // observes absence, starts grace
	if m.Len() != 1 {
		t.Fatal("room removed when grace only started")
	}
	clock.advance(EmptyGrace - time.Second)
	m.Sweep()
	if m.Len() != 1 {
		t.Fatal("room removed before grace elapsed")
	}
	clock.advance(2 * time.Second)
	m.Sweep()
	if m.Len() != 0 {
		t.Fatalf("rooms = %d after grace, want 0", m.Len())
	}
}

func TestRejoinDuringGraceRescuesRoom(t *testing.T) {
	m, clock := newTestManager()
	id, _ := m.Create()

	clock.advance(EmptyGrace - time.Second)
	m.Sweep()
	_, _, err := m.Join(id, "")
	if err != nil {
		t.Fatalf("join during grace: %v", err)
	}
	clock.advance(EmptyGrace + time.Second)
	m.Sweep()
	if m.Len() != 1 {
		t.Fatal("rescued room expired anyway")
	}
}

func TestHardCapRemovesBusyRoom(t *testing.T) {
	m, clock := newTestManager()
	id, _ := m.Create()
	_, token, _ := m.Join(id, "")

	// Keep the player present right up to the cap; age wins anyway.
	clock.advance(MaxAge - time.Second)
	m.Sweep()
	if m.Len() != 1 {
		t.Fatal("room removed before hard cap")
	}
	if _, _, _, _, err := m.Status(id, token); err != nil {
		t.Fatalf("status before cap: %v", err)
	}
	clock.advance(2 * time.Second)
	m.Sweep()
	if m.Len() != 0 {
		t.Fatalf("rooms = %d after hard cap, want 0", m.Len())
	}
}

func TestSweepLoopStopsOnCancel(t *testing.T) {
	m, _ := newTestManager()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sweep loop did not stop on cancel")
	}
}
