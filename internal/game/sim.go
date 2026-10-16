// Package game hosts the authoritative per-room simulation loop.
package game

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// TickRate is the fixed simulation rate per room.
const TickRate = 30

// BroadcastEvery is how many ticks pass between snapshot broadcasts.
// TickRate/BroadcastEvery = 15 Hz, within the README's 10–15 Hz target.
const BroadcastEvery = 2

// Snapshot is one immutable view of room state produced by the loop.
// Later tasks extend it with ship, asteroid, and round fields.
type Snapshot struct {
	Seq  uint64
	Tick uint64
}

// ErrStopped is returned by Subscribe after the loop has stopped.
var ErrStopped = errors.New("sim stopped")

// Sim runs a fixed-timestep loop on a single goroutine and fans snapshots
// out to subscribers without ever blocking on them.
type Sim struct {
	cmds     chan any
	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	startMu sync.Mutex
	started bool

	ticks atomic.Uint64
}

type subscribeReq struct {
	ch chan Snapshot
}

type unsubscribeReq struct {
	ch chan Snapshot
}

// NewSim creates a stopped sim. Call Run to start the loop.
func NewSim() *Sim {
	return &Sim{
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
	subscribers := make(map[chan Snapshot]struct{})

	for {
		select {
		case <-s.done:
			for ch := range subscribers {
				close(ch)
			}
			return
		case cmd := <-s.cmds:
			switch c := cmd.(type) {
			case subscribeReq:
				subscribers[c.ch] = struct{}{}
			case unsubscribeReq:
				if _, ok := subscribers[c.ch]; ok {
					delete(subscribers, c.ch)
					close(c.ch)
				}
			}
		case <-ticker.C:
			tick++
			s.ticks.Store(tick)
			if tick%BroadcastEvery != 0 {
				continue
			}
			seq++
			snap := Snapshot{Seq: seq, Tick: tick}
			for ch := range subscribers {
				trySendLatest(ch, snap)
			}
		}
	}
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
