package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// Iron Maiden hurts the cursed unit when IT deals melee damage; a hero hitting the cursed monster takes nothing.
// Life Tap heals the attacker of a cursed unit (event functions 4 and 5, 0x5c3ec0 / 0x5c4180).
func TestIronMaidenNoLongerHurtsTheHeroHitter(t *testing.T) {
	e, p := defenseEngine()
	p.Stats = &d2hero.HeroStatsState{Health: 50, MaxHealth: 100, Mana: 10, MaxMana: 10}
	m := &d2mapentity.Monster{}
	e.setOf(m.ID()).Apply(0, d2state.Instance{Name: "ironmaiden", Mods: []d2state.StatMod{{Stat: d2state.StatIronMaiden, Value: 200}}})

	e.afterHit(m, p, 20)

	if p.Stats.Health != 50 {
		t.Errorf("hero life %d, want 50 (the hero hitting the cursed monster is not hurt)", p.Stats.Health)
	}
}

func TestLifeTapHealsAttacker(t *testing.T) {
	e, p := defenseEngine()
	p.Stats = &d2hero.HeroStatsState{Health: 50, MaxHealth: 100, Mana: 10, MaxMana: 10}
	m := &d2mapentity.Monster{}
	e.setOf(m.ID()).Apply(0, d2state.Instance{Name: "lifetap", Mods: []d2state.StatMod{{Stat: d2state.StatLifeTap, Value: 50}}})

	e.afterHit(m, p, 20)

	if p.Stats.Health != 60 {
		t.Errorf("hero life %d, want 60", p.Stats.Health)
	}

	e.afterHit(m, p, 400)

	if p.Stats.Health != 100 {
		t.Errorf("hero life %d, want the maximum 100", p.Stats.Health)
	}
}

// A cursed monster that lands a melee hit takes pct percent of it; missiles and an uncursed monster do not.
func TestIronMaidenBackOnMeleeOnly(t *testing.T) {
	e, p := defenseEngine()
	m := &d2mapentity.Monster{}
	m.Vitals.HP, m.Vitals.MaxHP = 100, 100
	e.monsters = &d2monsters.Director{}

	if got := e.ironMaidenBack(m, p, true, 10); got != 0 {
		t.Errorf("an uncursed monster took %d", got)
	}

	e.setOf(m.ID()).Apply(0, d2state.Instance{Name: "ironmaiden", Mods: []d2state.StatMod{{Stat: d2state.StatIronMaiden, Value: 225}}})

	if got := e.ironMaidenBack(m, p, false, 10); got != 0 {
		t.Errorf("a missile hit gave %d back", got)
	}

	if got := e.ironMaidenBack(m, p, true, 0); got != 0 {
		t.Errorf("a blocked hit gave %d back", got)
	}

	if got := e.ironMaidenBack(m, p, true, 10); got != 22 {
		t.Errorf("melee hit of 10 at 225 percent gave %d, want 22", got)
	}
}
