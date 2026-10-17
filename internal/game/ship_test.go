package game

import (
	"math"
	"testing"
)

const dt = 1.0 / TickRate

func TestThrustAcceleratesAlongHeading(t *testing.T) {
	sh := newShip()
	for i := 0; i < 5; i++ {
		stepShip(&sh, Intent{Thrust: true}, dt)
	}

	if sh.Vel.X <= 0 {
		t.Fatalf("thrust along heading 0 gave vel.X = %v, want > 0", sh.Vel.X)
	}
	if math.Abs(sh.Vel.Y) > 1e-9 {
		t.Fatalf("thrust along heading 0 drifted on Y: vel.Y = %v", sh.Vel.Y)
	}
	if sh.Pos.X <= ArenaSize/2 {
		t.Fatalf("ship did not advance along +X: pos = %v", sh.Pos)
	}
	if sh.Pos.Y != ArenaSize/2 {
		t.Fatalf("ship drifted off axis: pos = %v", sh.Pos)
	}
}

func TestThrustFollowsHeading(t *testing.T) {
	// Point the ship along +Y and thrust: acceleration must follow.
	sh := newShip()
	sh.Heading = math.Pi / 2
	for i := 0; i < 5; i++ {
		stepShip(&sh, Intent{Thrust: true}, dt)
	}
	if sh.Vel.Y <= 0 || math.Abs(sh.Vel.X) > 1e-6 {
		t.Fatalf("thrust did not follow heading: vel = %v", sh.Vel)
	}
}

func TestRotationUpdatesHeading(t *testing.T) {
	sh := newShip()
	stepShip(&sh, Intent{Rotate: 1}, dt)
	want := ShipTuning.TurnRate * dt
	if math.Abs(sh.Heading-want) > 1e-12 {
		t.Fatalf("heading after one clockwise tick = %v, want %v", sh.Heading, want)
	}

	sh = newShip()
	stepShip(&sh, Intent{Rotate: -1}, dt)
	if math.Abs(sh.Heading+want) > 1e-12 {
		t.Fatalf("heading after one counter-clockwise tick = %v, want %v", sh.Heading, -want)
	}
}

func TestSteerTurnsTowardTheDirection(t *testing.T) {
	steer := math.Pi / 2
	sh := newShip()
	stepShip(&sh, Intent{Steer: &steer}, dt)
	// Straight ahead is +X, so a quarter turn clockwise is the short way.
	want := ShipTuning.TurnRate * dt
	if !approx(sh.Heading, want) {
		t.Fatalf("heading after one steer tick = %v, want %v", sh.Heading, want)
	}
}

func TestSteerRestsOnTheDirectionWithoutOvershooting(t *testing.T) {
	steer := 2.0
	sh := newShip()
	sh.Heading = -1.0
	for i := 0; i < 100; i++ {
		stepShip(&sh, Intent{Steer: &steer}, dt)
		if sh.Heading > steer+1e-9 {
			t.Fatalf("steer overshot: heading %v past steer %v", sh.Heading, steer)
		}
	}
	if !approx(sh.Heading, steer) {
		t.Fatalf("heading settled at %v, want %v", sh.Heading, steer)
	}

	// Aligned is not a limit cycle: further ticks hold the heading.
	for i := 0; i < 10; i++ {
		stepShip(&sh, Intent{Steer: &steer}, dt)
	}
	if !approx(sh.Heading, steer) {
		t.Fatalf("heading chattered off the steer: %v", sh.Heading)
	}
}

func TestSteerTakesTheShortestWayAcrossTheSeam(t *testing.T) {
	steer := -3.0
	sh := newShip()
	sh.Heading = 3.0
	// The short way from 3.0 to -3.0 crosses ±π, so the heading must
	// never pass through the middle of the arena's angle space.
	for i := 0; i < 3; i++ {
		stepShip(&sh, Intent{Steer: &steer}, dt)
		if math.Abs(sh.Heading) < 2.5 {
			t.Fatalf("steer took the long way round: heading %v", sh.Heading)
		}
	}
	if !approx(sh.Heading, steer) {
		t.Fatalf("heading settled at %v, want %v", sh.Heading, steer)
	}
}

func TestSteerFoldsFarAngles(t *testing.T) {
	steer := 7.0 // more than a full turn
	sh := newShip()
	for i := 0; i < 20; i++ {
		stepShip(&sh, Intent{Steer: &steer}, dt)
	}
	if !approx(sh.Heading, wrapAngle(steer)) {
		t.Fatalf("heading settled at %v, want %v", sh.Heading, wrapAngle(steer))
	}
}

