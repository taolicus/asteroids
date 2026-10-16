// Package rooms manages two-player room lifecycle, role assignment,
// player identity, and expiry. Decisions live in
// tasks/closed/resolve-room-lifecycle.md.
package rooms

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// Role is a player's seat in a room.
type Role string

const (
	RolePilot  Role = "pilot"
	RoleGunner Role = "gunner"
)

const (
	// idBytes is the entropy in room IDs and player tokens; base64url
	// without padding yields 12-char strings.
	idBytes = 9
	// PresenceTTL is how long a player counts as connected without an
	// authenticated request.
	PresenceTTL = 15 * time.Second
	// EmptyGrace is how long a room survives with no connected player.
	EmptyGrace = 60 * time.Second
	// MaxAge is the hard room lifetime cap.
	MaxAge = 30 * time.Minute
	// sweepInterval is the periodic expiry sweep cadence.
	sweepInterval = time.Second
)

// Sentinels reported by Manager methods.
var (
	ErrNotFound     = errors.New("room not found")
	ErrFull         = errors.New("room is full")
	ErrUnauthorized = errors.New("unknown player token")
)

type player struct {
	role     Role
	token    string
	lastSeen time.Time
}

type room struct {
	id        string
	createdAt time.Time
	// emptySince is set when no player is connected; zero means at
	// least one player is present.
	emptySince time.Time
	players    map[Role]*player
}

func (p *player) present(now time.Time) bool {
	return now.Sub(p.lastSeen) < PresenceTTL
}

func (r *room) present(now time.Time) bool {
	for _, p := range r.players {
		if p.present(now) {
			return true
		}
	}
	return false
}

// Manager owns every room. All room state is guarded by one mutex;
// there is no per-room goroutine. The expiry sweep runs on a single
// server-wide goroutine started by Start.
type Manager struct {
	mu    sync.Mutex
	rooms map[string]*room
	now   func() time.Time
}

// NewManager creates an empty manager using the wall clock and
// crypto/rand.
func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*room),
		now:   time.Now,
	}
}

func (m *Manager) newID() (string, error) {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Create makes an empty room and returns its ID. The empty-grace
// clock starts at creation: the creator has not joined yet.
func (m *Manager) Create() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var id string
	for {
		var err error
		if id, err = m.newID(); err != nil {
			return "", err
		}
		if _, taken := m.rooms[id]; !taken {
			break
		}
	}
	now := m.now()
	m.rooms[id] = &room{
		id:         id,
		createdAt:  now,
		emptySince: now,
		players:    make(map[Role]*player),
	}
	return id, nil
}

// Join assigns a seat and issues a player token. A non-empty token
// that matches a player of the room reattaches that player to their
// role even when the room is full; an unknown token is ignored and
// treated as a fresh join. Joining refreshes presence.
func (m *Manager) Join(id, token string) (Role, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.rooms[id]
	if !ok {
		return "", "", ErrNotFound
	}
	now := m.now()

	if token != "" {
		for _, p := range r.players {
			if p.token == token {
				p.lastSeen = now
				r.emptySince = time.Time{}
				return p.role, p.token, nil
			}
		}
	}

	for _, role := range []Role{RolePilot, RoleGunner} {
		if _, taken := r.players[role]; !taken {
			tok, err := m.newID()
			if err != nil {
				return "", "", err
			}
			r.players[role] = &player{role: role, token: tok, lastSeen: now}
			r.emptySince = time.Time{}
			return role, tok, nil
		}
	}
	return "", "", ErrFull
}

// Status reports a player's seat and room state, refreshing presence.
// state is "ready" once both seats are filled (a disconnect does not
// unfill a seat) and "waiting" otherwise. pilot and gunner report
// current presence per seat.
func (m *Manager) Status(id, token string) (role Role, state string, pilot, gunner bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.rooms[id]
	if !ok {
		return "", "", false, false, ErrNotFound
	}
	if token == "" {
		return "", "", false, false, ErrUnauthorized
	}
	var me *player
	for _, p := range r.players {
		if p.token == token {
			me = p
			break
		}
	}
	if me == nil {
		return "", "", false, false, ErrUnauthorized
	}

	now := m.now()
	me.lastSeen = now
	r.emptySince = time.Time{}

	pilot = r.players[RolePilot] != nil && r.players[RolePilot].present(now)
	gunner = r.players[RoleGunner] != nil && r.players[RoleGunner].present(now)
	state = "waiting"
	if r.players[RolePilot] != nil && r.players[RoleGunner] != nil {
		state = "ready"
	}
	return me.role, state, pilot, gunner, nil
}

// Sweep removes expired rooms: past the hard age cap, or with no
// connected player for longer than the empty grace period.
func (m *Manager) Sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	for id, r := range m.rooms {
		if now.Sub(r.createdAt) >= MaxAge {
			delete(m.rooms, id)
			continue
		}
		if r.present(now) {
			r.emptySince = time.Time{}
			continue
		}
		if r.emptySince.IsZero() {
			r.emptySince = now
		}
		if now.Sub(r.emptySince) >= EmptyGrace {
			delete(m.rooms, id)
		}
	}
}

// Start runs the expiry sweep every second until ctx is cancelled. It
// is a single server-wide goroutine; individual rooms never own one.
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Sweep()
		}
	}
}

// Len reports the number of live rooms. Used by tests.
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms)
}
