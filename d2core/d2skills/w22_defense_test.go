package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

func defenseEngine() (*Engine, *d2mapentity.Player) {
	e := auraEngine(d2state.Defs{})
	e.Logger = d2util.NewLogger()
	p := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Mana: 100, MaxMana: 100}}

	return e, p
}

// Bone Armor soaks only the physical part: fire passes through untouched.
func TestDefensePartsBoneArmorPhysicalOnly(t *testing.T) {
	e, p := defenseEngine()
	e.setOf(p.ID()).Apply(0, d2state.Instance{Name: "bonearmor", Mods: []d2state.StatMod{{Stat: "bonearmor", Value: 30 << 8}}})

	hp := d2monsters.HitParts{Phys: 20, Fire: 15}
	e.heroDefenseParts(p, nil, false, &hp)

	if hp.Phys != 0 || hp.Fire != 15 {
		t.Fatalf("parts %+v", hp)
	}

	if got := e.setOf(p.ID()).Stat(0, "bonearmor"); got != 10<<8 {
		t.Errorf("pool left %d", got)
	}

	hp = d2monsters.HitParts{Phys: 25}
	e.heroDefenseParts(p, nil, false, &hp)

	if hp.Phys != 15 {
		t.Errorf("a pool of 10 should leave 15 of 25, got %d", hp.Phys)
	}
}

// Energy Shield walks the types in order and spends the mana left after the earlier ones.
func TestDefensePartsEnergyShieldWalk(t *testing.T) {
	e, p := defenseEngine()
	p.Stats.Mana = 4
	e.setOf(p.ID()).Apply(0, d2state.Instance{Name: "energyshield", Mods: []d2state.StatMod{
		{Stat: "x_energyshield_pct", Value: 100}, {Stat: "x_energyshield_ratio", Value: 16}}})

	// ratio 16: one mana per damage point; 4 mana absorb 4 physical, then nothing is left for fire
	hp := d2monsters.HitParts{Phys: 4, Fire: 10, Pois: 7}
	e.heroDefenseParts(p, nil, false, &hp)

	if hp.Phys != 0 || hp.Fire != 10 || hp.Pois != 7 {
		t.Fatalf("parts %+v", hp)
	}

	if p.Stats.Mana != 0 || e.setOf(p.ID()).Stat(0, "x_energyshield_pct") != 0 {
		t.Errorf("shield should end with the mana gone (mana %d)", p.Stats.Mana)
	}
}

// The missile cell stamp keeps the footprint for the frame it was made and one more.
func TestCellStampsFootprintAndExpiry(t *testing.T) {
	c := newCellStamps()
	c.mark(10, 10, 2, d2missile.MissileCellBit)

	if c.flags(11, 10) != d2missile.MissileCellBit || c.flags(11, 11) != 0 {
		t.Fatal("size 2 should stamp a plus")
	}

	c.roll()

	if c.flags(10, 10) == 0 {
		t.Error("the grace frame lost the stamp")
	}

	c.roll()

	if c.flags(10, 10) != 0 {
		t.Error("a stamp that was not refreshed must lapse")
	}

	var nilStamps *cellStamps
	nilStamps.mark(1, 1, 1, 1)
	nilStamps.roll()

	if nilStamps.flags(1, 1) != 0 {
		t.Error("nil stamps")
	}
}
