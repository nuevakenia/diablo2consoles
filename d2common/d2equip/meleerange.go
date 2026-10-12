package d2equip

// HandWeapon is what UNIT_GetMeleeRange (0x622a30) needs to know about one weapon hand of the hero.
type HandWeapon struct {
	Present    bool // an item stands in the slot
	Broken     bool // ITEM_IsNotBrokenAndEquippable (0x62a650) fails: item flag 0x100 or 0x4000
	Weapon     bool // the item is of type weap (0x2d)
	RangeAdder int  // weapons.txt rangeadder column of the base item (ITEM_GetRangeAdder 0x628ab0)
}

// ActiveMeleeRangeAdder is UNIT_GetMeleeRange (0x622a30, VERIFIED w39) for the hero. INV_GetActiveWeaponHandItems
// (0x63da80) takes the right hand when it holds an unbroken item of type weap, else the left hand under the same
// test, else nothing. The result is the rangeadder byte of that item (0 with no weapon). A non-weapon right hand
// (or a broken right weapon) therefore falls through to the left hand.
func ActiveMeleeRangeAdder(right, left HandWeapon) int {
	for _, h := range [2]HandWeapon{right, left} {
		if h.Present && !h.Broken && h.Weapon {
			if h.RangeAdder < 0 {
				return 0
			}

			return h.RangeAdder
		}
	}

	return 0
}
