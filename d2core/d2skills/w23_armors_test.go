package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// The retaliation helpers do nothing without the state, without a melee hit
// or without physical damage (handler 0x5c8e20 returns for a physical part
// below one).
func TestFrozenArmorRetaliationGates(t *testing.T) {
	e, p := defenseEngine()
	m := &d2mapentity.Monster{}

	if got := e.frozenArmorRetaliation(p, m, true, 5); got != "" {
		t.Fatalf("no state: %q", got)
	}

	e.setOf(p.ID()).Apply(0, d2state.Instance{Name: "frozenarmor", SkillID: 40, Level: 1})

	if got := e.frozenArmorRetaliation(p, m, false, 5); got != "" {
		t.Fatalf("missile hit: %q", got)
	}

	if got := e.frozenArmorRetaliation(p, m, true, 0); got != "" {
		t.Fatalf("no physical part: %q", got)
	}

	if got := e.frozenArmorRetaliation(p, nil, true, 5); got != "" {
		t.Fatalf("no attacker: %q", got)
	}
}

func TestHeroHitByMissileNeedsChillingArmor(t *testing.T) {
	e, p := defenseEngine()
	// no state, no pipeline: must return before touching either
	e.heroHitByMissile(p, &d2mapentity.Monster{}, "firebolt")
}
