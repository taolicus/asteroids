package game

import (
	"math/rand"
	"reflect"
	"testing"
)

// newTestRound opens round one at tick 0, as the sim's loop does.
func newTestRound(seed int64) *roundState {
	r := newRoundState(rand.New(rand.NewSource(seed)))
	r.startRound(0)
	return r
}

// beginPlay steps a waiting round up to and including the tick at which
// its field goes live, and returns that tick.
func beginPlay(t *testing.T, r *roundState, from uint64) uint64 {
	t.Helper()
	for tick := from; ; tick++ {
		r.step(tick, Intent{}, Intent{}, dt)
		if r.phase == PhasePlaying {
			return tick
		}
		if tick > from+1000 {
			t.Fatalf("round never began playing: %q", r.phase)
		}
	}
}

func TestRoundOpensWaitingWithItsRolesRolled(t *testing.T) {
	r := newTestRound(7)
	if r.number != 1 || r.phase != PhaseWaiting {
		t.Fatalf("round/phase = %d/%q, want 1/%q", r.number, r.phase, PhaseWaiting)
	}
	if r.pilot != PlayerOne && r.pilot != PlayerTwo {
		t.Fatalf("pilot = %d, want a player", r.pilot)
	}
	if r.w != nil {
		t.Fatal("the field exists before the round plays")
	}

	// The waiting snapshot carries the roll and the round's starting HUD.
	snap := r.snapshot(1, 1)
	if snap.Phase != PhaseWaiting || snap.Round != 1 || snap.Pilot != r.pilot {
		t.Fatalf("waiting snapshot = %+v", snap)
	}
	if snap.Lives != StartLives || snap.Score != 0 || snap.Survival != 0 {
		t.Fatalf("waiting HUD = %d/%d/%v", snap.Lives, snap.Score, snap.Survival)
	}
	if snap.Ship != (Ship{}) || len(snap.Asteroids) != 0 {
		t.Fatalf("the field exists in the waiting snapshot: %+v", snap)
	}

	// Nothing happens a tick before the window closes.
	r.step(roundStartTicks-1, Intent{}, Intent{}, dt)
	if r.phase != PhaseWaiting || r.w != nil {
		t.Fatalf("play began early: phase %q", r.phase)
	}
}

func TestRoundBeginsPlayingWhenTheWindowCloses(t *testing.T) {
	r := newTestRound(7)
	tick := beginPlay(t, r, 1)
	if tick < roundStartTicks {
		t.Fatalf("play began at tick %d, before the window closed", tick)
	}
	snap := r.snapshot(1, tick)
	if snap.Phase != PhasePlaying || snap.Round != 1 {
		t.Fatalf("playing snapshot round/phase = %d/%q", snap.Round, snap.Phase)
	}
	if snap.Lives != StartLives || snap.Score != 0 {
		t.Fatalf("playing HUD = %d/%d, want %d/0", snap.Lives, snap.Score, StartLives)
	}
	if len(snap.Asteroids) != AsteroidTuning.StartCount {
		t.Fatalf("opening field = %d rocks, want %d", len(snap.Asteroids), AsteroidTuning.StartCount)
	}
	// Round-start invulnerability is timed from when play begins, not
	// from when the sim started.
	if snap.Invuln != 2 {
		t.Fatalf("round-start invuln = %v, want 2", snap.Invuln)
	}
}

func TestGameOverFreezesTheRoundAndReplayRestartsIt(t *testing.T) {
	r := newTestRound(7)
	tick := beginPlay(t, r, 1)
	firstSeed := r.seed

	// Force the last life: a rock on the ship, no invulnerability.
	r.w.invulnUntil = 0
	r.w.lives = 1
	r.w.rocks = []Asteroid{{ID: 1, Tier: TierLarge, Pos: r.w.ship.Pos}}
	tick++
	r.step(tick, Intent{}, Intent{}, dt)
	if r.phase != PhaseOver || r.w.lives != 0 {
		t.Fatalf("phase/lives = %q/%d, want over/0", r.phase, r.w.lives)
	}
	over := r.snapshot(1, tick)
	if over.Phase != PhaseOver || over.Round != 1 || over.Lives != 0 {
		t.Fatalf("over snapshot = %+v", over)
	}

	// The replay opens a fresh round, with its own roll and field.
	if err := r.restart(tick + 1); err != nil {
		t.Fatalf("restart = %v, want nil", err)
	}
	if r.number != 2 || r.phase != PhaseWaiting || r.w != nil {
		t.Fatalf("after replay: round=%d phase=%q world=%v", r.number, r.phase, r.w)
	}
	if r.seed == firstSeed {
		t.Fatal("the replay reused the round's world seed")
	}

	beginPlay(t, r, tick+2)
	if r.w.lives != StartLives || r.w.score != 0 {
		t.Fatalf("replay did not reset the field: lives=%d score=%d", r.w.lives, r.w.score)
	}
}

func TestReplayIsRefusedWhilePlaying(t *testing.T) {
	r := newTestRound(7)
	beginPlay(t, r, 1)
	if err := r.restart(roundStartTicks + 1); err != ErrRoundInProgress {
		t.Fatalf("replay while playing = %v, want %v", err, ErrRoundInProgress)
	}
	if r.number != 1 || r.phase != PhasePlaying {
		t.Fatalf("a refused replay changed the round: %d/%q", r.number, r.phase)
	}
}

func TestReplayWhileWaitingKeepsThePendingRound(t *testing.T) {
	r := newTestRound(7)
	number, pilot, seed := r.number, r.pilot, r.seed
	if err := r.restart(1); err != nil {
		t.Fatalf("replay while waiting = %v, want nil", err)
	}
	if r.number != number || r.pilot != pilot || r.seed != seed {
		t.Fatal("a replay press skipped or re-rolled the pending round")
	}
}

func TestRollsAreDeterministicPerSeedAndUseBothPlayers(t *testing.T) {
	rolls := func() []Player {
		r := newRoundState(rand.New(rand.NewSource(99)))
		out := make([]Player, 0, 100)
		for i := 0; i < 100; i++ {
			r.startRound(uint64(i) * 1000)
			out = append(out, r.pilot)
		}
		return out
	}

	a, b := rolls(), rolls()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed rolled different roles")
	}

	seen := map[Player]bool{}
	for _, p := range a {
		seen[p] = true
	}
	if !seen[PlayerOne] || !seen[PlayerTwo] {
		t.Fatalf("100 rolls used only %v", seen)
	}
}