func TestSteerWinsOverTheAxis(t *testing.T) {
	steer := 1.0
	sh := newShip()
	stepShip(&sh, Intent{Rotate: -1, Steer: &steer}, dt)
	if sh.Heading <= 0 {
		t.Fatalf("the axis beat the steer: heading %v", sh.Heading)
	}
}

func TestHeadingWraps(t *testing.T) {
	sh := newShip()
	sh.Heading = math.Pi - 0.01
	stepShip(&sh, Intent{Rotate: 1}, dt)
	if sh.Heading < -math.Pi || sh.Heading >= math.Pi {
		t.Fatalf("heading out of [-π, π): %v", sh.Heading)
	}
	// Turning past +π re-enters from the negative side, which is the
	// same direction.
	want := math.Pi - 0.01 + ShipTuning.TurnRate*dt - 2*math.Pi
	if !approx(sh.Heading, want) {
		t.Fatalf("heading = %v, want %v", sh.Heading, want)
	}
}

func TestVelocityDecaysWithoutThrust(t *testing.T) {
	sh := newShip()
	sh.Vel = Vec2{X: 100}
	prev := math.Hypot(sh.Vel.X, sh.Vel.Y)
	for i := 0; i < 10; i++ {
		stepShip(&sh, neutral, dt)
		speed := math.Hypot(sh.Vel.X, sh.Vel.Y)
		if speed >= prev {
			t.Fatalf("speed did not decay: %v then %v", prev, speed)
		}
		prev = speed
	}
}

func TestSpeedIsCapped(t *testing.T) {
	sh := newShip()
	sh.Vel = Vec2{X: ShipTuning.MaxSpeed * 10}
	stepShip(&sh, Intent{Thrust: true}, dt)
	if speed := math.Hypot(sh.Vel.X, sh.Vel.Y); speed > ShipTuning.MaxSpeed+1e-9 {
		t.Fatalf("speed %v exceeds cap %v", speed, ShipTuning.MaxSpeed)
	}
}

func TestWraparound(t *testing.T) {
	// Velocity decays before integration, so the step distance is
	// vel * decay * dt.
	adv := 100 * (1 - ShipTuning.Damping*dt) * dt

	cases := []struct {
		name string
		pos  Vec2
		vel  Vec2
		want Vec2
	}{
		{"right edge", Vec2{X: 999, Y: 500}, Vec2{X: 100}, Vec2{X: 999 + adv - ArenaSize, Y: 500}},
		{"left edge", Vec2{X: 1, Y: 500}, Vec2{X: -100}, Vec2{X: ArenaSize + 1 - adv, Y: 500}},
		{"bottom edge", Vec2{X: 500, Y: 999}, Vec2{Y: 100}, Vec2{X: 500, Y: 999 + adv - ArenaSize}},
		{"top edge", Vec2{X: 500, Y: 1}, Vec2{Y: -100}, Vec2{X: 500, Y: ArenaSize + 1 - adv}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sh := Ship{Pos: tc.pos, Vel: tc.vel}
			stepShip(&sh, neutral, dt)
			if !approx(sh.Pos.X, tc.want.X) || !approx(sh.Pos.Y, tc.want.Y) {
				t.Fatalf("wrapped to %v, want %v", sh.Pos, tc.want)
			}
			if sh.Pos.X < 0 || sh.Pos.X >= ArenaSize || sh.Pos.Y < 0 || sh.Pos.Y >= ArenaSize {
				t.Fatalf("position outside arena: %v", sh.Pos)
			}
		})
	}
}

func TestSteppingIsDeterministic(t *testing.T) {
	// Two identical runs must agree exactly, and a ship with no intent
	// must not drift: there is no jitter between tick boundaries.
	a, b := newShip(), newShip()
	for i := 0; i < 50; i++ {
		in := Intent{Rotate: 1, Thrust: i%2 == 0}
		stepShip(&a, in, dt)
		stepShip(&b, in, dt)
	}
	if a != b {
		t.Fatalf("identical runs diverged:\n%+v\n%+v", a, b)
	}

	rest := newShip()
	for i := 0; i < 50; i++ {
		stepShip(&rest, neutral, dt)
	}
	if rest != newShip() {
		t.Fatalf("idle ship drifted: %+v", rest)
	}
}

func approx(got, want float64) bool { return math.Abs(got-want) < 1e-9 }
