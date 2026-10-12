package d2monster

import "testing"

// MONAI_GetEdgeDistanceBetweenUnits 0x5db0d0: first unit's size off both axes, then max + min/2.
func TestW37EdgeDistance(t *testing.T) {
	cases := []struct{ dx, dy, size, want int }{
		{10, 4, 1, 9 + 3/2},
		{-10, -4, 1, 9 + 3/2},
		{1, 1, 1, 0},
		{3, 0, 2, 1},
		{0, 0, 3, 0},
	}

	for _, c := range cases {
		if got := EdgeDistance(c.dx, c.dy, c.size); got != c.want {
			t.Errorf("EdgeDistance(%d,%d,%d)=%d want %d", c.dx, c.dy, c.size, got, c.want)
		}
	}
}
