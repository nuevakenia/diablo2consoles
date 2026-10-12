package d2skill

import "testing"

// UNIT_GetTableDistance 0x642980 with unit sizes (VERIFIED w37).
func TestTableDistanceSized(t *testing.T) {
	cases := []struct{ dx, dy, sa, sb, want int }{
		{3, 0, 1, 1, TableDistance(3, 0)}, // size 1 equals the old table
		{3, 0, 2, 1, TableDistance(3, 0)}, // one size below 2 keeps the +1
		{3, 0, 2, 2, TableDistance(3, 0) - 1},
		{4, 0, 2, 3, TableDistance(4, 0) - 2}, // size 3: minus 1 and no +1
		{0, 0, 2, 2, 0},
		{9, 0, 2, 4, 12}, // far or size 4: each axis loses sa/2+sb/2 = 3, then 2*max+min
		{9, 3, 2, 4, 12},
	}

	for _, c := range cases {
		if got := TableDistanceSized(c.dx, c.dy, c.sa, c.sb); got != c.want {
			t.Errorf("%v: got %d", c, got)
		}
	}

	// a big boss is reachable from further than a size 1 monster
	if AttackInReachSized(false, 0, 4, 0, HeroUnitSize, 1) || !AttackInReachSized(false, 0, 4, 0, HeroUnitSize, 3) {
		t.Error("size 3 must reach further than size 1")
	}
}
