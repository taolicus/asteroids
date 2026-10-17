package game

import (
	"math"
	"reflect"
	"testing"
)

// testWorld returns a world with exactly the given rocks, so a test can
// place them precisely. The ship, lives, and round-start invulnerability
// are as newWorld leaves them unless the test overwrites them.
func testWorld(t *testing.T, seed int64, rocks ...Asteroid) *world {
	t.Helper()
	w := newWorld(seed, 0)
	w.rocks = rocks
	return w
}

func TestSnapshotCarriesAsteroidsAndProjectiles(t *testing.T) {
	w := newWorld(7, 0)
	if len(w.rocks) != AsteroidTuning.StartCount {
		t.Fatalf("opening field = %d rocks, want %d", len(w.rocks), AsteroidTuning.StartCount)
	}
	w.fire(1, Intent{Fire: true})

	snap := w.snapshot(1, 1)
	if snap.Lives != StartLives || snap.Score != 0 {
		t.Fatalf("lives/score = %d/%d, want %d/0", snap.Lives, snap.Score, StartLives)
	}
	if len(snap.Asteroids) != AsteroidTuning.StartCount {
		t.Fatalf("snapshot rocks = %d, want %d", len(snap.Asteroids), AsteroidTuning.StartCount)
	}
	if got := snap.Asteroids[0].Radius(); got != TierLarge.Radius() {
		t.Fatalf("snapshot rock radius = %v, want %v", got, TierLarge.Radius())
	}
	if len(snap.Projectiles) != 1 {
		t.Fatalf("snapshot shots = %d, want 1", len(snap.Projectiles))
	}
	if snap.Projectiles[0].ID == 0 {
		t.Fatal("snapshot projectile has no ID")
	}
}

func TestSnapshotDoesNotAliasTheWorld(t *testing.T) {
	w := newWorld(7, 0)
	w.fire(1, Intent{Fire: true})
	snap := w.snapshot(1, 1)

	if len(snap.Asteroids) == 0 || len(snap.Projectiles) == 0 {
		t.Fatal("need asteroids and projectiles to check aliasing")
	}
	if &snap.Asteroids[0] == &w.rocks[0] {
		t.Fatal("snapshot shares the live asteroid array")
	}
	if &snap.Projectiles[0] == &w.shots[0] {
		t.Fatal("snapshot shares the live projectile array")
	}

	// Stepping the world must not disturb an earlier snapshot.
	before := snap.Asteroids[0].Pos
	w.step(1, Intent{}, Intent{}, dt)
	if snap.Asteroids[0].Pos != before {
		t.Fatal("a snapshot changed when the world stepped")
	}
}

func TestWorldIsDeterministicForAGivenSeed(t *testing.T) {
	run := func(seed int64) Snapshot {
		w := newWorld(seed, 0)
		for tick := uint64(1); tick <= 150; tick++ {
			// Vary the intents so splits and shots both occur.
			pilot := Intent{Seq: 1, Rotate: 1, Thrust: tick%3 != 0}
			gunner := Intent{Seq: 1, Fire: tick%2 == 0}
			w.step(tick, pilot, gunner, dt)
		}
		return w.snapshot(1, 150)
	}

	a, b := run(99), run(99)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed produced different worlds")
	}
	if reflect.DeepEqual(a, run(100)) {
		t.Fatal("different seeds produced identical worlds")
	}
}

func TestShotSplitsRockAndScores(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 1000 // the ship must not take this rock's hit
	parent := Vec2{X: w.ship.Pos.X + 10, Y: w.ship.Pos.Y}
	w.rocks = []Asteroid{{ID: 1, Tier: TierLarge, Pos: parent}}

	w.step(1, Intent{}, Intent{Seq: 1, Fire: true}, dt)

	if w.score != TierLarge.Score() {
		t.Fatalf("score = %d, want %d", w.score, TierLarge.Score())
	}
	if len(w.shots) != 0 {
		t.Fatalf("shot survived its hit: %d remain", len(w.shots))
	}
	if len(w.rocks) != 2 {
		t.Fatalf("rocks = %d, want 2 children", len(w.rocks))
	}
	for _, r := range w.rocks {
		if r.Tier != TierMedium {
			t.Fatalf("child tier = %d, want medium", r.Tier)
		}
		if r.Pos != parent {
			t.Fatalf("child spawned at %v, want the parent's %v", r.Pos, parent)
		}
	}
}

