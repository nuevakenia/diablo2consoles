package d2player

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
)

// WeaponRangeAdder is UNIT_GetMeleeRange (0x622a30) for the hero: the rangeadder column of the active weapon hand
// (ITEM_GetRangeAdder 0x628ab0). INV_GetActiveWeaponHandItems (0x63da80, VERIFIED w39) takes the right hand if it
// holds an unbroken item of type weap, else the left hand under the same test (d2equip.ActiveMeleeRangeAdder).
// The 1.14 tables hold 1..4 in the column (clubs, hammers and swords 1..2, polearms 1..4, spears 3..4), blank
// (0) for bare hands and for the other weapons. It feeds the Fend / Zeal / Frenzy reach (reach = this + 4).
func (g *GameControls) WeaponRangeAdder() int {
	if g == nil || g.inventory == nil {
		return 0
	}

	return d2equip.ActiveMeleeRangeAdder(g.handWeapon(d2equip.LocRightHand), g.handWeapon(d2equip.LocLeftHand))
}

// handWeapon describes the item worn at a hand for the melee range test.
func (g *GameControls) handWeapon(loc d2equip.Loc) d2equip.HandWeapon {
	it, ok := g.inventory.WornAt(loc).(*diablo2item.Item)
	if !ok || it == nil {
		return d2equip.HandWeapon{}
	}

	h := d2equip.HandWeapon{Present: true, Weapon: true, Broken: it.HasBrokenFlag()}

	if rec := it.CommonRecord(); rec != nil {
		h.RangeAdder = rec.ExtraRange
	}

	if g.heroState != nil {
		if rules, bases, ok := g.heroState.EquipRules(); ok {
			h.Weapon = rules.Types.IsA(bases[it.GetItemCode()].Type, "weap")
		}
	}

	return h
}
