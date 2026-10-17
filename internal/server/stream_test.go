package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/taolicus/asteroids/internal/game"
	"github.com/taolicus/asteroids/internal/identity"
	"github.com/taolicus/asteroids/internal/rooms"
)

// newTestServerWithTimings builds a server with shortened stream
// cadences, so keepalive and presence behaviour is testable without
// waiting out the production intervals.
func newTestServerWithTimings(t *testing.T, tm streamTimings) *testServer {
	t.Helper()
	sessions := game.NewSessions(rooms.IdleGrace)
	ts := httptest.NewServer(handlerWithTimings(rooms.NewManager(), identity.NewIssuer([]byte(testKey)), sessions, tm))
	t.Cleanup(func() {
		ts.Close()
		sessions.StopAll()
	})
	return &testServer{Server: ts, sessions: sessions}
}

// openStream opens the events stream and returns the response. The
// caller reads res.Body as raw SSE bytes.
func openStream(t *testing.T, ts *testServer, id, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/lobby/%s/events", ts.URL, id), nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set(playerTokenHeader, token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

// lineScanner delivers every line of body, including the blank lines
// that separate SSE frames. Calling stop unblocks it.
func lineScanner(body io.Reader) (<-chan string, func()) {
	out := make(chan string, 128)
	done := make(chan struct{})
	go func() {
		defer close(out)
		sc := bufio.NewScanner(body)
		for sc.Scan() {
			select {
			case out <- sc.Text():
			case <-done:
				return
			}
		}
	}()
	return out, func() { close(done) }
}

// readSnapshot returns the next snapshot event, skipping keepalives and
// frame delimiters.
func readSnapshot(t *testing.T, lines <-chan string, timeout time.Duration) snapshot {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("stream closed before a snapshot")
			}
			data, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}
			var s snapshot
			if err := json.Unmarshal([]byte(data), &s); err != nil {
				t.Fatalf("decode snapshot %q: %v", data, err)
			}
			return s
		case <-deadline:
			t.Fatal("no snapshot within the deadline")
		}
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within the deadline")
}

func TestEventsStreamsOrderedSnapshots(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, _ := readyLobby(t, ts)

	res := openStream(t, ts, id, ownerToken)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("events status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type = %q, want text/event-stream", ct)
	}
	// The stream starts the room's sim, like the first accepted intent.
	if sim := ts.sessions.Get(id); sim == nil {
		t.Fatal("streaming did not start the room's sim")
	}

	lines, stop := lineScanner(res.Body)
	defer stop()

	first := readSnapshot(t, lines, time.Second)
	if first.Arena != game.ArenaSize {
		t.Fatalf("arena = %v, want %v", first.Arena, game.ArenaSize)
	}
	if first.Phase != phasePlaying {
		t.Fatalf("phase = %q, want %q", first.Phase, phasePlaying)
	}
	if first.Lives != game.StartLives || first.Score != 0 {
		t.Fatalf("lives/score = %d/%d, want %d/0", first.Lives, first.Score, game.StartLives)
	}
	if first.Ship == nil {
		t.Fatal("no ship in the opening snapshot")
	}
	if len(first.Rocks) != game.AsteroidTuning.StartCount {
		t.Fatalf("rocks = %d, want %d", len(first.Rocks), game.AsteroidTuning.StartCount)
	}
	if got := first.Rocks[0].Radius; got != game.TierLarge.Radius() {
		t.Fatalf("rock radius = %v, want %v", got, game.TierLarge.Radius())
	}
	if first.Seq == 0 || first.Tick != first.Seq*game.BroadcastEvery {
		t.Fatalf("first snapshot seq/tick = %d/%d", first.Seq, first.Tick)
	}

	prev := first.Seq
	for i := 0; i < 3; i++ {
		s := readSnapshot(t, lines, time.Second)
		if s.Seq <= prev {
			t.Fatalf("sequence went backwards: %d then %d", prev, s.Seq)
		}
		prev = s.Seq
	}
}

