package d2skill

// ReasonRestricted: the skills.txt restrict rule refuses the cast for the
// unit's shape (human only skill in a form, form only skill outside its form).
const ReasonRestricted = "restricted"

// FormStateHolder is implemented by a Unit that can name which shape shift
// state it is in (wolf, bear, delerium).
type FormStateHolder interface {
	HasFormState(name string) bool
}

// RestrictOK is SKILL_CheckStateRestrictions 0x645450 (VERIFIED, wave 23).
// The skill's restrict byte (skills.txt "restrict", blank = 0):
//   - 0: usable only when the unit has none of the states whose States.txt
//     restrict column is set (wolf, bear, delerium), i.e. human shape only
//     (Firestorm, Twister, Cyclone Armor, Molten Boulder ... Hurricane);
//   - 1 (and any value other than 2): always usable (summons, totems, the
//     form skills themselves, auras, Attack);
//   - 2: the unit must be in a restrict state AND carry one of State1..State3
//     (the scan of the three words stops at the first negative id, i.e. an
//     unset column): Feral Rage, Rabies, Fury need the wolf; Maul, Shock Wave
//     the bear; Fire Claws and Hunger either. Outside any form these refuse.
func RestrictOK(u Unit, sk *Skill) bool {
	shifted := false
	if h, ok := u.(StateHolder); ok {
		shifted = h.Shapeshifted()
	}

	switch sk.Restrict {
	case 0:
		return !shifted
	case 2:
		if !shifted {
			return false
		}

		fh, ok := u.(FormStateHolder)
		if !ok {
			return false
		}

		for _, name := range sk.FormStates {
			if name == "" {
				return false
			}

			if fh.HasFormState(name) {
				return true
			}
		}

		return false
	}

	return true
}
