package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// The Fallen Shaman group scan (w20): dead followers of the shaman itself, the
// last one of the walk wins, the count covers all, and the radius is the
// squared value compared with the edge distance.
func TestShamanCorpseScanGroupMode(t *testing.T) {
	self := scanUnit(1, 0, 0, "Fallen", 0)
	mine1 := corpseUnit(2, 3, 0, false)
	mine2 := corpseUnit(3, 0, 4, false)
	other := corpseUnit(4, 1, 0, false) // not led by the shaman
	alive := scanUnit(5, 2, 0, "Fallen", 0)
	d := testDirector(self, mine1, mine2, other, alive)
	d.units[1] = self

	for _, u := range []*unit{mine1, mine2, other} {
		u.b.Mode = d2monster.ModeDead
	}

	for _, u := range []*unit{mine1, mine2, alive} {
		u.b.Leader = self.b
	}

	got, n := d.ShamanCorpseScan(self.b, 100)
	if n != 2 || got.ID != corpseTargetBase+mine2.b.ID {
		t.Fatalf("n=%d target=%+v, want 2 followers and the last", n, got)
	}

	if _, n = d.ShamanCorpseScan(self.b, 1); n != 0 {
		t.Fatalf("tiny radius still found %d", n)
	}
}
