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

// 0x622e40: the water watcher parts count as reachable below meleeRange + 8 table distance.
func TestWithinMeleeRangeOfClassWaterWatcher(t *testing.T) {
	if WithinMeleeRange(6, 0, 1, 1) {
		t.Fatal("setup: 6 subtiles must be out of normal reach")
	}

	if !WithinMeleeRangeOfClass(6, 0, 1, 1, 0x102) || !WithinMeleeRangeOfClass(6, 0, 1, 1, 0x105) {
		t.Error("water watcher not reachable at 6")
	}

	if WithinMeleeRangeOfClass(6, 0, 1, 1, 7) || WithinMeleeRangeOfClass(30, 0, 1, 1, 0x102) {
		t.Error("ordinary class, or too far, reachable")
	}
}

// UNIT_GetTableDistance 0x642980: size 1 units reproduce TableDistance everywhere; larger units shorten the axes.
func TestTableDistanceSized(t *testing.T) {
	for dx := -12; dx <= 12; dx++ {
		for dy := -12; dy <= 12; dy++ {
			if a, b := TableDistance(dx, dy), TableDistanceSized(dx, dy, 1, 1); a != b {
				t.Fatalf("size 1 differs at %d,%d: %d vs %d", dx, dy, a, b)
			}
		}
	}

	// Two large (size 2) units get no +1 on the table path; a size 3 unit takes one off.
	if got := TableDistanceSized(4, 0, 2, 2); got != tableDistanceRows[0][4] {
		t.Errorf("size 2,2 at 4,0 = %d", got)
	}

	if got := TableDistanceSized(4, 0, 3, 1); got != tableDistanceRows[0][4] { // (2-1)+1
		t.Errorf("size 3,1 at 4,0 = %d", got)
	}

	// Beyond the table the half sizes (3/2 + 1/2 = 1) come off both axes before 2*max+min.
	if got := TableDistanceSized(10, 4, 3, 1); got != 21 { // axes 9 and 3: 3 + 2*9
		t.Errorf("size 3,1 at 10,4 = %d", got)
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
