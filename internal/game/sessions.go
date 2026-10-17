// Per-room simulation ownership.
//
// One Sim runs per room, started lazily and reclaimed when idle. This is
// the shared owner that the input endpoint submits intents to and that
// the snapshot stream subscribes to.
//
// A sim owns its whole round sequence, so the lifecycle here stays about
// the room rather than the round: created on first use (an intent or a
// stream subscription), stopped when it goes idle. Rounds within a sim
// start and restart on their own (round.go).
package game

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// SweepInterval is how often Sessions looks for sims to reclaim.
const SweepInterval = time.Second

type session struct {
	sim      *Sim
	lastUsed time.Time
}

// Sessions maps room IDs to running sims.
type Sessions struct {
	mu   sync.Mutex
	sims map[string]*session
	idle time.Duration
	now  func() time.Time
	// seed supplies each new room's simulation seed, so a round is
	// reproducible while rooms differ.
	seed func() int64
}

// NewSessions creates an empty registry. idle is how long a sim with no
// intents and no subscribers survives before reclaim; callers pass the
// lobby's idle grace so the two lifecycles agree.
func NewSessions(idle time.Duration) *Sessions {
	return &Sessions{
		sims: make(map[string]*session),
		idle: idle,
		now:  time.Now,
		seed: rand.Int63,
	}
}

// Start returns the room's sim, creating and running it if needed. It is
// idempotent and marks the sim as used.
func (s *Sessions) Start(id string) *Sim {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.sims[id]
	if !ok {
		e = &session{sim: NewSim(s.seed())}
		s.sims[id] = e
		go e.sim.Run()
	}
	e.lastUsed = s.now()
	return e.sim
}

// Get returns the room's sim, or nil if none is running. It marks the
// sim as used.
func (s *Sessions) Get(id string) *Sim {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.sims[id]
	if !ok {
		return nil
	}
	e.lastUsed = s.now()
	return e.sim
}

// Stop stops and forgets the room's sim. Stopping blocks until the loop
// has exited, so it happens outside the lock.
func (s *Sessions) Stop(id string) {
	s.mu.Lock()
	e, ok := s.sims[id]
	if ok {
		delete(s.sims, id)
	}
	s.mu.Unlock()

	if ok {
		e.sim.Stop()
	}
}

// Len reports how many sims are running. Used by tests.
func (s *Sessions) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sims)
}

// Sweep stops sims that have had no intents and no subscribers for
// longer than the idle grace. A sim with a subscriber is never
// reclaimed, and is kept fresh while it has one.
func (s *Sessions) Sweep() {
	s.mu.Lock()
	now := s.now()
	var stop []*Sim
	for id, e := range s.sims {
		if e.sim.Subscribers() > 0 {
			e.lastUsed = now
			continue
		}
		if now.Sub(e.lastUsed) >= s.idle {
			stop = append(stop, e.sim)
			delete(s.sims, id)
		}
	}
	s.mu.Unlock()

	for _, sim := range stop {
		sim.Stop()
	}
}

// StopAll stops every sim. Used on shutdown and by tests.
func (s *Sessions) StopAll() {
	s.mu.Lock()
	sims := make([]*Sim, 0, len(s.sims))
	for id, e := range s.sims {
		sims = append(sims, e.sim)
		delete(s.sims, id)
	}
	s.mu.Unlock()

	for _, sim := range sims {
		sim.Stop()
	}
}

// StartSweeper runs Sweep every SweepInterval until ctx is cancelled.
func (s *Sessions) StartSweeper(ctx context.Context) {
	ticker := time.NewTicker(SweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Sweep()
		}
	}
}