// TestSnapshotForMapsTheRoundState checks the sim-to-wire mapping without
// waiting for a real round to end.
func TestSnapshotForMapsTheRoundState(t *testing.T) {
	over := snapshotFor(game.Snapshot{
		Seq:         8,
		Tick:        16,
		Lives:       0,
		Score:       250,
		Survival:    12.5,
		Ship:        game.Ship{Pos: game.Vec2{X: 1, Y: 2}, Heading: 3},
		Asteroids:   []game.Asteroid{{ID: 4, Tier: game.TierMedium, Pos: game.Vec2{X: 10, Y: 20}}},
		Projectiles: []game.Projectile{{ID: 5, Pos: game.Vec2{X: 30, Y: 40}}},
	})
	if over.Phase != phaseOver {
		t.Fatalf("phase = %q, want %q", over.Phase, phaseOver)
	}
	if over.Ship != nil {
		t.Fatalf("ship = %+v, want nil once the round is over", over.Ship)
	}
	if over.Score != 250 || over.Time != 12.5 {
		t.Fatalf("score/time = %d/%v, want 250/12.5", over.Score, over.Time)
	}
	if len(over.Rocks) != 1 || over.Rocks[0].Radius != game.TierMedium.Radius() {
		t.Fatalf("rocks = %+v", over.Rocks)
	}
	if len(over.Shots) != 1 || over.Shots[0].ID != 5 {
		t.Fatalf("shots = %+v", over.Shots)
	}

	playing := snapshotFor(game.Snapshot{Seq: 1, Tick: 2, Lives: game.StartLives, Invuln: 1.5})
	if playing.Phase != phasePlaying || playing.Ship == nil {
		t.Fatalf("playing phase/ship = %q/%v", playing.Phase, playing.Ship)
	}
	if playing.Ship.Invuln != 1.5 {
		t.Fatalf("invuln = %v, want 1.5", playing.Ship.Invuln)
	}
	// Empty arrays, not null: the renderer iterates them unconditionally.
	if playing.Rocks == nil || playing.Shots == nil {
		t.Fatal("empty rock/shot arrays must be present, not null")
	}
}

func TestEventsStatusCodes(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, guestToken := readyLobby(t, ts)

	loneID, loneOwner := issueIdentity(t, ts)
	enterLobby(t, ts, loneID, loneOwner)

	tests := []struct {
		name  string
		id    string
		token string
		want  int
	}{
		{"owner streams", id, ownerToken, http.StatusOK},
		{"guest streams", id, guestToken, http.StatusOK},
		{"no token", id, "", http.StatusUnauthorized},
		{"unknown token", id, "not-a-credential", http.StatusUnauthorized},
		{"unknown lobby", "zzzzzzzzzzzz", ownerToken, http.StatusNotFound},
		{"malformed id", "short", ownerToken, http.StatusNotFound},
		{"lobby not ready", loneID, loneOwner, http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := openStream(t, ts, tt.id, tt.token)
			if res.StatusCode != tt.want {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.want)
			}
		})
	}
}

func TestEventsDisconnectCleansUp(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, _ := readyLobby(t, ts)

	res := openStream(t, ts, id, ownerToken)
	lines, stop := lineScanner(res.Body)
	defer stop()
	readSnapshot(t, lines, time.Second)

	sim := ts.sessions.Get(id)
	if sim == nil {
		t.Fatal("stream did not start the sim")
	}
	waitFor(t, time.Second, func() bool { return sim.Subscribers() == 1 })

	res.Body.Close()

	// Teardown is asynchronous: unsubscribing goes through the loop.
	waitFor(t, time.Second, func() bool { return sim.Subscribers() == 0 })

	// Only the client left; the room keeps running.
	if ts.sessions.Len() != 1 {
		t.Fatalf("sessions = %d, want 1: the stream stopped the room", ts.sessions.Len())
	}
	if err := sim.SubmitIntent(game.RolePilot, game.Intent{Seq: 1, Thrust: true}); err != nil {
		t.Fatalf("room stopped accepting intents after a disconnect: %v", err)
	}
}

func TestEventsHeartbeat(t *testing.T) {
	ts := newTestServerWithTimings(t, streamTimings{
		heartbeat:    20 * time.Millisecond,
		presence:     time.Hour,
		writeTimeout: time.Second,
	})
	id, ownerToken, _ := readyLobby(t, ts)

	res := openStream(t, ts, id, ownerToken)
	lines, stop := lineScanner(res.Body)
	defer stop()

	// Snapshots flow too; the heartbeat is the comment line.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatal("stream closed before a heartbeat")
			}
			if strings.HasPrefix(line, ": ") {
				return
			}
		case <-deadline:
			t.Fatal("no heartbeat within the deadline")
		}
	}
}

// TestEventsSlowReaderDoesNotStallSim holds a connection open without
// reading the response and checks the simulation keeps ticking.
func TestEventsSlowReaderDoesNotStallSim(t *testing.T) {
	ts := newTestServer(t)
	id, ownerToken, _ := readyLobby(t, ts)

	conn, err := net.Dial("tcp", ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	_, err = fmt.Fprintf(conn, "GET /api/lobby/%s/events HTTP/1.1\r\nHost: %s\r\nX-Player-Token: %s\r\n\r\n",
		id, ts.Listener.Addr(), ownerToken)
	if err != nil {
		t.Fatal(err)
	}

	// The stream starts the sim; wait for it, then never read.
	var sim *game.Sim
	waitFor(t, 2*time.Second, func() bool {
		sim = ts.sessions.Get(id)
		return sim != nil
	})

	before := sim.Ticks()
	time.Sleep(300 * time.Millisecond)
	if got := sim.Ticks(); got <= before+5 {
		t.Fatalf("sim stalled while a client did not read: %d ticks then %d", before, got)
	}
}
