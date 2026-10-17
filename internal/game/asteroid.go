// Asteroid field: tiers, spawn placement, and splitting. Rules and
// tuning live in tasks/closed/define-game-rules-and-tuning.md.
package game

import (
	"math"
	"math/rand"
)

// Tier is an asteroid size class. There are 3 tiers; large splits into
// medium, medium into small, and small is terminal.
type Tier int

const (
	TierLarge Tier = iota
	TierMedium
	TierSmall
)

// tierTable holds each tier's settled radius and score, indexed by Tier
// so the two can never drift apart.
var tierTable = [3]struct {
	radius float64
	score  int
}{
	TierLarge:  {radius: 40, score: 20},
	TierMedium: {radius: 24, score: 50},
	TierSmall:  {radius: 14, score: 100},
}

// Radius is the tier's collision radius.
func (t Tier) Radius() float64 { return tierTable[t].radius }

// Score is awarded for destroying an asteroid of this tier.
func (t Tier) Score() int { return tierTable[t].score }

// split reports what destroying this tier produces: two of the next
// smaller tier, or nothing for the terminal small tier.
func (t Tier) split() (child Tier, count int) {
	switch t {
	case TierLarge:
		return TierMedium, 2
	case TierMedium:
		return TierSmall, 2
	default:
		return t, 0
	}
}

// Asteroid is one drifting rock. Radius and score follow its Tier, so
// they cannot disagree with the rules.
type Asteroid struct {
	ID   uint64
	Tier Tier
	Pos  Vec2
	Vel  Vec2
}

// Radius is the asteroid's collision radius.
func (a Asteroid) Radius() float64 { return a.Tier.Radius() }

// AsteroidTuning groups the field constants. The tier radii and scores
// above are settled rules; the values here are initial, because the
// settled rules fix only the split increment and the cap.
var AsteroidTuning = struct {
	// SpawnSpeed is the drift speed of a newly spawned large asteroid,
	// units per second.
	SpawnSpeed float64
	// SplitSpeedStep is added to a parent's speed for each child.
	SplitSpeedStep float64
	// MaxSpeed caps an asteroid's drift.
	MaxSpeed float64
	// StartCount is the number of large asteroids at round start.
	StartCount int
	// MaxCount caps the live asteroid count; spawning pauses at it.
	MaxCount int
	// SpawnClearance is the minimum distance from the ship spawn point
	// for the opening field.
	SpawnClearance float64
}{
	SpawnSpeed:     50,
	SplitSpeedStep: 30,
	MaxSpeed:       200,
	StartCount:     5,
	MaxCount:       18,
	SpawnClearance: 200,
}

// newAsteroid builds a rock of tier at pos, drifting in a random
// direction at speed.
func newAsteroid(rng *rand.Rand, id uint64, tier Tier, pos Vec2, speed float64) Asteroid {
	dir := rng.Float64() * 2 * math.Pi
	return Asteroid{
		ID:   id,
		Tier: tier,
		Pos:  pos,
		Vel:  Vec2{X: math.Cos(dir) * speed, Y: math.Sin(dir) * speed},
	}
}

// openingField returns the round-start field: StartCount large asteroids
// placed at random points at least SpawnClearance from the arena centre.
func openingField(rng *rand.Rand, nextID *uint64) []Asteroid {
	centre := Vec2{X: ArenaSize / 2, Y: ArenaSize / 2}
	rocks := make([]Asteroid, 0, AsteroidTuning.StartCount)
	for i := 0; i < AsteroidTuning.StartCount; i++ {
		pos := clearPoint(rng, centre, AsteroidTuning.SpawnClearance)
		rocks = append(rocks, newAsteroid(rng, *nextID, TierLarge, pos, AsteroidTuning.SpawnSpeed))
		*nextID++
	}
	return rocks
}

// clearPoint returns a uniform random point at least clearance from
// centre, measured across the wraparound seam.
func clearPoint(rng *rand.Rand, centre Vec2, clearance float64) Vec2 {
	for {
		p := Vec2{X: rng.Float64() * ArenaSize, Y: rng.Float64() * ArenaSize}
		dx := wrappedDelta(p.X, centre.X)
		dy := wrappedDelta(p.Y, centre.Y)
		if dx*dx+dy*dy >= clearance*clearance {
			return p
		}
	}
}

// edgeAsteroid returns a large asteroid entering from a random edge,
// aimed inward within ±45° of the edge normal.
func edgeAsteroid(rng *rand.Rand, id uint64) Asteroid {
	const spread = math.Pi / 4

	t := rng.Float64() * ArenaSize
	jitter := (rng.Float64()*2 - 1) * spread

	var pos Vec2
	var angle float64
	switch rng.Intn(4) {
	case 0: // top edge, heading down
		pos, angle = Vec2{X: t, Y: 0}, math.Pi/2+jitter
	case 1: // bottom edge, heading up
		pos, angle = Vec2{X: t, Y: ArenaSize}, -math.Pi/2+jitter
	case 2: // left edge, heading right
		pos, angle = Vec2{X: 0, Y: t}, jitter
	default: // right edge, heading left
		pos, angle = Vec2{X: ArenaSize, Y: t}, math.Pi+jitter
	}
	return Asteroid{
		ID:   id,
		Tier: TierLarge,
		Pos:  pos,
		Vel:  Vec2{X: math.Cos(angle) * AsteroidTuning.SpawnSpeed, Y: math.Sin(angle) * AsteroidTuning.SpawnSpeed},
	}
}

// stepAsteroids drifts every rock by dt seconds and wraps it.
func stepAsteroids(rocks []Asteroid, dt float64) {
	for i := range rocks {
		rocks[i].Pos.X = wrap(rocks[i].Pos.X + rocks[i].Vel.X*dt)
		rocks[i].Pos.Y = wrap(rocks[i].Pos.Y + rocks[i].Vel.Y*dt)
	}
}

// splitSpeed is the speed of a split child: the parent's speed plus a
// fixed step, capped.
func splitSpeed(parentSpeed float64) float64 {
	speed := parentSpeed + AsteroidTuning.SplitSpeedStep
	if speed > AsteroidTuning.MaxSpeed {
		speed = AsteroidTuning.MaxSpeed
	}
	return speed
}
