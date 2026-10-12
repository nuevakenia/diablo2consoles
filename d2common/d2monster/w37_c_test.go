package d2monster

import "testing"

// fbnTiles is UNIT_IsWithinTileDistance 0x5db310: no division by 5.
func TestW37FbnTilesNoDivision(t *testing.T) {
	b := &Brain{}
	b.X, b.Y = 10, 10

	if got := fbnTiles(b, 14, 10); got != 4 {
		t.Errorf("straight 4: got %d", got)
	}

	if got := fbnTiles(b, 14, 12); got != 5 {
		t.Errorf("4,2: got %d want 5", got)
	}

	if got := fbnTiles(b, 10, 10); got != 0 {
		t.Errorf("same: got %d", got)
	}

	if fbnTiles(b, 13, 7) != unitTileDist(b, 13, 7) {
		t.Error("fbnTiles must equal unitTileDist")
	}
}
