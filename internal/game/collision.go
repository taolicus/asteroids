// Collision helpers. The arena wraps, so distances and overlaps are
// measured across the seam; a naive difference would miss hits between
// objects on opposite edges.
package game

import "math"

// wrappedDelta is the shortest signed distance from b to a on an axis of
// length ArenaSize, in [-ArenaSize/2, ArenaSize/2).
func wrappedDelta(a, b float64) float64 {
	d := math.Mod(a-b, ArenaSize)
	switch {
	case d < -ArenaSize/2:
		return d + ArenaSize
	case d >= ArenaSize/2:
		return d - ArenaSize
	default:
		return d
	}
}

// hit reports whether two circles overlap, measuring across the seam.
func hit(a, b Vec2, radiusA, radiusB float64) bool {
	dx := wrappedDelta(a.X, b.X)
	dy := wrappedDelta(a.Y, b.Y)
	r := radiusA + radiusB
	return dx*dx+dy*dy <= r*r
}
