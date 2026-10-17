// The SSE snapshot stream: the server half of the wire protocol whose
// format and client interpolation are settled in
// tasks/closed/define-snapshot-format-and-interpolation.md.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/taolicus/asteroids/internal/game"
	"github.com/taolicus/asteroids/internal/identity"
	"github.com/taolicus/asteroids/internal/rooms"
)

// streamTimings groups the stream's liveness cadences so tests can
// shorten them.
type streamTimings struct {
	// heartbeat is how often a comment frame is written when snapshots
	// pause, so a proxy does not treat an idle stream as dead.
	heartbeat time.Duration
	// presence is how often the player's presence is refreshed. It must
	// stay well inside rooms.PresenceTTL, or a player with a live stream
	// would look absent.
	presence time.Duration
	// writeTimeout bounds a single write, so a reader that stops reading
	// is disconnected instead of pinning the handler.
	writeTimeout time.Duration
}

var defaultStreamTimings = streamTimings{
	heartbeat:    15 * time.Second,
	presence:     5 * time.Second,
	writeTimeout: 15 * time.Second,
}

// phasePlaying is the round phase while there are lives left, and
// phaseOver once the sim reports zero. Waiting is still round flow's to
// report (implement-round-flow-and-replay.md).
const (
	phasePlaying = "playing"
	phaseOver    = "over"
)

// heartbeatFrame is an SSE comment: it is ignored by parsers but keeps
// the connection warm.
const heartbeatFrame = ": ping\n\n"

// snapshot is the wire form of one simulation snapshot. The sim's own
// Snapshot is not the wire format: this DTO is the protocol surface, and
// it grows as the sim gains state. See the settled contract for every
// field.
type snapshot struct {
	Seq   uint64  `json:"seq"`
	Tick  uint64  `json:"tick"`
	Arena float64 `json:"arena"`
	Phase string  `json:"phase"`
	Lives int     `json:"lives"`
	Score int     `json:"score"`
	// Time is the survival timer in seconds.
	Time  float64     `json:"time"`
	Ship  *shipState  `json:"ship"`
	Rocks []rockState `json:"rocks"`
	Shots []shotState `json:"shots"`
}

type shipState struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Heading float64 `json:"heading"`
	Invuln  float64 `json:"invuln"`
}

type rockState struct {
	ID     uint64  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Radius float64 `json:"radius"`
}

type shotState struct {
	ID uint64  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// snapshotFor maps the sim's current state to the wire schema. The ship
// and the phase both follow the life count: there is no ship once the
// round is over.
func snapshotFor(snap game.Snapshot) snapshot {
	s := snapshot{
		Seq:   snap.Seq,
		Tick:  snap.Tick,
		Arena: game.ArenaSize,
		Phase: phasePlaying,
		Lives: snap.Lives,
		Score: snap.Score,
		Time:  snap.Survival,
		Rocks: make([]rockState, 0, len(snap.Asteroids)),
		Shots: make([]shotState, 0, len(snap.Projectiles)),
	}
	if snap.Lives <= 0 {
		s.Phase = phaseOver
	} else {
		s.Ship = &shipState{
			X:       snap.Ship.Pos.X,
			Y:       snap.Ship.Pos.Y,
			Heading: snap.Ship.Heading,
			Invuln:  snap.Invuln,
		}
	}
	for _, a := range snap.Asteroids {
		s.Rocks = append(s.Rocks, rockState{
			ID:     a.ID,
			X:      a.Pos.X,
			Y:      a.Pos.Y,
			Radius: a.Radius(),
		})
	}
	for _, p := range snap.Projectiles {
		s.Shots = append(s.Shots, shotState{ID: p.ID, X: p.Pos.X, Y: p.Pos.Y})
	}
	return s
}

// snapshotFrame renders one snapshot as an SSE event. The JSON is
// compact and has no newlines, so a single data field suffices.
func snapshotFrame(s snapshot) (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("event: snapshot\nid: %d\ndata: %s\n\n", s.Seq, data), nil
}

// sseWriter writes frames to one streamed response with a write deadline,
// then flushes each frame so a client sees it immediately.
type sseWriter struct {
	w       io.Writer
	flusher http.Flusher
	rc      *http.ResponseController
	timeout time.Duration
}

func (s sseWriter) write(frame string) error {
	if err := s.rc.SetWriteDeadline(time.Now().Add(s.timeout)); err != nil &&
		!errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := io.WriteString(s.w, frame); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// serveEvents streams the room's snapshots to one player as SSE. It
// authenticates exactly like the lobby endpoints, requires both players
// to be present, and treats the open stream as presence.
func serveEvents(w http.ResponseWriter, r *http.Request, m *rooms.Manager, iss *identity.Issuer, sessions *game.Sessions, tm streamTimings) {
	id := r.PathValue("id")
	if !identity.ValidID(id) {
		httpError(w, http.StatusNotFound, rooms.ErrNotFound)
		return
	}

	token := r.Header.Get(playerTokenHeader)
	if token == "" {
		log.Printf("events %s rejected: no player token", id)
		httpError(w, http.StatusUnauthorized, rooms.ErrUnauthorized)
		return
	}

	// Status authenticates the token and refreshes presence. Presence is
	// refreshed before the ready check, so a player who just reconnected
	// after their own presence lapsed is not refused by their own
	// absence.
	asOwner := isOwner(iss, id, token)
	st, err := m.Status(id, token, asOwner)
	if err != nil {
		switch {
		case errors.Is(err, rooms.ErrNotFound):
			httpError(w, http.StatusNotFound, err)
		case errors.Is(err, rooms.ErrUnauthorized):
			log.Printf("events %s rejected: unknown player token", id)
			httpError(w, http.StatusUnauthorized, err)
		default:
			httpError(w, http.StatusInternalServerError, err)
		}
		return
	}
	if st.State != rooms.StateReady {
		httpError(w, http.StatusConflict, errLobbyNotReady)
		return
	}

	// Every status above is a real HTTP status, because the stream has
	// not started yet. From here the response is a live event stream.
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpError(w, http.StatusInternalServerError, errors.New("response writer does not support streaming"))
		return
	}

	// Subscribing starts the room's sim if it is not running, like the
	// first accepted intent. The mailbox is capacity-1 latest-wins, so a
	// slow reader drops snapshots and never blocks the sim.
	sim := sessions.Start(id)
	ch, cancel := sim.Subscribe()
	defer cancel()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	// Ask proxies (nginx) not to buffer; buffered events never arrive.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	out := sseWriter{
		w:       w,
		flusher: flusher,
		rc:      http.NewResponseController(w),
		timeout: tm.writeTimeout,
	}

	heartbeat := time.NewTicker(tm.heartbeat)
	defer heartbeat.Stop()
	presence := time.NewTicker(tm.presence)
	defer presence.Stop()

	for {
		select {
		case <-r.Context().Done():
			// The client went away; teardown is just cancel.
			return
		case <-presence.C:
			// A live stream counts as presence. If the seat is gone
			// (reclaimed or replaced) there is nothing left to stream.
			if _, err := m.Status(id, token, asOwner); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := out.write(heartbeatFrame); err != nil {
				return
			}
		case snap, ok := <-ch:
			if !ok {
				// The sim stopped; the room is gone.
				return
			}
			frame, err := snapshotFrame(snapshotFor(snap))
			if err != nil {
				log.Printf("events %s: encode snapshot: %v", id, err)
				return
			}
			if err := out.write(frame); err != nil {
				return
			}
		}
	}
}
