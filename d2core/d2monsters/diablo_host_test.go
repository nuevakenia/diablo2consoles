package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

func TestDiabloCandidatesListLivingHeroes(t *testing.T) {
	mk := func(hp int) *d2mapentity.Player {
		return &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: hp, MaxHealth: 200}}
	}

	d := testDirector()
	d.opt.IgnoreTown = true
	d.targets = map[uint32]*d2mapentity.Player{1: mk(50), 2: mk(0), 3: mk(200)}

	var src d2monster.DiabloTargetSource = d

	got := src.DiabloCandidates(&d2monster.Brain{ID: 9})
	if len(got) != 2 || got[0].Target.ID != 1 || got[1].Target.ID != 3 {
		t.Fatalf("candidates: %+v", got)
	}

	if got[0].Stats.LifePct != 25 || got[1].Stats.LifePct != 100 || !got[0].Target.IsPlayer {
		t.Fatalf("stats: %+v", got)
	}
}
