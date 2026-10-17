// Package game hosts the authoritative per-room simulation loop.
package game

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// TickRate is the fixed simulation rate per room.
const TickRate = 30

// BroadcastEvery is how many ticks pass between snapshot broadcasts.
// TickRate/BroadcastEvery = 15 Hz, within the README's 10–15 Hz target.
const BroadcastEvery = 2

// intentTTLTicks is how long an intent stays live without a refresh:
// 8 ticks ≈ 267 ms at 30 Hz. Past it the loop applies a neutral intent,
// so a backgrounded tab cannot leave thrust or rotation stuck on. See
// tasks/closed/define-input-intent-contract.md.
const intentTTLTicks = 8

// maxIntentsPerSecond caps accepted intents per role per second. It is a
// coarse safety bound, not a gameplay limit: excess is normally harmless
// because the loop only samples the latest intent.
const maxIntentsPerSecond = 2 * TickRate

// Snapshot is one immutable view of room state produced by the loop. It
// carries the round's number, phase, and role roll alongside the shared
// score, lives, and survival timer. Slices are value copies, safe to read
// from another goroutine.
type Snapshot struct {
	Seq   uint64
	Tick  uint64
	Round uint64
	Phase Phase
	// Pilot is the player holding the pilot's controls this round; the
	// other player is the gunner.
	Pilot       Player
	Ship        Ship
	Lives       int
	Score       int
	Survival    float64 // seconds survived this round
	Invuln      float64 // seconds of ship invulnerability remaining
	Asteroids   []Asteroid
	Projectiles []Projectile
}

// intentSlot is one role's latest intent plus the bookkeeping used to
// coalesce stale posts and cap the rate.
type intentSlot struct {
	in     Intent
	tick   uint64 // tick at which in last refreshed
	has    bool
	window uint64 // current rate window (seconds since start)
	count  int    // intents accepted within the window
}

// ErrStopped is returned by Subscribe after the loop has stopped.
var ErrStopped = errors.New("sim stopped")

// Sim runs a fixed-timestep loop on a single goroutine and fans snapshots
// out to subscribers without ever blocking on them.
type Sim struct {
	seed     int64
	cmds     chan any
	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	startMu sync.Mutex
	started bool

	ticks atomic.Uint64
	subs  atomic.Int64
}

type subscribeReq struct {
	ch chan Snapshot
}

type unsubscribeReq struct {
	ch chan Snapshot
}

type intentReq struct {
	role  Role
	in    Intent
	reply chan error
}

type roleReq struct {
	player Player
	reply  chan Role
}

type restartReq struct {
	reply chan error
}

// NewSim creates a stopped sim seeded with seed. The seed makes the
// round's asteroid field and split directions reproducible. Call Run to
// start the loop.
func NewSim(seed int64) *Sim {
	return &Sim{
		seed: seed,
		cmds: make(chan any),
		done: make(chan struct{}),
	}
}

// Run starts the loop on the calling goroutine's caller-owned lifecycle:
// it returns after Stop is called and the loop has drained. Callers
// typically go sim.Run() in a dedicated goroutine.
func (s *Sim) Run() {
	s.wg.Add(1)
	s.startMu.Lock()
	s.started = true
	s.startMu.Unlock()

	defer s.wg.Done()

	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()

	var tick uint64
	var seq uint64
	r := newRoundState(rand.New(rand.NewSource(s.seed)))
	r.startRound(0)
	var pilot, gunner intentSlot
	subscribers := make(map[chan Snapshot]struct{})

	for {
		select {
		case <-s.done:
			for ch := range subscribers {
				close(ch)
			}
			s.subs.Store(0)
			return
		case cmd := <-s.cmds:
			switch c := cmd.(type) {
			case subscribeReq:
				subscribers[c.ch] = struct{}{}
				s.subs.Add(1)
			case unsubscribeReq:
				if _, ok := subscribers[c.ch]; ok {
					delete(subscribers, c.ch)
					s.subs.Add(-1)
					close(c.ch)
				}
			case intentReq:
				c.reply <- applyIntent(slotFor(&pilot, &gunner, c.role), c.in, tick)
			case roleReq:
				c.reply <- roleOf(c.player, r.pilot)
			case restartReq:
				c.reply <- r.restart(tick)
			}
		case <-ticker.C:
			tick++
			s.ticks.Store(tick)
			r.step(tick, liveIntent(&pilot, tick), liveIntent(&gunner, tick), 1.0/TickRate)
			if tick%BroadcastEvery != 0 {
				continue
			}
			seq++
			snap := r.snapshot(seq, tick)
			for ch := range subscribers {
				trySendLatest(ch, snap)
			}
		}
	}
}

// roleOf returns the role a player holds while pilot is the round's
// pilot.
func roleOf(player, pilot Player) Role {
	if player == pilot {
		return RolePilot
	}
	return RoleGunner
}

