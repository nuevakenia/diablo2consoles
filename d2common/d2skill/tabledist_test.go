package d2skill

import "testing"

func TestTableDistanceAndMeleeRange(t *testing.T) {
	tests := []struct{ dx, dy, want int }{
		{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 0, 1}, {0, 3, 1}, {-4, 0, 3}, {3, 1, 2}, {2, 2, 1}, {7, 7, 12}, {9, 0, 18}, {0, 10, 20}, {9, 3, 21},
	}
	for _, tc := range tests {
		if got := tableDistance(tc.dx, tc.dy); got != tc.want {
			t.Errorf("tableDistance(%d,%d) = %d, want %d", tc.dx, tc.dy, got, tc.want)
		}
	}

	// reach with bare hands is table distance <= 1: three subtiles straight, not the old four
	if !withinMeleeRange(3, 0, 0, 0) || withinMeleeRange(4, 0, 0, 0) || withinMeleeRange(3, 1, 0, 0) {
		t.Error("melee range 0 reach wrong")
	}

	// a polearm (rangeadder 2) and the attack outcome's extra 1 widen it
	if !withinMeleeRange(4, 0, 2, 0) || !withinMeleeRange(3, 1, 0, 1) {
		t.Error("rangeadder / extra reach wrong")
	}
}
