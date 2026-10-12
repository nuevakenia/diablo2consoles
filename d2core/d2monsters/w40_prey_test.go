package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

func TestNearestPreyPlayers(t *testing.T) {
	mk := func(x, hp int) *d2mapentity.Player {
		p := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: hp, MaxHealth: 1000}}
		p.Position.Set(float64(x), 0)

		return p
	}

	d := testDirector()
	d.targets = map[uint32]*d2mapentity.Player{1: mk(5, 900), 2: mk(8, 100), 3: mk(3, 0), 4: mk(30, 10)}
	b := &d2monster.Brain{ID: 9}

	// radius 11: player 2 is wounded (100 <= 10*40) and in range; 1 is healthy; 3 is dead; 4 is out of range
	got, ok := d.NearestPrey(b, 11, 40)
	if !ok || got.ID != 2 {
		t.Fatalf("prey = %+v %v", got, ok)
	}

	if _, ok := d.NearestPrey(b, 11, 5); ok { // 100 > 10*5
		t.Fatal("pct bound ignored")
	}
}
