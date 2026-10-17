package game

import (
	"math"
	"testing"
)

func TestSecondsTicksTracksTheSettledDurations(t *testing.T) {
	cases := []struct {
		seconds float64
		want    uint64
	}{
		{WeaponTuning.Lifespan, 45}, // 1.5 s
		{WeaponTuning.Cooldown, 9},  // 300 ms, and 0.3*30 rounds down without Round
		{2, respawnInvulnTicks},
		{7, spawnIntervalTicks},
	}
	for _, tc := range cases {
		if got := secondsTicks(tc.seconds); got != tc.want {
			t.Errorf("secondsTicks(%v) = %d, want %d", tc.seconds, got, tc.want)
		}
	}
}

func TestNewProjectileFollowsTheHeading(t *testing.T) {
	ship := Ship{Pos: Vec2{X: 100, Y: 200}}
	p := newProjectile(3, ship, 10)

	if p.ID != 3 {
		t.Errorf("ID = %d, want 3", p.ID)
	}
	if p.Pos != ship.Pos {
		t.Errorf("spawn pos = %v, want %v", p.Pos, ship.Pos)
	}
	if !approx(p.Vel.X, WeaponTuning.Speed) || math.Abs(p.Vel.Y) > 1e-9 {
		t.Errorf("heading 0 velocity = %v, want (%v, 0)", p.Vel, WeaponTuning.Speed)
	}
	if want := uint64(10) + secondsTicks(WeaponTuning.Lifespan); p.ExpiresAt != want {
		t.Errorf("ExpiresAt = %d, want %d", p.ExpiresAt, want)
	}

	ship.Heading = math.Pi
	p = newProjectile(4, ship, 0)
	if !approx(p.Vel.X, -WeaponTuning.Speed) || math.Abs(p.Vel.Y) > 1e-9 {
		t.Errorf("heading pi velocity = %v, want (-%v, 0)", p.Vel, WeaponTuning.Speed)
	}
}

func TestProjectileMotionWraps(t *testing.T) {
	shots := []Projectile{{
		Pos: Vec2{X: ArenaSize - 1, Y: 500},
		Vel: Vec2{X: WeaponTuning.Speed, Y: 0},
	}}
	stepProjectiles(shots, dt)

	if shots[0].Pos.X < 0 || shots[0].Pos.X >= ArenaSize {
		t.Fatalf("wrapped position out of the arena: %v", shots[0].Pos)
	}
	want := ArenaSize - 1 + WeaponTuning.Speed*dt - ArenaSize
	if !approx(shots[0].Pos.X, want) {
		t.Fatalf("wrapped position = %v, want %v", shots[0].Pos.X, want)
	}
}
