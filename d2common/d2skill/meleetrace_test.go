package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
)

func TestMeleeLineClear(t *testing.T) {
	open := d2path.NewCellGrid(0, 0, 40, 40)

	if !MeleeLineClear(open, MeleeTraceMask, 5, 5, 2, 9, 5, 1) {
		t.Fatal("open ground must be clear")
	}

	// a wall cell between the two units on a row, a column and both diagonals
	for _, c := range []struct{ x1, y1, x2, y2, wx, wy int }{
		{5, 5, 11, 5, 8, 5},
		{5, 5, 5, 12, 5, 8},
		{5, 5, 12, 12, 8, 8},
		{12, 12, 5, 5, 8, 8},
	} {
		g := d2path.NewCellGrid(0, 0, 40, 40)
		g.Cells[c.wy*g.W+c.wx] = d2path.FlagWall

		if MeleeLineClear(g, MeleeTraceMask, c.x1, c.y1, 1, c.x2, c.y2, 1) {
			t.Errorf("wall at (%d,%d) not seen on %+v", c.wx, c.wy, c)
		}
	}

	// a door blocks too, a corpse or a unit footprint does not
	g2 := d2path.NewCellGrid(0, 0, 40, 40)
	g2.Cells[5*g2.W+8] = d2path.FlagDoor

	if MeleeLineClear(g2, MeleeTraceMask, 5, 5, 1, 11, 5, 1) {
		t.Error("door must block")
	}

	g2.Cells[5*g2.W+8] = d2path.FlagMonster | d2path.FlagCorpse | d2path.FlagPlayer
	if !MeleeLineClear(g2, MeleeTraceMask, 5, 5, 1, 11, 5, 1) {
		t.Error("unit footprints are not in the mask")
	}

	g3 := d2path.NewCellGrid(0, 0, 40, 40)
	g3.Cells[5*g3.W+5] = d2path.FlagWall

	if !MeleeLineClear(g3, MeleeTraceMask, 5, 5, 2, 6, 5, 2) {
		t.Error("units overlapping their sizes are clear")
	}

	if !MeleeLineClear(g3, MeleeTraceMask, 5, 5, 2, 10, 5, 1) {
		t.Error("the attacker's own cell is skipped by the size pull")
	}

	if MeleeLineClear(g3, MeleeTraceMask, 5, 5, 0, 10, 5, 0) {
		t.Error("a size 0 start cell is tested")
	}

	if MeleeLineClear(g3, MeleeTraceMask, 38, 5, 1, 44, 5, 1) {
		t.Error("out of grid must block")
	}
}
