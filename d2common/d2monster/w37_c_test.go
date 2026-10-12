package d2monster

import "testing"

// 0x5fac60 counts the cold resist always, 0x5e7420 only in melee reach; otherwise the two sums are the same.
func TestW37CrabScoreColdAlways(t *testing.T) {
	st := DiaStats{S2B: 1500, LifePct: 100}

	if CrabScore(st, false, true, false) <= DiabloScore(st, false, true, false) {
		t.Error("crab must count cold out of melee reach")
	}

	if CrabScore(st, true, true, false) != DiabloScore(st, true, true, false) {
		t.Error("in melee reach both count cold")
	}

	if CrabScore(DiaStats{IsMonster: true, Threat: 1}, true, true, true) != 0 {
		t.Error("threat below 2 scores 0")
	}
}

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
