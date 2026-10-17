package game

import (
	"math"
	"math/rand"
	"testing"
)

func TestTierRules(t *testing.T) {
	cases := []struct {
		tier   Tier
		radius float64
		score  int
		child  Tier
		count  int
	}{
		{TierLarge, 40, 20, TierMedium, 2},
		{TierMedium, 24, 50, TierSmall, 2},
		{TierSmall, 14, 100, TierSmall, 0},
	}
	for _, tc := range cases {
		if got := tc.tier.Radius(); got != tc.radius {
			t.Errorf("tier %d radius = %v, want %v", tc.tier, got, tc.radius)
		}
		if got := tc.tier.Score(); got != tc.score {
			t.Errorf("tier %d score = %d, want %d", tc.tier, got, tc.score)
		}
		child, count := tc.tier.split()
		if child != tc.child || count != tc.count {
			t.Errorf("tier %d split = (%d, %d), want (%d, %d)",
				tc.tier, child, count, tc.child, tc.count)
		}
	}
}

func TestSplitSpeedAddsTheStepAndCaps(t *testing.T) {
	cases := []struct {
		parent, want float64
	}{
		{0, 30},
		{100, 130},
		{190, 200}, // 190 + 30 is over the cap
		{200, 200},
	}
	for _, tc := range cases {
		if got := splitSpeed(tc.parent); got != tc.want {
			t.Errorf("splitSpeed(%v) = %v, want %v", tc.parent, got, tc.want)
		}
	}
}

func TestOpeningFieldKeepsClearOfSpawn(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	nextID := uint64(1)
	rocks := openingField(rng, &nextID)

	if len(rocks) != AsteroidTuning.StartCount {
		t.Fatalf("opening field = %d rocks, want %d", len(rocks), AsteroidTuning.StartCount)
	}
	centre := Vec2{X: ArenaSize / 2, Y: ArenaSize / 2}
	for i, r := range rocks {
		if r.Tier != TierLarge {
			t.Errorf("rock %d tier = %d, want large", i, r.Tier)
		}
		dx := wrappedDelta(r.Pos.X, centre.X)
		dy := wrappedDelta(r.Pos.Y, centre.Y)
		if dist := math.Hypot(dx, dy); dist < AsteroidTuning.SpawnClearance {
			t.Errorf("rock %d is %v from centre, want >= %v",
				i, dist, AsteroidTuning.SpawnClearance)
		}
		if speed := math.Hypot(r.Vel.X, r.Vel.Y); !approx(speed, AsteroidTuning.SpawnSpeed) {
			t.Errorf("rock %d drifts at %v, want %v", i, speed, AsteroidTuning.SpawnSpeed)
		}
	}
}

func TestEdgeAsteroidEntersFromAnEdge(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for i := 0; i < 60; i++ {
		a := edgeAsteroid(rng, uint64(i+1))
		if a.Tier != TierLarge {
			t.Fatalf("edge asteroid %d tier = %d, want large", i, a.Tier)
		}
		inward := false
		switch {
		case a.Pos.X == 0:
			inward = a.Vel.X > 0
		case a.Pos.X == ArenaSize:
			inward = a.Vel.X < 0
		case a.Pos.Y == 0:
			inward = a.Vel.Y > 0
		case a.Pos.Y == ArenaSize:
			inward = a.Vel.Y < 0
		default:
			t.Fatalf("edge asteroid %d is not on an edge: %v", i, a.Pos)
		}
		if !inward {
			t.Errorf("edge asteroid %d at %v heads outward: vel %v", i, a.Pos, a.Vel)
		}
		if speed := math.Hypot(a.Vel.X, a.Vel.Y); !approx(speed, AsteroidTuning.SpawnSpeed) {
			t.Errorf("edge asteroid %d drifts at %v, want %v", i, speed, AsteroidTuning.SpawnSpeed)
		}
	}
}

func TestAsteroidDriftWraps(t *testing.T) {
	rocks := []Asteroid{{
		Pos: Vec2{X: ArenaSize - 1, Y: 500},
		Vel: Vec2{X: 60, Y: 0},
	}}
	stepAsteroids(rocks, 1)

	// 999 + 60 crosses the seam and comes back at 59.
	if rocks[0].Pos.X < 0 || rocks[0].Pos.X >= ArenaSize {
		t.Fatalf("wrapped position out of the arena: %v", rocks[0].Pos)
	}
	if !approx(rocks[0].Pos.X, ArenaSize-1+60-ArenaSize) {
		t.Fatalf("wrapped position = %v, want 59", rocks[0].Pos.X)
	}
}
