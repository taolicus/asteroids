// Ship weapons: projectiles fired by the gunner. Rules and tuning live
// in tasks/closed/define-game-rules-and-tuning.md.
package game

import "math"

// WeaponTuning groups the settled weapon constants.
var WeaponTuning = struct {
	// Speed is the projectile speed along the ship heading, units per
	// second.
	Speed float64
	// Lifespan is how long a projectile lives, in seconds.
	Lifespan float64
	// MaxLive caps simultaneous projectiles.
	MaxLive int
	// Cooldown is the minimum time between shots, in seconds.
	Cooldown float64
}{
	Speed:    500,
	Lifespan: 1.5,
	MaxLive:  6,
	Cooldown: 0.3,
}

// Projectile is one shot. It travels along the turret's aim, which is
// the gunner's independent direction and falls back to the ship's
// heading. Tuning is unaffected: only the direction differs.
type Projectile struct {
	ID  uint64
	Pos Vec2
	Vel Vec2
	// ExpiresAt is the tick at which the shot disappears.
	ExpiresAt uint64
}

// newProjectile spawns a shot at the ship, travelling along angle. Ticks
// are back-converted from the settled seconds.
func newProjectile(id uint64, pos Vec2, angle float64, tick uint64) Projectile {
	return Projectile{
		ID:  id,
		Pos: pos,
		Vel: Vec2{
			X: math.Cos(angle) * WeaponTuning.Speed,
			Y: math.Sin(angle) * WeaponTuning.Speed,
		},
		ExpiresAt: tick + secondsTicks(WeaponTuning.Lifespan),
	}
}

// stepProjectiles advances every shot by dt seconds and wraps it.
func stepProjectiles(shots []Projectile, dt float64) {
	for i := range shots {
		shots[i].Pos.X = wrap(shots[i].Pos.X + shots[i].Vel.X*dt)
		shots[i].Pos.Y = wrap(shots[i].Pos.Y + shots[i].Vel.Y*dt)
	}
}

// secondsTicks converts a tuned duration in seconds to whole ticks. The
// rounding keeps a value like 0.3 s from truncating to one tick short.
func secondsTicks(seconds float64) uint64 {
	return uint64(math.Round(seconds * TickRate))
}
