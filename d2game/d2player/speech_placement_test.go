package d2player

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
)

// Golden numbers of the subtitle box in column space; on screen it is centred on the column, so the screen centre
// and the distance to the bottom edge follow from the display model.
func TestSpeechPlacementGolden(t *testing.T) {
	x, y, w, h := SpeechPlacement(3)
	if x != 140 || y != 382 || w != 520 || h != 88 {
		t.Fatalf("3 lines: %d %d %d %d", x, y, w, h)
	}

	for _, sz := range []d2display.Size{{W: 800, H: 600}, {W: 1512, H: 982}, {W: 1920, H: 1080}, {W: 3440, H: 1440}, {W: 5120, H: 1440}} {
		ox, oy := d2display.Origin(sz, d2display.AnchorBottom)
		sx, sy := ox+x, oy+y

		if sx+w/2 != sz.W/2 {
			t.Errorf("%v: box centre %d, screen centre %d", sz, sx+w/2, sz.W/2)
		}

		if got := sz.H - (sy + h); got != 130 {
			t.Errorf("%v: gap to the bottom edge %d, want 130", sz, got)
		}
	}
}
