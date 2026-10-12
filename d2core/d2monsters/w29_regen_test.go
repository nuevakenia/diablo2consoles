package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// State 0x34 (preventheal) stops the life regeneration tick (MONSTER_LifeRegenEventTick 0x5a4340).
func TestRegenBlockedByPreventHeal(t *testing.T) {
	d := &Director{}
	m := &d2mapentity.Monster{}
	m.Vitals.HP, m.Vitals.MaxHP = 50, 100
	u := &unit{m: m, stats: map[int]int{statHPRegen: 256}, states: map[int]bool{0x34: true}}

	d.regenStep(u)

	if m.Vitals.HP != 50 {
		t.Fatalf("regen ran under state 0x34: hp %d", m.Vitals.HP)
	}

	delete(u.states, 0x34)
	d.regenStep(u)

	if m.Vitals.HP != 51 {
		t.Fatalf("regen must resume without the state: hp %d", m.Vitals.HP)
	}
}
