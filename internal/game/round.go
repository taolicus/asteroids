// Round flow: the phase machine that owns a room's round lifecycle.
//
// A room's sim plays a sequence of rounds. Each round opens in a short
// "waiting" window that rolls and announces the players' roles before
// play begins; play runs until the shared lives run out; the round then
// freezes in "over" until a player asks for a replay. Roles are rolled
// against the sim's two players, not the room's seats: the server binds
// player one to the owner seat and player two to the guest.
//
// The state here is touched only by the sim's loop goroutine, so it needs
// no locking, and it is a pure function of (seed, intents, ticks), so a
// fixed seed replays a whole sequence of rounds exactly.
package game

import (
	"errors"
	"math/rand"
)

// Phase is where a round stands. The names are the wire's, settled in
// tasks/closed/define-snapshot-format-and-interpolation.md.
type Phase string

const (
	// PhaseWaiting is a round's opening window: the roles are rolled and
	// announced, and the field is not yet live.
	PhaseWaiting Phase = "waiting"
	// PhasePlaying is a live round.
	PhasePlaying Phase = "playing"
	// PhaseOver is a round whose lives ran out. Its field keeps drifting
	// behind the game-over screen until a replay starts.
	PhaseOver Phase = "over"
)

// Player identifies one of the two players sharing the ship. The roll is
// expressed against players because the sim knows only that a room has
// two, not who they are: the server binds PlayerOne to the owner seat and
// PlayerTwo to the guest.
type Player int

const (
	PlayerOne Player = iota
	PlayerTwo
)

// roundStartTicks is the opening window (2 s) in which a round announces
// its roles before play begins, so both clients see them first.
const roundStartTicks = 2 * TickRate

// ErrRoundInProgress is returned by restart while the current round is
// still playing.
var ErrRoundInProgress = errors.New("round in progress")

// roundState is one room's round bookkeeping.
type roundState struct {
	rng    *rand.Rand
	number uint64 // the current round; 0 before the first starts
	phase  Phase
	pilot  Player
	seed   int64  // the pending round's world seed
	begins uint64 // the tick at which waiting gives way to play
	w      *world // nil until the round plays
}

// newRoundState starts the flow seeded with rng, waiting with no round
// yet. The caller opens the first round with startRound.
func newRoundState(rng *rand.Rand) *roundState {
	return &roundState{rng: rng, phase: PhaseWaiting}
}

// startRound rolls the next round's roles and world seed and opens its
// waiting window at tick.
func (r *roundState) startRound(tick uint64) {
	r.number++
	r.pilot = Player(r.rng.Intn(2))
	r.seed = r.rng.Int63()
	r.phase = PhaseWaiting
	r.begins = tick + roundStartTicks
	r.w = nil
}

// step advances the flow one tick. When the waiting window closes the
// round's world is created and stepped; a round that loses its last life
// falls into PhaseOver.
func (r *roundState) step(tick uint64, pilotIn, gunnerIn Intent, dt float64) {
	if r.phase == PhaseWaiting {
		if tick < r.begins {
			return
		}
		r.phase = PhasePlaying
		r.w = newWorld(r.seed, tick)
	}
	r.w.step(tick, pilotIn, gunnerIn, dt)
	if r.w.over {
		r.phase = PhaseOver
	}
}

// restart begins a fresh round after game over. It is a no-op while the
// next round is already waiting for its own start, so a replay press from
// a stale screen cannot skip a round, and ErrRoundInProgress while one is
// still playing.
func (r *roundState) restart(tick uint64) error {
	switch r.phase {
	case PhasePlaying:
		return ErrRoundInProgress
	case PhaseOver:
		r.startRound(tick)
	}
	return nil
}

// snapshot folds the round's number, phase, and roll into a view of its
// world. A round that is still waiting has no world yet, so it reports
// the round's starting values instead.
func (r *roundState) snapshot(seq, tick uint64) Snapshot {
	if r.w == nil {
		return Snapshot{
			Seq:   seq,
			Tick:  tick,
			Round: r.number,
			Phase: r.phase,
			Pilot: r.pilot,
			Lives: StartLives,
		}
	}
	snap := r.w.snapshot(seq, tick)
	snap.Round, snap.Phase, snap.Pilot = r.number, r.phase, r.pilot
	return snap
}
