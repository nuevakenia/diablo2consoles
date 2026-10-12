package d2monsters

import "strings"

// Host side of the Assassin's shadows. Since w24 the ported thinkers run
// (MONAI_Think_ShadowWarrior 0x5e9ff0, MONAI_Think_ShadowMaster 0x5ea9f0):
// follow the owner by MONAI_SelectPetActionByRange, leash, target distance
// gates and the roll for the plain attack are the exe's. Since w25 the skill
// list, the Pre-hooks, the scoring environment, the buff upkeep and the
// Warrior's copy of the hero's skills are hosted too: see shadowskills.go
// (skill book, Pre-hook, copy, upkeep, casting) and shadowenv.go (scan and
// state queries for the scoring).

// isShadowAI reports whether the unit's AI is one of the shadow thinkers.
func isShadowAI(u *unit) bool {
	if u == nil || u.b == nil || u.b.Def == nil {
		return false
	}

	switch strings.ToLower(u.b.Def.Name) {
	case "shadowwarrior", "shadowmaster", "shadowmasternoinit":
		return true
	}

	return false
}
