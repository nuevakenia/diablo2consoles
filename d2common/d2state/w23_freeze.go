package d2state

// BossFreezeAsChill is the first branch of COMBAT_ApplyFreezeOrColdSlowState
// (0x578f50, VERIFIED wave 23): a monster that has the MonStats boss bit
// (MONSTER_IsBossMonster 0x63fa40, mask 0x40 of the monstats bit word) is never
// frozen. The freeze length is handed to the cold slow routine instead, which
// runs after the chill part of the same hit, so the unit ends up chilled for
// the longer of the two lengths. The exe also takes this branch for a monster
// with data flag 0x8 and for hirelings; those are not modelled (UNVERIFIED:
// what data flag 0x8 stands for).
func BossFreezeAsChill(h Hit) Hit {
	if h.FreezeLen > 0 {
		if h.FreezeLen > h.ColdLen {
			h.ColdLen = h.FreezeLen
		}

		h.FreezeLen = 0
	}

	return h
}
