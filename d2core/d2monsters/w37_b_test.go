package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// attackReach: melee reach 7 is the verified value; ranged is the engine's distance.
func TestW37AttackReach(t *testing.T) {
	if attackReach(false) != meleeInRange || attackReach(true) != rangedInRange {
		t.Fatal("attackReach mismatch")
	}
	if meleeInRange != 7 {
		t.Fatalf("meleeInRange = %d", meleeInRange)
	}
}

// A second blessing stacks (MONAI_Think_WillOWisp 0x5f2970 allocates a stat list per dance, VERIFIED w37).
func TestW37WispBlessStacks(t *testing.T) {
	p := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{}}
	d := testDirector()
	d.targets = map[uint32]*d2mapentity.Player{1: p}
	d.frame = 100

	d.WispBless(nil, d2monster.Target{ID: 1}, 0x50, 50, 1000)
	d.WispBless(nil, d2monster.Target{ID: 1}, 0x50, 50, 2000)

	if got := d.TimedHeroStats(p)["item_magicbonus"]; got != 100 {
		t.Fatalf("stacked magic find = %d", got)
	}

	d.frame = 1200 // the first list has expired, the second has not

	if got := d.TimedHeroStats(p)["item_magicbonus"]; got != 50 {
		t.Fatalf("after the first expired = %d", got)
	}
}