// The gunner's turret is independent of the ship: the posted aim decides
// the shot direction, an absent aim follows the nose, and an aim beyond a
// full turn is folded rather than rejected.
func TestTurretAimIsIndependentOfTheHeading(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 1000

	quarter := math.Pi / 2
	w.step(1, Intent{}, Intent{Seq: 1, Fire: true, Aim: &quarter}, dt)

	if len(w.shots) != 1 {
		t.Fatalf("shots = %d, want 1", len(w.shots))
	}
	if got := w.shots[0].Vel; math.Abs(got.X) > 1e-9 || !approx(got.Y, WeaponTuning.Speed) {
		t.Fatalf("aim velocity = %v, want (0, %v)", got, WeaponTuning.Speed)
	}
	if snap := w.snapshot(1, 1); !approx(snap.Turret, quarter) {
		t.Fatalf("snapshot turret = %v, want %v", snap.Turret, quarter)
	}

	// Without an aim the turret follows the nose, which is what a stale
	// intent leaves behind.
	w.shots, w.fireReadyAt = nil, 0
	w.ship.Heading = math.Pi
	w.step(2, Intent{}, Intent{Seq: 2, Fire: true}, dt)

	if len(w.shots) != 1 {
		t.Fatalf("shots = %d, want 1", len(w.shots))
	}
	if got := w.shots[0].Vel; !approx(got.X, -WeaponTuning.Speed) || math.Abs(got.Y) > 1e-9 {
		t.Fatalf("nose velocity = %v, want (-%v, 0)", got, WeaponTuning.Speed)
	}
	if snap := w.snapshot(2, 2); !approx(snap.Turret, snap.Ship.Heading) {
		t.Fatalf("snapshot turret = %v, want the heading %v", snap.Turret, snap.Ship.Heading)
	}

	// An aim past a half turn folds into [-π, π) instead of drifting.
	w.shots, w.fireReadyAt = nil, 0
	wrapped := 3 * math.Pi / 2
	w.step(3, Intent{}, Intent{Seq: 3, Aim: &wrapped}, dt)

	if snap := w.snapshot(3, 3); !approx(snap.Turret, -math.Pi/2) {
		t.Fatalf("folded turret = %v, want %v", snap.Turret, -math.Pi/2)
	}
}

func TestSmallRockIsTerminalAndScoresFull(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 1000
	w.rocks = []Asteroid{{
		ID:   1,
		Tier: TierSmall,
		Pos:  Vec2{X: w.ship.Pos.X + 10, Y: w.ship.Pos.Y},
	}}

	w.step(1, Intent{}, Intent{Seq: 1, Fire: true}, dt)

	if w.score != TierSmall.Score() {
		t.Fatalf("score = %d, want %d", w.score, TierSmall.Score())
	}
	if len(w.rocks) != 0 {
		t.Fatalf("small rock was not destroyed: %d left", len(w.rocks))
	}
}

func TestShipHitCostsALifeAndRespawns(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 0 // vulnerable
	w.ship = Ship{Pos: Vec2{X: 100, Y: 100}, Vel: Vec2{X: 80, Y: 60}}
	w.rocks = []Asteroid{{ID: 1, Tier: TierLarge, Pos: Vec2{X: 100, Y: 100}}}

	w.step(1, Intent{}, Intent{}, dt)

	if w.lives != StartLives-1 {
		t.Fatalf("lives = %d, want %d", w.lives, StartLives-1)
	}
	if w.over {
		t.Fatal("the round ended before the lives ran out")
	}
	want := Vec2{X: ArenaSize / 2, Y: ArenaSize / 2}
	if w.ship.Pos != want || w.ship.Vel != (Vec2{}) {
		t.Fatalf("ship not respawned at rest: pos %v vel %v", w.ship.Pos, w.ship.Vel)
	}
	if w.invulnUntil != 1+respawnInvulnTicks {
		t.Fatalf("invulnUntil = %d, want %d", w.invulnUntil, 1+respawnInvulnTicks)
	}
	if len(w.rocks) != 1 {
		t.Fatalf("the rock should survive the ship hit: %d rocks", len(w.rocks))
	}
}

func TestInvulnerabilityPreventsLifeLoss(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 5
	w.rocks = []Asteroid{{ID: 1, Tier: TierLarge, Pos: w.ship.Pos}}

	for tick := uint64(1); tick < 5; tick++ {
		w.step(tick, Intent{}, Intent{}, dt)
	}
	if w.lives != StartLives {
		t.Fatalf("lost a life while invulnerable: %d lives", w.lives)
	}

	w.step(5, Intent{}, Intent{}, dt)
	if w.lives != StartLives-1 {
		t.Fatalf("hit not applied once invulnerability expired: %d lives", w.lives)
	}
}

func TestShipCollidesAcrossTheSeam(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 0
	w.ship = Ship{Pos: Vec2{X: 1, Y: 500}}
	w.rocks = []Asteroid{{ID: 1, Tier: TierLarge, Pos: Vec2{X: ArenaSize - 1, Y: 500}}}

	w.step(1, Intent{}, Intent{}, dt)

	if w.lives != StartLives-1 {
		t.Fatalf("seam collision missed: %d lives", w.lives)
	}
}

