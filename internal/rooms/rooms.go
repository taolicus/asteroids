// Package rooms manages ephemeral two-player lobby instances and the
// seats in them.
//
// A lobby is identified by a permanent, public lobby ID. The instance
// behind it is created lazily on first use and reclaimed after an idle
// window, so memory is bounded by concurrently connected players rather
// than by how many links have ever been shared. Seats follow identity:
// the owner seat is reserved for the player holding the owner
// credential, and everyone else takes the guest seat. Roles are not tied
// to seats; they are rolled per round elsewhere.
//
// Decisions live in tasks/closed/player-owned-lobby-rooms.md.
package rooms

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// Seat is the seat a player holds in a lobby. It is not a role: roles are
// rolled at each round start by the simulation, and the server binds the
// owner seat to the first player and the guest to the second.
type Seat string

const (
	SeatOwner Seat = "owner"
	SeatGuest Seat = "guest"
)

// Lobby states reported in Status.State.
const (
	// StateWaiting means the lobby is missing at least one player.
	StateWaiting = "waiting"
	// StateReady means both players are present.
	StateReady = "ready"
)

const (
	// idBytes is the entropy in per-instance guest tokens; base64url
	// without padding yields 12-char strings.
	idBytes = 9
	// PresenceTTL is how long a player counts as connected without an
	// authenticated request.
	PresenceTTL = 15 * time.Second
	// IdleGrace is how long an instance survives with no connected
	// player before it is reclaimed. Reclaiming never invalidates the
	// link: the next use recreates the instance under the same ID.
	IdleGrace = 60 * time.Second
	// sweepInterval is the periodic reclaim cadence.
	sweepInterval = time.Second
)

// Sentinels reported by Manager methods.
var (
	ErrNotFound     = errors.New("lobby not found")
	ErrFull         = errors.New("lobby is full")
	ErrUnauthorized = errors.New("unknown player token")
)

type player struct {
	// token is the per-instance reconnect token. The owner's secret
	// credential identifies them instead, so the owner's is empty.
	token    string
	lastSeen time.Time
}

func (p *player) present(now time.Time) bool {
	if p == nil {
		return false
	}
	return !p.lastSeen.IsZero() && now.Sub(p.lastSeen) < PresenceTTL
}

type lobby struct {
	id string
	// owner holds the owner seat; its lastSeen is zero until the owner
	// has ever entered.
	owner player
	// guest is nil until someone takes the guest seat, and is replaced
	// once its holder has been absent past the presence TTL.
	guest *player
	// emptySince is set when no player is connected; zero means at
	// least one player is present.
	emptySince time.Time
}

// Status reports the lobby's observable state to one player.
type Status struct {
	// Seat is the seat held by the player the status is for.
	Seat Seat
	// State is "ready" when both players are present and "waiting"
	// otherwise. A lobbied guest waits while the owner is elsewhere.
	State string
	// OwnerPresent and GuestPresent report current presence per seat.
	OwnerPresent bool
	GuestPresent bool
}

// Manager owns every lobby instance. All state is guarded by one mutex;
// there is no per-lobby goroutine. The reclaim sweep runs on a single
// server-wide goroutine started by Start.
type Manager struct {
	mu      sync.Mutex
	lobbies map[string]*lobby
	now     func() time.Time
}

// NewManager creates an empty manager using the wall clock and
// crypto/rand.
func NewManager() *Manager {
	return &Manager{
		lobbies: make(map[string]*lobby),
		now:     time.Now,
	}
}

func (m *Manager) newID() (string, error) {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Enter creates the instance if needed, assigns the caller a seat, and
// refreshes their presence. asOwner must have been established by
// verifying the caller's credential for this lobby. It returns the
// caller's view of the lobby and, for a guest, a reconnect token.
func (m *Manager) Enter(id string, asOwner bool) (Status, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	l, ok := m.lobbies[id]
	if !ok {
		l = &lobby{id: id}
		m.lobbies[id] = l
	}

	if asOwner {
		l.owner.lastSeen = now
		l.emptySince = time.Time{}
		return l.status(now, SeatOwner), "", nil
	}

	if l.guest.present(now) {
		return Status{}, "", ErrFull
	}
	token, err := m.newID()
	if err != nil {
		return Status{}, "", err
	}
	l.guest = &player{token: token, lastSeen: now}
	l.emptySince = time.Time{}
	return l.status(now, SeatGuest), token, nil
}

// Leave marks the player absent at once instead of waiting out their
// presence TTL. It is what a closing tab's beacon calls; the token rules
// are the same as Status's, and leaving an instance that has moved on
// (reclaimed, or the guest seat replaced) is refused rather than
// applied to whoever sits there now.
func (m *Manager) Leave(id, token string, asOwner bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, ok := m.lobbies[id]
	if !ok {
		return ErrNotFound
	}
	if asOwner {
		l.owner.lastSeen = time.Time{}
		return nil
	}
	if token == "" || l.guest == nil ||
		subtle.ConstantTimeCompare([]byte(token), []byte(l.guest.token)) != 1 {
		return ErrUnauthorized
	}
	l.guest.lastSeen = time.Time{}
	return nil
}

// SetClock replaces the manager's clock. Production keeps the wall clock
// from NewManager; it exists so tests outside this package can exercise
// presence expiry without waiting out PresenceTTL, and must be called
// before the manager is served.
func (m *Manager) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

// Status reports the caller's view, refreshing their presence. For the
// owner, asOwner must already be established; for a guest, token must
// match the instance's guest token.
func (m *Manager) Status(id, token string, asOwner bool) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, ok := m.lobbies[id]
	if !ok {
		return Status{}, ErrNotFound
	}
	now := m.now()

	if asOwner {
		l.owner.lastSeen = now
		l.emptySince = time.Time{}
		return l.status(now, SeatOwner), nil
	}
	if token == "" || l.guest == nil ||
		subtle.ConstantTimeCompare([]byte(token), []byte(l.guest.token)) != 1 {
		return Status{}, ErrUnauthorized
	}
	l.guest.lastSeen = now
	l.emptySince = time.Time{}
	return l.status(now, SeatGuest), nil
}

func (l *lobby) present(now time.Time) bool {
	return l.owner.present(now) || l.guest.present(now)
}

func (l *lobby) status(now time.Time, seat Seat) Status {
	owner := l.owner.present(now)
	guest := l.guest.present(now)
	state := StateWaiting
	if owner && guest {
		state = StateReady
	}
	return Status{Seat: seat, State: state, OwnerPresent: owner, GuestPresent: guest}
}

// Sweep reclaims instances that have had no connected player for longer
// than the idle grace. There is no age cap: a lobby a player is
// legitimately waiting in must not be killed.
func (m *Manager) Sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	for id, l := range m.lobbies {
		if l.present(now) {
			l.emptySince = time.Time{}
			continue
		}
		if l.emptySince.IsZero() {
			l.emptySince = now
		}
		if now.Sub(l.emptySince) >= IdleGrace {
			delete(m.lobbies, id)
		}
	}
}

// Start runs the reclaim sweep every second until ctx is cancelled. It
// is a single server-wide goroutine; individual lobbies never own one.
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

// Len reports the number of live instances. Used by tests.
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.lobbies)
}
