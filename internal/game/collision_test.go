package game

import "testing"

func TestWrappedDeltaIsTheShortestSignedDistance(t *testing.T) {
	cases := []struct {
		a, b, want float64
	}{
		{5, 995, 10},
		{995, 5, -10},
		{123, 456, -333},
		{500, 500, 0},
	}
	for _, tc := range cases {
		if got := wrappedDelta(tc.a, tc.b); !approx(got, tc.want) {
			t.Errorf("wrappedDelta(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestHitMeasuresAcrossTheSeam(t *testing.T) {
	near := Vec2{X: 1, Y: 500}
	far := Vec2{X: ArenaSize - 1, Y: 500} // 2 units away across the seam
	if !hit(near, far, ShipTuning.Radius, TierLarge.Radius()) {
		t.Fatal("missed a hit across the seam")
	}
	if hit(Vec2{X: 0, Y: 0}, Vec2{X: 500, Y: 500}, ShipTuning.Radius, TierLarge.Radius()) {
		t.Fatal("reported a hit between distant circles")
	}
	// Overlap exactly at the sum of the radii, and not just past it.
	if !hit(Vec2{X: 0, Y: 0}, Vec2{X: 50, Y: 0}, 10, 40) {
		t.Fatal("missed touching circles")
	}
	if hit(Vec2{X: 0, Y: 0}, Vec2{X: 50.001, Y: 0}, 10, 40) {
		t.Fatal("reported a hit between separated circles")
	}
}
