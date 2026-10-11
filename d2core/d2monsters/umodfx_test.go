package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monreg"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

var fxStock = []int{20, 100, 75, 50, 200, 150, 100, 300, 200, 100, 75, 100, 50, 100, 75, 150, 0, 33, 33, 0, 50, 50, 33, 33, 33, 50, 50, 50, 66, 66, 66, 100, 100, 100}

func fxTestMonster() *d2mapentity.Monster {
	st := &d2records.MonStatRecord{}
	st.SpeedBase = 6
	st.ResistanceFireNormal = 10

	m := &d2mapentity.Monster{Stat: st}
	m.Vitals = d2mapentity.MonsterVitals{Level: 30, HP: 400, MaxHP: 400, Defense: 100, Experience: 200}
	m.Vitals.A1 = d2mapentity.MonsterAttack{ToHit: 100, Min: 10, Max: 20}
	m.TypeFlags = d2mapentity.MonTypeUnique

	return m
}

func fxTestCtx() *d2monreg.FxCtx {
	return &d2monreg.FxCtx{
		Expansion: true, LvlCount: 100, ChampionDamageBonus: 90,
		Const:     func(r int) int { return fxStock[r] },
		LvlDamage: func(row int, _ bool, _ int) int { return 10 * row },
	}
}

// A rare with Fire Enchanted and Extra Fast on Normal: +3 levels, five times the experience, the unique life bonus
// (300 percent), fire damage per hit, fire resistance +75 and extra speed.
func TestApplyRareFireFast(t *testing.T) {
	m := fxTestMonster()
	m.Modifiers = []int{9, 6}
	u := (&Director{}).newFxUnit(m)

	fxTestCtx().ApplyInit(u, m.Modifiers, nil)
	u.finish()

	if m.Vitals.Level != 33 || m.Vitals.Experience != 1000 {
		t.Errorf("level %d xp %d", m.Vitals.Level, m.Vitals.Experience)
	}

	if m.Vitals.MaxHP != 1600 || m.Vitals.HP != 1600 {
		t.Errorf("life %d/%d", m.Vitals.HP, m.Vitals.MaxHP)
	}

	// level 33 damage 330: leader rows 28 / 31 = 66 and 100 percent
	if len(m.UMod.Elems) != 1 || m.UMod.Elems[0] != (d2mapentity.UModElem{Type: "fire", Min: 217, Max: 330}) {
		t.Errorf("elements %+v", m.UMod.Elems)
	}

	if got := MonsterResistsOf(m); got[2] != 10+75 {
		t.Errorf("fire resist %d", got[2])
	}

	// velocity 6: 2048/6 - 128 = 213, capped at 100
	if m.UMod.SpeedPct != 100 {
		t.Errorf("speed %d", m.UMod.SpeedPct)
	}
}

// A champion with Berserk: a quarter of the life and 270 percent more damage and to-hit; its minion only gets the
// base ids.
func TestApplyChampionBerserk(t *testing.T) {
	m := fxTestMonster()
	m.TypeFlags = d2mapentity.MonTypeChampion
	minion := fxTestMonster()
	minion.TypeFlags = d2mapentity.MonTypeChampion | d2mapentity.MonTypeMinion

	lu, mu := (&Director{}).newFxUnit(m), (&Director{}).newFxUnit(minion)

	fxTestCtx().ApplyInit(lu, []int{39}, []d2monreg.FxUnit{mu})
	lu.finish()
	mu.finish()

	// champion life row 4 (200 percent) then berserk -75: 400 * 3 = 1200 * 25 percent
	if m.Vitals.MaxHP != 300 {
		t.Errorf("berserk life %d", m.Vitals.MaxHP)
	}

	// 90 * 300 / 100 = 270 percent
	if m.Vitals.A1.Max != 20*370/100 || m.Vitals.A1.ToHit != 100*370/100 {
		t.Errorf("berserk attack %+v", m.Vitals.A1)
	}

	// the member: minion life row 1 (100 percent), +3 levels
	if minion.Vitals.MaxHP != 800 || minion.Vitals.Level != 33 || len(minion.UMod.Elems) != 0 {
		t.Errorf("minion %+v", minion.Vitals)
	}
}
