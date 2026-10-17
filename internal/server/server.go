// Package server exposes the lobby HTTP API used by the frontend.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/taolicus/asteroids/internal/game"
	"github.com/taolicus/asteroids/internal/identity"
	"github.com/taolicus/asteroids/internal/rooms"
)

// playerTokenHeader carries either the owner credential (from the
// identity endpoint) or a guest's per-instance reconnect token.
const playerTokenHeader = "X-Player-Token"

// maxIntentBytes caps an intent body. Intents are tiny; anything larger
// is a client bug or abuse.
const maxIntentBytes = 512

type identityResponse struct {
	LobbyID string `json:"lobbyId"`
	Token   string `json:"token"`
}

type lobbyResponse struct {
	Seat  rooms.Seat `json:"seat"`
	State string     `json:"state"`
	Owner bool       `json:"owner"`
	Guest bool       `json:"guest"`
	// Path is the lobby's relative share link.
	Path string `json:"path"`
	// Token is a freshly issued guest reconnect token, omitted for the
	// owner (whose credential never changes).
	Token string `json:"token,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type intentResponse struct {
	Role string `json:"role"`
	Seq  uint64 `json:"seq"`
}

type restartResponse struct {
	OK bool `json:"ok"`
}

// Handler returns the API routes. The caller mounts it under "/api/"
// alongside the static frontend; paths here are full request paths
// because ServeMux does not strip the mount point.
//
// sessions owns the per-room simulations that intents are submitted to
// and that the snapshot stream subscribes to.
func Handler(m *rooms.Manager, iss *identity.Issuer, sessions *game.Sessions) http.Handler {
	return handlerWithTimings(m, iss, sessions, defaultStreamTimings)
}

// handlerWithTimings builds the routes with explicit stream cadences, so
// tests can shorten the keepalive and presence intervals.
func handlerWithTimings(m *rooms.Manager, iss *identity.Issuer, sessions *game.Sessions, tm streamTimings) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/identities", func(w http.ResponseWriter, r *http.Request) {
		id, err := iss.Issue()
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, identityResponse{LobbyID: id.LobbyID, Token: id.Token})
	})

	mux.HandleFunc("POST /api/lobby/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !identity.ValidID(id) {
			httpError(w, http.StatusNotFound, rooms.ErrNotFound)
			return
		}
		asOwner := isOwner(iss, id, r.Header.Get(playerTokenHeader))
		st, token, err := m.Enter(id, asOwner)
		if err != nil {
			switch {
			case errors.Is(err, rooms.ErrFull):
				httpError(w, http.StatusConflict, err)
			default:
				httpError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, lobbyPayload(id, st, token))
	})

	mux.HandleFunc("GET /api/lobby/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !identity.ValidID(id) {
			httpError(w, http.StatusNotFound, rooms.ErrNotFound)
			return
		}
		token := r.Header.Get(playerTokenHeader)
		asOwner := isOwner(iss, id, token)
		if token == "" {
			httpError(w, http.StatusUnauthorized, rooms.ErrUnauthorized)
			return
		}
		st, err := m.Status(id, token, asOwner)
		if err != nil {
			switch {
			case errors.Is(err, rooms.ErrNotFound):
				httpError(w, http.StatusNotFound, err)
			case errors.Is(err, rooms.ErrUnauthorized):
				httpError(w, http.StatusUnauthorized, err)
			default:
				httpError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, lobbyPayload(id, st, ""))
	})

	// authenticate verifies the caller's player token, refreshes
	// presence, and requires a ready lobby, exactly as the lobby
	// endpoints do. It writes the rejection itself under the given label
	// and reports ok=false, so a handler can return as soon as it does
	// not.
	authenticate := func(w http.ResponseWriter, r *http.Request, what string) (rooms.Status, bool) {
		id := r.PathValue("id")
		if !identity.ValidID(id) {
			httpError(w, http.StatusNotFound, rooms.ErrNotFound)
			return rooms.Status{}, false
		}

		// The call authenticates and refreshes presence, exactly like a
		// lobby poll. Status also reports whether both seats are filled.
		token := r.Header.Get(playerTokenHeader)
		if token == "" {
			log.Printf("%s %s rejected: no player token", what, id)
			httpError(w, http.StatusUnauthorized, rooms.ErrUnauthorized)
			return rooms.Status{}, false
		}
		asOwner := isOwner(iss, id, token)
		st, err := m.Status(id, token, asOwner)
		if err != nil {
			switch {
			case errors.Is(err, rooms.ErrNotFound):
				httpError(w, http.StatusNotFound, err)
			case errors.Is(err, rooms.ErrUnauthorized):
				log.Printf("%s %s rejected: unknown player token", what, id)
				httpError(w, http.StatusUnauthorized, err)
			default:
				httpError(w, http.StatusInternalServerError, err)
			}
			return rooms.Status{}, false
		}
		if st.State != rooms.StateReady {
			httpError(w, http.StatusConflict, errLobbyNotReady)
			return rooms.Status{}, false
		}
		return st, true
	}

	mux.HandleFunc("POST /api/lobby/{id}/intent", func(w http.ResponseWriter, r *http.Request) {
		st, ok := authenticate(w, r, "intent")
		if !ok {
			return
		}
		id := r.PathValue("id")

		// Which controls this player holds is rolled per round, so ask
		// the sim before decoding the body against a role's shape.
		sim := sessions.Start(id)
		role, err := sim.RoleOf(playerFor(st.Seat))
		if err != nil {
			// Before decoding, the only failure left is the stopped loop.
			httpError(w, http.StatusConflict, err)
			return
		}

		if r.ContentLength > maxIntentBytes {
			log.Printf("intent %s rejected: body too large (%d bytes)", id, r.ContentLength)
			httpError(w, http.StatusRequestEntityTooLarge, errIntentTooLarge)
			return
		}
		in, err := decodeIntent(http.MaxBytesReader(w, r.Body, maxIntentBytes), role)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				log.Printf("intent %s rejected: body too large", id)
				httpError(w, http.StatusRequestEntityTooLarge, errIntentTooLarge)
				return
			}
			log.Printf("intent %s/%s rejected: %v", id, role, err)
			httpError(w, http.StatusBadRequest, err)
			return
		}

		if err := sim.SubmitIntent(role, in); err != nil {
			switch {
			case errors.Is(err, game.ErrRateLimited):
				log.Printf("intent %s/%s rejected: %v", id, role, err)
				httpError(w, http.StatusTooManyRequests, err)
			case errors.Is(err, game.ErrStopped), errors.Is(err, game.ErrUnknownRole):
				httpError(w, http.StatusConflict, err)
			default:
				httpError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, intentResponse{Role: string(role), Seq: in.Seq})
	})

	// restart asks the sim for a fresh round after game over. Either
	// player may ask: the room shares one ship and one score, so a replay
	// is a joint decision and the sim announces the new round's roles on
	// the stream.
	mux.HandleFunc("POST /api/lobby/{id}/restart", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticate(w, r, "restart"); !ok {
			return
		}
		id := r.PathValue("id")
		if err := sessions.Start(id).Restart(); err != nil {
			switch {
			case errors.Is(err, game.ErrRoundInProgress), errors.Is(err, game.ErrStopped):
				httpError(w, http.StatusConflict, err)
			default:
				httpError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, restartResponse{OK: true})
	})

	mux.HandleFunc("GET /api/lobby/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		serveEvents(w, r, m, iss, sessions, tm)
	})

	return mux
}

// errLobbyNotReady is returned when a player submits an intent before
// both seats are filled.
var errLobbyNotReady = errors.New("lobby is not ready")

// errIntentTooLarge is returned when an intent body exceeds the cap.
var errIntentTooLarge = errors.New("intent body too large")

// decodeIntent reads one strictly-decoded intent for role. Unknown
// fields, wrong types, and empty bodies are errors, so a player cannot
// send the other role's fields or any state the server did not ask for.
func decodeIntent(r io.Reader, role game.Role) (game.Intent, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var in game.Intent
	switch role {
	case game.RolePilot:
		var body struct {
			Seq    uint64 `json:"seq"`
			Rotate int    `json:"rotate"`
			Thrust bool   `json:"thrust"`
		}
		if err := dec.Decode(&body); err != nil {
			return in, err
		}
		if body.Rotate < -1 || body.Rotate > 1 {
			return in, fmt.Errorf("rotate %d out of range -1..1", body.Rotate)
		}
		in = game.Intent{Seq: body.Seq, Rotate: body.Rotate, Thrust: body.Thrust}
	case game.RoleGunner:
		var body struct {
			Seq  uint64   `json:"seq"`
			Fire bool     `json:"fire"`
			Aim  *float64 `json:"aim"`
		}
		if err := dec.Decode(&body); err != nil {
			return in, err
		}
		in = game.Intent{Seq: body.Seq, Fire: body.Fire, Aim: body.Aim}
	default:
		return in, fmt.Errorf("unknown role %q", role)
	}

	if dec.More() {
		return in, errors.New("unexpected trailing content")
	}
	return in, nil
}

// isOwner reports whether token is a valid owner credential for id.
func isOwner(iss *identity.Issuer, id, token string) bool {
	if token == "" {
		return false
	}
	owner, err := iss.Verify(token)
	return err == nil && owner == id
}

// playerFor and seatFor are the one binding between a room's seats and
// the sim's two players: the owner is player one and the guest is player
// two. The sim rolls roles against players because it knows nothing about
// lobbies; the server owns the mapping in both directions.
func playerFor(seat rooms.Seat) game.Player {
	if seat == rooms.SeatGuest {
		return game.PlayerTwo
	}
	return game.PlayerOne
}

func seatFor(player game.Player) rooms.Seat {
	if player == game.PlayerTwo {
		return rooms.SeatGuest
	}
	return rooms.SeatOwner
}

func lobbyPayload(id string, st rooms.Status, token string) lobbyResponse {
	return lobbyResponse{
		Seat:  st.Seat,
		State: st.State,
		Owner: st.OwnerPresent,
		Guest: st.GuestPresent,
		Path:  "/?r=" + id,
		Token: token,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, errorResponse{Error: err.Error()})
}
