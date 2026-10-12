package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// weaponBlockChance is the Weapon Block chance of a hero for the avoid roll (COMBAT_GetEquippedTypeBonusFromStat15C
// 0x57bcb0) and whether the weapon class test of 0x57bd60 passes: an Assassin with a claw (item type "h2h", the
// engine's name for exe type 0x2d, UNVERIFIED mapping) in BOTH hands (see d2skill/weaponblock.go). The chance is 0
// when the test fails, so the avoid chain does not roll.
func (e *Engine) weaponBlockChance(h *heroUnit) int {
	if h == nil || h.merc != nil || h.p == nil || h.p.Class != d2enum.HeroAssassin || h.p.Equipment == nil ||
		h.p.Equipment.LeftHand == nil || h.p.Equipment.RightHand == nil || h.inPassive {
		return 0
	}

	lt := e.weaponType(h.p.Equipment.LeftHand.GetItemCode())
	rt := e.weaponType(h.p.Equipment.RightHand.GetItemCode())

	is := func(have string) func(string) bool {
		return func(want string) bool { return d2skill.TypeIs(e.equivOf, have, want) }
	}

	if lt == "" || rt == "" || !is(lt)("h2h") || !is(rt)("h2h") {
		return 0
	}

	var mods []d2skill.TruePassiveMod

	h.inPassive = true

	for id, s := range h.p.Skills {
		if s != nil && s.SkillPoints > 0 {
			mods = append(mods, e.pipe.TruePassiveStats(h, id)...)
		}
	}

	h.inPassive = false

	return d2skill.WeaponBlockChance(mods, false, is(lt), is(rt))
}
