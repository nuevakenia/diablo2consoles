package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func mummyNeighbour(id uint32, x, y int, undeadLow, undeadHigh bool, hp, maxHP int) *unit {
	u := petUnit(id, x, y, "Melee", "")
	u.m.Stat = &d2records.MonStatRecord{}
	u.m.Stat.IsUndeadLow, u.m.Stat.IsUndeadHigh = undeadLow, undeadHigh
	u.m.Vitals.HP, u.m.Vitals.MaxHP = hp, maxHP
	u.m.Position.Set(float64(x), float64(y))

	return u
}

func TestMummyScanFilters(t *testing.T) {
	self := mummyNeighbour(1, 0, 0, true, false, 10, 10)
	d := testDirector(self,
		mummyNeighbour(2, 3, 0, true, false, 10, 10), // healthy low undead: counted only
		mummyNeighbour(3, 0, 4, false, false, 5, 10), // not undead: ignored
		mummyNeighbour(4, 50, 0, true, false, 5, 10), // outside the radius
	)

	d.units[1] = self

	sc := d.MummyScan(self.b, 10, false)
	if sc.InReach != 1 || sc.Wounded || sc.Corpse != nil {
		t.Fatalf("healthy neighbour only: %+v", sc)
	}

	d.nextID = 6
	d.units[5] = mummyNeighbour(5, 0, 6, true, false, 4, 10) // wounded low undead

	if sc = d.MummyScan(self.b, 10, false); sc.InReach != 2 || !sc.Wounded {
		t.Fatalf("wounded neighbour: %+v", sc)
	}

	d.units[6] = mummyNeighbour(6, 5, 5, false, true, 10, 10) // high undead: only Radament's filter takes it

	if sc = d.MummyScan(self.b, 10, false); sc.InReach != 2 {
		t.Fatalf("high undead must be ignored outside Radament: %+v", sc)
	}

	if sc = d.MummyScan(self.b, 10, true); sc.InReach != 3 {
		t.Fatalf("Radament counts high undead: %+v", sc)
	}

	if sc = d.MummyScan(self.b, 100, false); sc.InReach != 3 {
		t.Fatalf("wider radius reaches the far neighbour: %+v", sc)
	}
}

func TestUnitStateAndStat(t *testing.T) {
	u := petUnit(1, 0, 0, "Melee", "")
	d := testDirector(u)

	if d.HasState(u.b, 0xc) {
		t.Fatal("state starts down")
	}

	d.SetUnitState(u.b, 0xc, true)

	if !d.HasState(u.b, 0xc) {
		t.Fatal("state set")
	}

	d.SetUnitState(u.b, 0xc, false)

	if d.HasState(u.b, 0xc) {
		t.Fatal("state cleared")
	}

	if d.UnitStat(u.b, 0x4a) != 0 {
		t.Fatal("stat starts at 0")
	}

	d.SetUnitStat(u.b, 0x4a, 12)

	if d.UnitStat(u.b, 0x4a) != 12 {
		t.Fatal("stat stored")
	}
}

func TestWispBlessTimedMagicFind(t *testing.T) {
	p := &d2mapentity.Player{}
	d := testDirector()
	d.targets = map[uint32]*d2mapentity.Player{1: p}
	d.frame = 100

	d.WispBless(nil, d2monster.Target{ID: 1}, 0x50, 100, 1000)

	if p.Stats != nil {
		t.Fatal("a hero without stats takes no blessing")
	}

	if got := d.TimedHeroStats(p); got != nil {
		t.Fatalf("no blessing recorded: %v", got)
	}
}