func TestLastLifeEndsTheRound(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 0
	w.lives = 1
	w.rocks = []Asteroid{{
		ID:   1,
		Tier: TierLarge,
		Pos:  w.ship.Pos,
		Vel:  Vec2{X: 30, Y: 0},
	}}

	w.step(1, Intent{}, Intent{}, dt)
	if !w.over || w.lives != 0 {
		t.Fatalf("round did not end on the last life: over=%v lives=%d", w.over, w.lives)
	}

	// The game-over screen freezes play: no further ship motion, score,
	// or survival time, though existing motion carries on.
	shipBefore := w.ship
	rockBefore := w.rocks[0].Pos
	survivalBefore := w.survival
	w.step(2, Intent{Seq: 1, Thrust: true}, Intent{Seq: 1, Fire: true}, dt)

	if w.ship != shipBefore {
		t.Fatal("the ship moved after the round ended")
	}
	if w.score != 0 {
		t.Fatal("scored after the round ended")
	}
	if w.survival != survivalBefore {
		t.Fatal("the survival timer ran on after the round ended")
	}
	if w.rocks[0].Pos == rockBefore {
		t.Fatal("asteroids stopped drifting after the round ended")
	}
}

func TestFiringRespectsCooldown(t *testing.T) {
	w := testWorld(t, 7)
	for tick := uint64(1); tick <= 20; tick++ {
		w.step(tick, Intent{}, Intent{Seq: 1, Fire: true}, dt)
	}

	// A 300 ms cooldown is 9 ticks: shots at ticks 1, 10, and 19.
	if len(w.shots) != 3 {
		t.Fatalf("shots after 20 ticks = %d, want 3", len(w.shots))
	}
}

func TestFiringStopsAtTheLiveProjectileCap(t *testing.T) {
	w := testWorld(t, 7)
	for i := 0; i < WeaponTuning.MaxLive; i++ {
		w.shots = append(w.shots, Projectile{ID: uint64(i + 1), ExpiresAt: 1 << 40})
	}

	w.fire(1, Intent{Fire: true})

	if len(w.shots) != WeaponTuning.MaxLive {
		t.Fatalf("fired past the cap: %d shots, want %d", len(w.shots), WeaponTuning.MaxLive)
	}
}

func TestProjectileLifespan(t *testing.T) {
	w := testWorld(t, 7)
	w.fire(1, Intent{Fire: true})
	if len(w.shots) != 1 {
		t.Fatalf("shots = %d, want 1", len(w.shots))
	}

	lifespan := secondsTicks(WeaponTuning.Lifespan)
	for tick := uint64(1); tick < 1+lifespan; tick++ {
		w.step(tick, Intent{}, Intent{}, dt)
	}
	if len(w.shots) != 1 {
		t.Fatalf("projectile died before its %d-tick lifespan", lifespan)
	}

	w.step(1+lifespan, Intent{}, Intent{}, dt)
	if len(w.shots) != 0 {
		t.Fatalf("projectile outlived its lifespan: %d left", len(w.shots))
	}
}

func TestNoFriendlyFire(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = 0 // so any ship hit would show
	for tick := uint64(1); tick <= 30; tick++ {
		w.step(tick, Intent{}, Intent{Seq: 1, Fire: true}, dt)
	}
	if w.lives != StartLives {
		t.Fatalf("the ship took its own projectiles: %d lives", w.lives)
	}
}

func TestSpawningFollowsTheCadenceAndCap(t *testing.T) {
	w := testWorld(t, 7)

	w.step(spawnIntervalTicks-1, Intent{}, Intent{}, dt)
	if len(w.rocks) != 0 {
		t.Fatalf("spawned before the 7 s cadence: %d rocks", len(w.rocks))
	}
	w.step(spawnIntervalTicks, Intent{}, Intent{}, dt)
	if len(w.rocks) != 1 {
		t.Fatalf("rocks = %d, want 1 on the cadence", len(w.rocks))
	}

	// At the cap a due spawn adds nothing.
	w.rocks = make([]Asteroid, AsteroidTuning.MaxCount)
	w.step(spawnIntervalTicks*10, Intent{}, Intent{}, dt)
	if len(w.rocks) != AsteroidTuning.MaxCount {
		t.Fatalf("rocks = %d, want the cap %d", len(w.rocks), AsteroidTuning.MaxCount)
	}
}

func TestSurvivalTimerCountsUp(t *testing.T) {
	w := testWorld(t, 7)
	for tick := uint64(1); tick <= TickRate; tick++ {
		w.step(tick, Intent{}, Intent{}, dt)
	}
	if math.Abs(w.survival-1) > 1e-9 {
		t.Fatalf("survival after a second = %v, want 1", w.survival)
	}
}

func TestInvulnRemainingIsReportedInSeconds(t *testing.T) {
	w := testWorld(t, 7)
	w.invulnUntil = respawnInvulnTicks
	if got := w.invulnRemaining(0); !approx(got, 2) {
		t.Fatalf("invulnRemaining(0) = %v, want 2", got)
	}
	if got := w.invulnRemaining(respawnInvulnTicks); got != 0 {
		t.Fatalf("invulnRemaining at expiry = %v, want 0", got)
	}
}
