// The world is one round's mutable simulation state: the shared ship,
// the asteroid field, projectiles, lives, score, and the survival timer.
// It is owned by a single sim goroutine, so it needs no locking, and it
// is a pure function of (seed, intents, ticks), so a fixed seed replays
// exactly.
package game

import (
	"math"
	"math/rand"
)

// Round constants settled in tasks/closed/define-game-rules-and-tuning.md.
// Durations are in ticks at TickRate (30 Hz).
const (
	// StartLives is the shared life count at round start.
	StartLives = 3
	// respawnInvulnTicks is the 2 s invulnerability after a respawn, and
	// the same at round start.
	respawnInvulnTicks = 2 * TickRate
	// spawnIntervalTicks is the 7 s cadence for a new large asteroid.
	spawnIntervalTicks = 7 * TickRate
)

// world holds one round's state. Accessed only by the loop goroutine.
type world struct {
	rng   *rand.Rand
	ship  Ship
	rocks []Asteroid
	shots []Projectile

	lives       int
	score       int
	survival    float64
	invulnUntil uint64
	fireReadyAt uint64
	spawnAt     uint64
	over        bool

	nextRockID uint64
	nextShotID uint64
}

// newWorld starts a fresh round from seed: the opening asteroid field,
// full lives, and round-start invulnerability.
func newWorld(seed int64) *world {
	w := &world{
		rng:         rand.New(rand.NewSource(seed)),
		ship:        newShip(),
		lives:       StartLives,
		invulnUntil: respawnInvulnTicks,
		spawnAt:     spawnIntervalTicks,
		nextRockID:  1,
		nextShotID:  1,
	}
	w.rocks = openingField(w.rng, &w.nextRockID)
	return w
}

// step advances the world one tick. pilotIn drives the ship; gunnerIn
// drives fire. dt is the tick duration in seconds.
func (w *world) step(tick uint64, pilotIn, gunnerIn Intent, dt float64) {
	if w.over {
		// The round is over: nothing new happens — no spawns, no
		// fire, no hits — but existing motion continues so the
		// game-over screen has something behind it.
		stepAsteroids(w.rocks, dt)
		stepProjectiles(w.shots, dt)
		w.expireProjectiles(tick)
		return
	}

	w.survival += dt
	stepShip(&w.ship, pilotIn, dt)
	stepAsteroids(w.rocks, dt)
	w.spawn(tick)
	w.fire(tick, gunnerIn)
	stepProjectiles(w.shots, dt)
	w.resolveCollisions(tick)
	w.expireProjectiles(tick)
}

// spawn lets a new large asteroid in from an edge on the spawn cadence,
// while the field is below the cap.
func (w *world) spawn(tick uint64) {
	if tick < w.spawnAt {
		return
	}
	w.spawnAt = tick + spawnIntervalTicks
	if len(w.rocks) >= AsteroidTuning.MaxCount {
		return
	}
	w.rocks = append(w.rocks, edgeAsteroid(w.rng, w.nextRockID))
	w.nextRockID++
}

// fire spawns a shot when the gunner holds fire, respecting the cooldown
// and the live-projectile cap.
func (w *world) fire(tick uint64, in Intent) {
	if !in.Fire || tick < w.fireReadyAt || len(w.shots) >= WeaponTuning.MaxLive {
		return
	}
	w.fireReadyAt = tick + secondsTicks(WeaponTuning.Cooldown)
	w.shots = append(w.shots, newProjectile(w.nextShotID, w.ship, tick))
	w.nextShotID++
}

// resolveCollisions applies projectile–asteroid hits (scoring) and the
// ship–asteroid hit (a life lost).
func (w *world) resolveCollisions(tick uint64) {
	w.shotsVsRocks()
	if tick < w.invulnUntil {
		return
	}
	w.shipVsRocks(tick)
}

// shotsVsRocks destroys the first rock each shot overlaps, awards its
// score, and splits it. A shot is consumed by its hit.
func (w *world) shotsVsRocks() {
	kept := w.shots[:0]
	for _, shot := range w.shots {
		index := -1
		for i := range w.rocks {
			if hit(shot.Pos, w.rocks[i].Pos, 0, w.rocks[i].Radius()) {
				index = i
				break
			}
		}
		if index < 0 {
			kept = append(kept, shot)
			continue
		}
		w.score += w.rocks[index].Tier.Score()
		w.splitRock(index)
	}
	w.shots = kept
}

// splitRock removes the rock at index and replaces it with its children.
func (w *world) splitRock(index int) {
	parent := w.rocks[index]
	w.rocks = append(w.rocks[:index], w.rocks[index+1:]...)

	child, count := parent.Tier.split()
	if count == 0 {
		return
	}
	speed := splitSpeed(math.Hypot(parent.Vel.X, parent.Vel.Y))
	for i := 0; i < count; i++ {
		w.rocks = append(w.rocks, newAsteroid(w.rng, w.nextRockID, child, parent.Pos, speed))
		w.nextRockID++
	}
}

// shipVsRocks costs a life on the first overlapping rock. The ship
// respawns at the centre with invulnerability, or the round ends.
func (w *world) shipVsRocks(tick uint64) {
	for i := range w.rocks {
		if !hit(w.ship.Pos, w.rocks[i].Pos, ShipTuning.Radius, w.rocks[i].Radius()) {
			continue
		}
		w.loseLife(tick)
		return
	}
}

func (w *world) loseLife(tick uint64) {
	w.lives--
	if w.lives <= 0 {
		w.over = true
		return
	}
	w.ship = newShip()
	w.invulnUntil = tick + respawnInvulnTicks
}

// expireProjectiles drops shots that have reached their lifespan.
func (w *world) expireProjectiles(tick uint64) {
	kept := w.shots[:0]
	for _, shot := range w.shots {
		if tick < shot.ExpiresAt {
			kept = append(kept, shot)
		}
	}
	w.shots = kept
}

// snapshot copies the world into an immutable value. Slices are copied
// because snapshots are read on other goroutines.
func (w *world) snapshot(seq, tick uint64) Snapshot {
	snap := Snapshot{
		Seq:      seq,
		Tick:     tick,
		Ship:     w.ship,
		Lives:    w.lives,
		Score:    w.score,
		Survival: w.survival,
		Invuln:   w.invulnRemaining(tick),
	}
	if len(w.rocks) > 0 {
		snap.Asteroids = append([]Asteroid(nil), w.rocks...)
	}
	if len(w.shots) > 0 {
		snap.Projectiles = append([]Projectile(nil), w.shots...)
	}
	return snap
}

// invulnRemaining is the ship's remaining invulnerability in seconds.
func (w *world) invulnRemaining(tick uint64) float64 {
	if tick >= w.invulnUntil {
		return 0
	}
	return float64(w.invulnUntil-tick) / TickRate
}
