// Package server exposes the lobby HTTP API used by the frontend.
package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/taolicus/asteroids/internal/identity"
	"github.com/taolicus/asteroids/internal/rooms"
)

// playerTokenHeader carries either the owner credential (from the
// identity endpoint) or a guest's per-instance reconnect token.
const playerTokenHeader = "X-Player-Token"

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

// Handler returns the API routes. The caller mounts it under "/api/"
// alongside the static frontend; paths here are full request paths
// because ServeMux does not strip the mount point.
func Handler(m *rooms.Manager, iss *identity.Issuer) http.Handler {
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

	return mux
}

// isOwner reports whether token is a valid owner credential for id.
func isOwner(iss *identity.Issuer, id, token string) bool {
	if token == "" {
		return false
	}
	owner, err := iss.Verify(token)
	return err == nil && owner == id
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
