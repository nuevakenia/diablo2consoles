package d2skill

// Batch 5 (docs: d2-re-notes/skills-batch5.md). Addresses are Game.exe 1.14b;
// VERIFIED marks a rule read in the decompilation.

// UnitKind is the unit type a filter is asked about (the exe's unit type
// numbers: 0 player, 1 monster, 2 object, 3 missile, 4 item).
type UnitKind int

// The unit kinds of SKILL_UnitMatchesTargetFilter.
const (
	KindPlayer UnitKind = iota
	KindMonster
	KindObject
	KindMissile
	KindItem
)

// Further bits of the target filter word (SKILL_UnitMatchesTargetFilter 0x569100
// and its client twin UNIT_PassesTargetFilterMask 0x4c23a0, VERIFIED). The
// type bits and FilterUndeadOnly, FilterNoBoss, FilterHostile, FilterAllied are
// declared in skills_next2.go.
const (
	FilterObjects      = 0x10    // objects
	FilterMissiles     = 0x8     // missiles (not those with missile bits4 & 2)
	FilterItems        = 0x20    // items
	FilterNeedFlag2    = 0x80    // the unit's flag bit 2 must be set
	FilterNoTown       = 0x100   // the unit must not stand in a town room
	FilterLineOfSight  = 0x200   // a collision trace (mask 4) between caster and unit must be clear
	FilterNeedFlag3    = 0x400   // the unit's flag bit 3 must be set
	FilterDeadOnly     = 0x1000  // only dead units (player mode 0x11, monster mode 0xc)
	FilterMelee        = 0x20000 // within melee range of the caster
	FilterNoStatFlag80 = 0x40000 // monsters whose stat record has flag 0x80 are excluded
	FilterNoState56    = 0x80000 // units in state 0x56 are excluded
)

// FilterFacts are the facts about a unit the filter looks at. The caller (the
// engine) fills them; unknown facts stay false.
type FilterFacts struct {
	Kind UnitKind
	// Dead is the unit's death mode (player mode 0x11, monster mode 0xc);
	// Dying the earlier mode 0 (a monster or player still in its first mode
	// is never a live target).
	Dead, Dying bool
	Undead      bool
	Boss        bool
	// StatFlag80 is MONSTER_IsStatRecordFlag80 (meaning unread).
	StatFlag80 bool
	// MissileBit2 is bit 2 of missiles.txt dwBits4 (such missiles never pass).
	MissileBit2 bool
	// Flag2 and Flag3 are bits 2 and 3 of the unit's flag word.
	Flag2, Flag3 bool
	InTownRoom   bool
	Allied       bool
	// NoAuraMonster is bit 0x100 of the monstats flag word: a monster with it
	// never passes the allied test (VERIFIED; meaning of the bit unread).
	NoAuraMonster bool
	Hostile       bool
	InMelee       bool
	HasState56    bool
	LineClear     bool
}

// FilterMatch is SKILL_UnitMatchesTargetFilter (0x569100, VERIFIED): the whole
// filter word of an aura or area skill against one unit. A zero word is
// replaced by 0x583 like the exe's default. The 0x10103 of the druid totem
// auras therefore reads: players and monsters (0x1, 0x2), not in a town room
// (0x100), allied to the caster (0x10000, which keeps pets and the mercenary
// and rejects monsters carrying bit 0x100 of their record).
func FilterMatch(filter int, u FilterFacts) bool {
	if filter == 0 {
		filter = 0x583
	}

	if !filterKind(filter, u) {
		return false
	}

	if filter&FilterNeedFlag2 != 0 && !u.Flag2 {
		return false
	}

	if filter&FilterNeedFlag3 != 0 && !u.Flag3 {
		return false
	}

	if filter&FilterNoTown != 0 && u.InTownRoom {
		return false
	}

	if filter&FilterAllied != 0 {
		if !u.Allied || (u.Kind == KindMonster && u.NoAuraMonster) {
			return false
		}
	}

	if filter&FilterHostile != 0 && !u.Hostile {
		return false
	}

	if filter&FilterMelee != 0 && !u.InMelee {
		return false
	}

	if filter&FilterNoState56 != 0 && u.HasState56 {
		return false
	}

	if filter&FilterLineOfSight != 0 && !u.LineClear {
		return false
	}

	return true
}

// filterKind is the type switch of the filter: the type bit, alive / dead,
// undead, boss and record-flag tests.
func filterKind(filter int, u FilterFacts) bool {
	alive := !u.Dead && !u.Dying

	switch u.Kind {
	case KindPlayer, KindMonster:
		bit := FilterPlayers
		if u.Kind == KindMonster {
			bit = FilterMonsters
		}

		if filter&bit == 0 {
			return false
		}

		if filter&FilterDeadOnly != 0 {
			if !u.Dead {
				return false
			}
		} else if !alive {
			return false
		}

		if u.Kind == KindMonster {
			if filter&FilterUndeadOnly != 0 && !u.Undead {
				return false
			}

			if filter&FilterNoBoss != 0 && u.Boss {
				return false
			}

			if filter&FilterNoStatFlag80 != 0 && u.StatFlag80 {
				return false
			}
		}
	case KindObject:
		return filter&FilterObjects != 0
	case KindMissile:
		return filter&FilterMissiles != 0 && !u.MissileBit2
	case KindItem:
		return filter&FilterItems != 0
	default:
		return false
	}

	return true
}
