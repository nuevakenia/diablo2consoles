package d2skill

import "testing"

// The exe tables 0x6e2580 / 0x6e2680 (read wave 23): the first quarter of the x column and the y column shifted
// by a quarter turn.
func TestNovaRingOffsetsMatchTable(t *testing.T) {
	xs := []int{30, 29, 29, 28, 27, 26, 24, 23, 21, 19, 16, 14, 11, 8, 5, 2, 0, -2, -5, -8, -11, -14, -16, -19, -21,
		-23, -24, -26, -27, -28, -29, -29, -30}
	ys := []int{0, 2, 5, 8, 11, 14, 16, 19, 21, 23, 24, 26, 27, 28, 29, 29, 30, 29, 29, 28, 27, 26, 24, 23, 21, 19, 16,
		14, 11, 8, 5, 2, 0}

	for i := range xs {
		if dx, dy := NovaRingOffset(i); dx != xs[i] || dy != ys[i] {
			t.Errorf("entry %d: (%d, %d), want (%d, %d)", i, dx, dy, xs[i], ys[i])
		}
	}

	// the last entries: (-29, -2) ... (29, -2)
	if dx, dy := NovaRingOffset(48); dx != 0 || dy != -30 {
		t.Errorf("entry 48: (%d, %d)", dx, dy)
	}

	if dx, dy := NovaRingOffset(63); dx != 29 || dy != -2 {
		t.Errorf("entry 63: (%d, %d)", dx, dy)
	}
}
