package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2summon"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// The Clay Golem slows the monster that hits it in melee (wave 23, event function 27).
func TestClayGolemSlowsMeleeAttacker(t *testing.T) {
	g := golemUnit(1, golemOwner(500), 0, 0, 300, 300)
	g.ally.opt.Stats = &d2summon.Stats{Other: map[string]int{"item_slow": 40}}
	foe := petUnit(2, 5, 5, "NecroPet", "minion")
	d := testDirector(g, foe)

	var got []int

	d.SummonSlow = func(_ *d2mapentity.Monster, pct int) { got = append(got, pct) }

	d.slowMeleeAttacker(foe, g, 10, true)  // a missile
	d.slowMeleeAttacker(foe, g, 0, false)  // blocked
	d.slowMeleeAttacker(g, foe, 10, false) // the victim is no golem

	if len(got) != 0 {
		t.Fatalf("slow without a landed melee blow on a golem: %v", got)
	}

	d.slowMeleeAttacker(foe, g, 10, false)

	if len(got) != 1 || got[0] != 40 {
		t.Errorf("slow %v, want [40]", got)
	}
}
