package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// AuraBoost is what a friendly aura (the druid totems, aurafilter 0x10103) puts on a summoned minion: damage and
// attack rating percentages and a life percentage. The skill engine refreshes it every pulse and clears it (zero
// value) when the minion leaves the aura.
type AuraBoost struct {
	DamagePct, ToHitPct, MaxHPPct int
}

// SetAuraBoost sets the aura boost of a minion. The life percent scales the maximum life from the value the
// minion had before any boost (current life keeps its ratio). It reports false when m is not a minion.
func (d *Director) SetAuraBoost(m *d2mapentity.Monster, b AuraBoost) bool {
	u := d.byEntity[m.ID()]
	if u == nil || u.ally == nil {
		return false
	}

	a := u.ally
	if a.boost == b {
		return true
	}

	if a.baseMaxHP == 0 {
		a.baseMaxHP = m.Vitals.MaxHP
	}

	a.boost = b

	max := a.baseMaxHP + a.baseMaxHP*b.MaxHPPct/100
	if max < 1 {
		max = 1
	}

	if old := m.Vitals.MaxHP; old > 0 && old != max {
		hp := m.Vitals.HP * max / old
		if hp < 1 && m.Vitals.HP > 0 {
			hp = 1
		}

		m.Vitals.HP = hp
	}

	m.Vitals.MaxHP = max

	return true
}

// AuraBoostOf returns the boost a minion currently has.
func (d *Director) AuraBoostOf(m *d2mapentity.Monster) AuraBoost {
	if u := d.byEntity[m.ID()]; u != nil && u.ally != nil {
		return u.ally.boost
	}

	return AuraBoost{}
}

// petCasterFlag is the "casting pet" flag of the NecroPet think (MONAI_Think_NecroPet 0x5e3c30, VERIFIED: a
// non-zero flag in the AI record picks the caster think MONAI_Pet_ThinkCaster 0x5e3a00, which casts skill slot 0 at
// a target in reach and circles it, instead of the melee think). Which field the exe fills it from was not found;
// the port sets it for a summoned NecroPet whose monstats row has a first skill played in an attack mode (the
// skeleton mage's NecromageMissile in A1; the plain skeleton has none and the blood golem's skill is a passive
// in mode NU), UNVERIFIED.
func petCasterFlag(ally *allyState, prof *d2monster.Profile) int {
	if ally == nil || prof == nil || !strings.EqualFold(prof.AI, "NecroPet") || !prof.Skills[0].Used() || prof.Skills[0].Mode == d2monster.ModeNeutral {
		return 0
	}

	return 1
}
