package d2skill

import "testing"

func TestTableDistance(t *testing.T) {
	cases := []struct{ dx, dy, want int }{{0, 0, 0}, {2, 0, 0}, {3, 0, 1}, {0, 3, 1}, {1, 3, 2}, {4, 0, 3}, {-4, 1, 3}, {8, 0, 16}, {9, 2, 20}}
	for _, c := range cases {
		if got := TableDistance(c.dx, c.dy); got != c.want {
			t.Errorf("TableDistance(%d,%d)=%d want %d", c.dx, c.dy, got, c.want)
		}
	}
}

// With a rangeadder of 0 the exe counts 3 subtiles on an axis as in reach, 4 not: the old stored-target test
// (Chebyshev <= 4) was looser. A polearm (5) reaches further.
func TestWithinMeleeRange(t *testing.T) {
	if !WithinMeleeRange(3, 0, 0, 0) || WithinMeleeRange(4, 0, 0, 0) {
		t.Error("bare hands: 3 in, 4 out")
	}

	if !WithinMeleeRange(4, 0, 2, 0) {
		t.Error("range 2 reaches 4")
	}

	if WithinMeleeRange(8, 0, 5, 0) || WithinMeleeRange(7, 0, 6, 0) || !WithinMeleeRange(7, 0, 8, 0) {
		t.Error("polearm edge")
	}
}
