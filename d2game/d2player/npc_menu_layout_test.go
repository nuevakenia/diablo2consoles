package d2player

import "testing"

// Golden numbers computed by hand from the original's formulas (UI_CreateDialog 0x4b47b0 / 0x4b4d40): width = widest
// text + 20 (given), height = 0x15 + 0xf*rows + 15, centred on the anchor, top = anchor - 0x15, clamped 10 px from the
// sides and the top and 0x3a from the bottom (then top = 600 - height - 0x30).
func TestNPCMenuBoundsGolden(t *testing.T) {
	tests := []struct {
		name                     string
		ax, ay, w, rows          int
		left, top, right, bottom int
	}{
		{"centre", 400, 200, 100, 3, 350, 179, 450, 260},
		{"left clamp", 20, 200, 100, 3, 10, 179, 110, 260},
		{"right clamp", 790, 200, 100, 3, 690, 179, 790, 260},
		{"top clamp", 400, 5, 100, 3, 350, 10, 450, 91},
		{"bottom clamp", 400, 560, 100, 3, 350, 600 - 81 - 0x30, 450, 600 - 0x30},
	}

	for _, c := range tests {
		l, tp, r, b := NPCMenuBounds(c.ax, c.ay, c.w, c.rows)
		if l != c.left || tp != c.top || r != c.right || b != c.bottom {
			t.Errorf("%s: got %d %d %d %d want %d %d %d %d", c.name, l, tp, r, b, c.left, c.top, c.right, c.bottom)
		}
	}

	if got := NPCMenuRowBottom(179, 0); got != 179+0x15+0xf {
		t.Errorf("row bottom %d", got)
	}
}
