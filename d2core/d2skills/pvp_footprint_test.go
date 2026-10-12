package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// TestTargetsWithinListsRivals: Size 2 and 3 missiles (Blizzard shards) find their targets through
// TargetsWithin (the footprint test, commit 47c9a80c). It once listed only monsters, so a hostile hero standing in
// a Blizzard was never hit (scenario 9f-pvp-skills-ear). It has to list the same heroes as Targets does.
func TestTargetsWithinListsRivals(t *testing.T) {
	foe := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: 100}}
	foe.SetPositionSubtile(10, 10)

	e := newTestEngine()
	e.monsters = &d2monsters.Director{}
	e.Rivals = func() []*d2mapentity.Player { return []*d2mapentity.Player{foe} }

	w := &world{e: e}

	for _, tc := range []struct {
		name string
		x, y int
		want int
	}{
		{"on the hero", 10, 10, 1},
		{"two cells away", 12, 9, 1},
		{"three cells away", 13, 10, 0},
	} {
		if got := len(w.TargetsWithin(tc.x, tc.y, 2)); got != tc.want {
			t.Errorf("%s: TargetsWithin = %d targets, want %d", tc.name, got, tc.want)
		}
	}

	// the same hero is what Targets finds on its own cell
	if got := len(w.Targets(10, 10)); got != 1 {
		t.Errorf("Targets on the hero's cell = %d, want 1", got)
	}
}
