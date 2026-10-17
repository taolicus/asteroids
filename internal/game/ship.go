// Ship and arena physics for the authoritative simulation.
//
// The physics is a pure function of (ship, intent, dt) so it can be
// tested without real time and without the loop. Decisions live in
// tasks/closed/define-game-rules-and-tuning.md and
// tasks/closed/define-input-intent-contract.md.
package game

import "math"

// ArenaSize is the side of the square wraparound arena, in sim units.
// Clients scale it to the viewport; the server always simulates this.
const ArenaSize = 1000

// ShipTuning groups the ship constants so they are easy to find and
// adjust. Values are initial; there is no settled tuning document for
// the ship yet, so treat them as a starting point.
var ShipTuning = struct {
	// TurnRate is the rotation speed, radians per second.
	TurnRate float64
	// Thrust is the acceleration along the heading, units per second².
	Thrust float64
	// Damping is the exponential velocity decay, per second. Light, so
	// the ship coasts and keeps arcade momentum.
	Damping float64
	// MaxSpeed caps the ship's speed, units per second.
	MaxSpeed float64
}{
	TurnRate: 4.0,
	Thrust:   320,
	Damping:  0.6,
	MaxSpeed: 480,
}

// Vec2 is a 2D position or velocity in sim units.
type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Ship is the shared ship's kinematic state. Both players steer this one
// ship. Heading is in radians, 0 pointing along +X.
type Ship struct {
	Pos     Vec2    `json:"pos"`
	Vel     Vec2    `json:"vel"`
	Heading float64 `json:"heading"`
}

// newShip returns the ship at its spawn state: arena centre, no motion,
// heading along +X.
func newShip() Ship {
	return Ship{Pos: Vec2{X: ArenaSize / 2, Y: ArenaSize / 2}}
}

// stepShip advances the ship by dt seconds under intent. It is pure and
// deterministic: the same arguments always produce the same result, and
// the ship only changes when something asks it to, so there is no
// drift or jitter between ticks.
func stepShip(sh *Ship, intent Intent, dt float64) {
	sh.Heading = wrapAngle(sh.Heading + float64(intent.Rotate)*ShipTuning.TurnRate*dt)

	if intent.Thrust {
		sh.Vel.X += math.Cos(sh.Heading) * ShipTuning.Thrust * dt
		sh.Vel.Y += math.Sin(sh.Heading) * ShipTuning.Thrust * dt
	}

	// Exponential decay, applied as a per-tick factor so it stays
	// stable at any dt.
	decay := 1 - ShipTuning.Damping*dt
	if decay < 0 {
		decay = 0
	}
	sh.Vel.X *= decay
	sh.Vel.Y *= decay

	if speed := math.Hypot(sh.Vel.X, sh.Vel.Y); speed > ShipTuning.MaxSpeed {
		scale := ShipTuning.MaxSpeed / speed
		sh.Vel.X *= scale
		sh.Vel.Y *= scale
	}

	sh.Pos.X = wrap(sh.Pos.X + sh.Vel.X*dt)
	sh.Pos.Y = wrap(sh.Pos.Y + sh.Vel.Y*dt)
}

// wrap folds a coordinate into [0, ArenaSize) so the arena has no edges.
func wrap(v float64) float64 {
	w := math.Mod(v, ArenaSize)
	if w < 0 {
		w += ArenaSize
	}
	return w
}

// wrapAngle folds an angle into [-π, π).
func wrapAngle(a float64) float64 {
	w := math.Mod(a+math.Pi, 2*math.Pi)
	if w < 0 {
		w += 2 * math.Pi
	}
	return w - math.Pi
}
