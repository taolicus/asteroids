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

func TestNewProjectileTravelsAlongItsAngle(t *testing.T) {
	pos := Vec2{X: 100, Y: 200}
	p := newProjectile(3, pos, 0, 10)

	if p.ID != 3 {
		t.Errorf("ID = %d, want 3", p.ID)
	}
	if p.Pos != pos {
		t.Errorf("spawn pos = %v, want %v", p.Pos, pos)
	}
	if !approx(p.Vel.X, WeaponTuning.Speed) || math.Abs(p.Vel.Y) > 1e-9 {
		t.Errorf("angle 0 velocity = %v, want (%v, 0)", p.Vel, WeaponTuning.Speed)
	}
	if want := uint64(10) + secondsTicks(WeaponTuning.Lifespan); p.ExpiresAt != want {
		t.Errorf("ExpiresAt = %d, want %d", p.ExpiresAt, want)
	}

	p = newProjectile(4, pos, math.Pi, 0)
	if !approx(p.Vel.X, -WeaponTuning.Speed) || math.Abs(p.Vel.Y) > 1e-9 {
		t.Errorf("angle pi velocity = %v, want (-%v, 0)", p.Vel, WeaponTuning.Speed)
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
