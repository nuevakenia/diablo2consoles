package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

func TestMephScanWeakestWithinRadius(t *testing.T) {
	mk := func(x, y float64, hp int) *d2mapentity.Player {
		p := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: hp, MaxHealth: 1000}}
		p.Position.Set(x, y)

		return p
	}

	d := testDirector()
	d.opt.IgnoreTown = true
	d.targets = map[uint32]*d2mapentity.Player{
		1: mk(10, 0, 500), 2: mk(20, 0, 100), 3: mk(40, 0, 10), // 3 is beyond 32 subtiles
		4: mk(0, 5, 0), // dead hero is skipped
	}

	b := &d2monster.Brain{ID: 9}
	w, n := d.MephScan(b)

	if w == nil || w.ID != 2 {
		t.Fatalf("weakest in range: %+v", w)
	}

	if n != nil {
		t.Fatal("a monster scanner has no nearest slot")
	}

	d.targets = map[uint32]*d2mapentity.Player{3: mk(40, 0, 10)}
	if w, _ = d.MephScan(b); w != nil {
		t.Fatal("nobody in range")
	}
}
