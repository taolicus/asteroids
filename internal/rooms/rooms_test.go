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

const testLobby = "abcdefghijkl"

func TestEnterCreatesInstanceLazily(t *testing.T) {
	m, _ := newTestManager()

	st, token, err := m.Enter(testLobby, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Seat != SeatOwner || st.State != "waiting" || !st.OwnerPresent || st.GuestPresent {
		t.Fatalf("owner enter = %+v", st)
	}
	if token != "" {
		t.Fatalf("owner token = %q, want empty (the credential identifies them)", token)
	}
	if m.Len() != 1 {
		t.Fatalf("instances = %d, want 1", m.Len())
	}
}

func TestSeatsFollowArrivalOrderIndependently(t *testing.T) {
	m, _ := newTestManager()

	// Guest first: the owner seat stays reserved for the owner.
	gst, guestToken, err := m.Enter(testLobby, false)
	if err != nil {
		t.Fatal(err)
	}
	if gst.Seat != SeatGuest || gst.State != "waiting" || gst.OwnerPresent {
		t.Fatalf("guest-only enter = %+v", gst)
	}
	if guestToken == "" {
		t.Fatal("guest got no reconnect token")
	}

	// Owner arrives later and takes the owner seat, not the guest seat.
	ost, _, err := m.Enter(testLobby, true)
	if err != nil {
		t.Fatal(err)
	}
	if ost.Seat != SeatOwner || ost.State != "ready" || !ost.OwnerPresent || !ost.GuestPresent {
		t.Fatalf("owner-later enter = %+v", ost)
	}

	// The guest polls with their token and sees the shared state.
	gst2, err := m.Status(testLobby, guestToken, false)
	if err != nil {
		t.Fatal(err)
	}
	if gst2.Seat != SeatGuest || gst2.State != "ready" {
		t.Fatalf("guest status = %+v", gst2)
	}
}

func TestSecondGuestSeesFull(t *testing.T) {
	m, _ := newTestManager()
	if _, _, err := m.Enter(testLobby, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Enter(testLobby, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Enter(testLobby, false); !errors.Is(err, ErrFull) {
		t.Fatalf("second guest err = %v, want ErrFull", err)
	}
}

func TestOwnerIsNotDisplacedByGuest(t *testing.T) {
	m, _ := newTestManager()

	// A guest arrives first; the owner seat stays reserved for the owner.
	if st, _, err := m.Enter(testLobby, false); err != nil || st.Seat != SeatGuest {
		t.Fatalf("first guest = %+v, %v", st, err)
	}
	if st, _, err := m.Enter(testLobby, true); err != nil || st.Seat != SeatOwner {
		t.Fatalf("owner seat = %+v, %v; want owner", st, err)
	}
	// A guest arriving a second time while still present is still just a
	// full lobby, never the owner seat.
	if st, _, err := m.Enter(testLobby, false); !errors.Is(err, ErrFull) {
		t.Fatalf("repeat guest = %+v, %v; want ErrFull", st, err)
	}
}

func TestGuestSeatFreesWhenHolderGoesAbsent(t *testing.T) {
	m, clock := newTestManager()
	m.Enter(testLobby, true)
	_, oldToken, _ := m.Enter(testLobby, false)

	clock.advance(PresenceTTL + time.Second)

	// The absent guest no longer holds the seat, so a new guest replaces
	// them and the old token stops working.
	_, newToken, err := m.Enter(testLobby, false)
	if err != nil {
		t.Fatalf("new guest after absence: %v", err)
	}
	if newToken == oldToken {
		t.Fatal("new guest reused the absent guest's token")
	}
	if _, err := m.Status(testLobby, oldToken, false); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("stale guest token err = %v, want ErrUnauthorized", err)
	}
}

func TestGuestReconnectWithinPresenceTTL(t *testing.T) {
	m, clock := newTestManager()
	_, token, _ := m.Enter(testLobby, false)

	clock.advance(PresenceTTL - time.Second)
	st, err := m.Status(testLobby, token, false)
	if err != nil {
		t.Fatalf("guest reconnect: %v", err)
	}
	if st.Seat != SeatGuest || !st.GuestPresent {
		t.Fatalf("guest status = %+v", st)
	}
}

func TestStatusAuthentication(t *testing.T) {
	m, _ := newTestManager()
	m.Enter(testLobby, true)
	_, guestToken, _ := m.Enter(testLobby, false)

	if _, err := m.Status("missinglobby", guestToken, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown lobby err = %v, want ErrNotFound", err)
	}
	if _, err := m.Status(testLobby, "", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("empty token err = %v, want ErrUnauthorized", err)
	}
	if _, err := m.Status(testLobby, "wrong-token", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong token err = %v, want ErrUnauthorized", err)
	}
	if st, err := m.Status(testLobby, "", true); err != nil || st.Seat != SeatOwner {
		t.Fatalf("owner status = %+v, %v", st, err)
	}
}

func TestReclaimAndRecreateUnderSameID(t *testing.T) {
	m, clock := newTestManager()
	m.Enter(testLobby, true)

	// Nobody present long enough to start the grace, then to elapse it.
	clock.advance(PresenceTTL + time.Second)
	m.Sweep()
	if m.Len() != 1 {
		t.Fatal("reclaimed while grace only started")
	}
	clock.advance(IdleGrace + time.Second)
	m.Sweep()
	if m.Len() != 0 {
		t.Fatalf("instances = %d after grace, want 0", m.Len())
	}

	// The link is unchanged: the same ID recreates the instance, and the
	// owner still gets the owner seat.
	st, _, err := m.Enter(testLobby, true)
	if err != nil {
		t.Fatalf("recreate: %v", err)
	}
	if st.Seat != SeatOwner {
		t.Fatalf("recreated seat = %q, want owner", st.Seat)
	}
}

func TestActivityRescuesInstanceDuringGrace(t *testing.T) {
	m, clock := newTestManager()
	m.Enter(testLobby, true)

	clock.advance(PresenceTTL + time.Second)
	m.Sweep() // starts the grace
	clock.advance(IdleGrace - time.Second)
	if _, _, err := m.Enter(testLobby, true); err != nil {
		t.Fatalf("enter during grace: %v", err)
	}
	clock.advance(IdleGrace + time.Second)
	m.Sweep()
	if m.Len() != 1 {
		t.Fatal("rescued instance was reclaimed anyway")
	}
}

func TestNoAgeCapWhilePresent(t *testing.T) {
	m, clock := newTestManager()
	m.Enter(testLobby, true)

	// Keep the owner present for well past the old 30-minute cap; the
	// instance must survive because it is not abandoned.
	for i := 0; i < 100; i++ {
		clock.advance(30 * time.Second)
		if _, err := m.Status(testLobby, "", true); err != nil {
			t.Fatalf("owner status at step %d: %v", i, err)
		}
		m.Sweep()
	}
	if m.Len() != 1 {
		t.Fatalf("instances = %d after 50m of presence, want 1", m.Len())
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