// slotFor returns the intent slot for role, or nil if the role is
// unknown.
func slotFor(pilot, gunner *intentSlot, role Role) *intentSlot {
	switch role {
	case RolePilot:
		return pilot
	case RoleGunner:
		return gunner
	default:
		return nil
	}
}

// applyIntent records an intent in slot. It returns nil when the intent
// is accepted or coalesced away, ErrUnknownRole for a nil slot, and
// ErrRateLimited when the role has exceeded the coarse cap.
//
// An intent that arrives after a gap longer than the freshness window
// starts a fresh session: the sequence watermark and rate window reset,
// so a reloaded client beginning again at seq 1 is not dropped.
func applyIntent(slot *intentSlot, in Intent, tick uint64) error {
	if slot == nil {
		return ErrUnknownRole
	}
	window := tick / TickRate
	stale := !slot.has || tick-slot.tick > intentTTLTicks

	switch {
	case stale:
		slot.count = 0
		slot.window = window
	case in.Seq != 0 && in.Seq <= slot.in.Seq:
		// Reordered or duplicated by the network: coalesced away.
		return nil
	}

	if slot.window != window {
		slot.window = window
		slot.count = 0
	}
	if slot.count >= maxIntentsPerSecond {
		return ErrRateLimited
	}
	slot.count++
	slot.in = in
	slot.tick = tick
	slot.has = true
	return nil
}

// liveIntent returns the intent to apply this tick, or a neutral intent
// once the slot has gone stale.
func liveIntent(slot *intentSlot, tick uint64) Intent {
	if !slot.has || tick-slot.tick > intentTTLTicks {
		return neutral
	}
	return slot.in
}

// Stop asks the loop to exit and blocks until it has. Safe to call
// multiple times and from multiple goroutines.
func (s *Sim) Stop() {
	s.stopOnce.Do(func() { close(s.done) })
	s.startMu.Lock()
	started := s.started
	s.startMu.Unlock()
	if started {
		s.wg.Wait()
	}
}

// Subscribe registers a snapshot consumer with a latest-value mailbox of
// capacity 1. Returns the read-only channel and a cancel function.
// After Stop, Subscribe returns an already-closed channel.
func (s *Sim) Subscribe() (<-chan Snapshot, func()) {
	ch := make(chan Snapshot, 1)
	select {
	case s.cmds <- subscribeReq{ch: ch}:
	case <-s.done:
		close(ch)
		return ch, func() {}
	}
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			select {
			case s.cmds <- unsubscribeReq{ch: ch}:
			case <-s.done:
			}
		})
	}
	return ch, cancel
}

// Ticks reports how many simulation ticks have been completed.
func (s *Sim) Ticks() uint64 { return s.ticks.Load() }

// Subscribers reports how many snapshot consumers are currently
// registered. Sessions uses it to avoid reclaiming a sim that a client
// is still streaming from.
func (s *Sim) Subscribers() int { return int(s.subs.Load()) }

// SubmitIntent records role's latest intent. It returns nil when the
// intent was accepted or coalesced away, ErrRateLimited when the role
// is over the contract's cap, ErrUnknownRole for a role the sim does not
// know, and ErrStopped when the loop is not running.
//
// It blocks until the loop has processed the intent, so a caller cannot
// observe its own intent half-applied.
func (s *Sim) SubmitIntent(role Role, in Intent) error {
	reply := make(chan error, 1)
	select {
	case s.cmds <- intentReq{role: role, in: in, reply: reply}:
	case <-s.done:
		return ErrStopped
	}
	select {
	case err := <-reply:
		return err
	case <-s.done:
		return ErrStopped
	}
}

// RoleOf reports the role player holds in the current round. It blocks
// until the loop has answered, so a caller cannot decode an intent for a
// role that a round boundary has already changed.
func (s *Sim) RoleOf(player Player) (Role, error) {
	reply := make(chan Role, 1)
	select {
	case s.cmds <- roleReq{player: player, reply: reply}:
	case <-s.done:
		return "", ErrStopped
	}
	select {
	case role := <-reply:
		return role, nil
	case <-s.done:
		return "", ErrStopped
	}
}

// Restart begins a fresh round after game over, rolling its roles and
// opening its waiting window. It is a no-op while the next round is
// already waiting for its own start, and ErrRoundInProgress while a round
// is still playing. Like SubmitIntent it waits for the loop, so it can
// report the outcome.
func (s *Sim) Restart() error {
	reply := make(chan error, 1)
	select {
	case s.cmds <- restartReq{reply: reply}:
	case <-s.done:
		return ErrStopped
	}
	select {
	case err := <-reply:
		return err
	case <-s.done:
		return ErrStopped
	}
}

// trySendLatest delivers snap without blocking: if the mailbox is full the
// stale value is dropped so the subscriber always sees the newest snapshot.
func trySendLatest(ch chan Snapshot, snap Snapshot) {
	select {
	case ch <- snap:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- snap:
		default:
		}
	}
}
