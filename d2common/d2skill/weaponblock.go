package d2skill

// Weapon Block (w23). COMBAT_GetEquippedTypeBonusFromStat15C 0x57bcb0 and COMBAT_RollDodgeAvoidEvade 0x57bd60,
// VERIFIED:
//
//   - The chance is the largest value among the entries of stat 0x15c (passive_weaponblock) whose param is an item
//     type the weapon in the left hand (inventory slot 5) or, failing that, the right hand (slot 4) is of. An
//     entry with param 0 counts unconditionally (and replaces the running value, even a larger one).
//   - The roll (rand(100) < chance) happens only when the unit is not moving and the weapon class of its current
//     mode is 0xd. COMPOSIT_ResolveWeaponClassCode 0x650480 gives that class for the Assassin (character class 6)
//     when BOTH hands hold an item of item type 0x2d (item type table row 45, code "weap" = any weapon; w37 corrected an older "claw" guess; the same test Fists of Fire makes). So the
//     block needs two claws; with a single claw or any other weapon it never rolls.
//   - A successful roll returns outcome 0x10 (d2combat.AvoidMonBlock) and ends the chain (no dodge or avoid roll).
//   - The earlier note in d2combat/avoid.go called the weapon-class test an "animation is block" test; the
//     weapon class value 0xd is what the exe compares.

// WeaponBlockChance is the exe's lookup. leftIs / rightIs say whether the item in the hand is of the given type
// (false for an empty hand); leftEmpty reports whether the left hand holds nothing usable.
func WeaponBlockChance(mods []TruePassiveMod, leftEmpty bool, leftIs, rightIs func(itemType string) bool) int {
	best := 0

	for _, m := range mods {
		if m.Stat != "passive_weaponblock" {
			continue
		}

		switch {
		case m.Param == "":
			best = m.Value
		case !leftEmpty && leftIs != nil && leftIs(m.Param):
			if best < m.Value {
				best = m.Value
			}
		case rightIs != nil && rightIs(m.Param):
			if best < m.Value {
				best = m.Value
			}
		}
	}

	return best
}
