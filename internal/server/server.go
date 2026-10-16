// Package server exposes the room HTTP API used by the frontend.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/taolicus/asteroids/internal/rooms"
)

// playerTokenHeader carries the per-player token issued on join.
const playerTokenHeader = "X-Player-Token"

type createResponse struct {
	ID   string `json:"id"`
	Link string `json:"link"`
}

type joinRequest struct {
	Token string `json:"token,omitempty"`
}

type joinResponse struct {
	Role  rooms.Role `json:"role"`
	Token string     `json:"token"`
}

type statusResponse struct {
	Role   rooms.Role `json:"role"`
	State  string     `json:"state"`
	Pilot  bool       `json:"pilot"`
	Gunner bool       `json:"gunner"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Handler returns the API routes for m. The caller mounts it under
// "/api/" alongside the static frontend; paths here are full request
// paths because ServeMux does not strip the mount point.
func Handler(m *rooms.Manager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rooms", func(w http.ResponseWriter, r *http.Request) {
		id, err := m.Create()
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, createResponse{
			ID:   id,
			Link: "/?r=" + id,
		})
	})
	mux.HandleFunc("POST /api/rooms/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		var req joinRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			httpError(w, http.StatusBadRequest, errors.New("invalid JSON body"))
			return
		}
		role, token, err := m.Join(r.PathValue("id"), req.Token)
		if err != nil {
			switch {
			case errors.Is(err, rooms.ErrNotFound):
				httpError(w, http.StatusNotFound, err)
			case errors.Is(err, rooms.ErrFull):
				httpError(w, http.StatusConflict, err)
			default:
				httpError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, joinResponse{Role: role, Token: token})
	})
	mux.HandleFunc("GET /api/rooms/{id}", func(w http.ResponseWriter, r *http.Request) {
		role, state, pilot, gunner, err := m.Status(r.PathValue("id"), r.Header.Get(playerTokenHeader))
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
		writeJSON(w, http.StatusOK, statusResponse{
			Role:   role,
			State:  state,
			Pilot:  pilot,
			Gunner: gunner,
		})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, errorResponse{Error: err.Error()})
}
