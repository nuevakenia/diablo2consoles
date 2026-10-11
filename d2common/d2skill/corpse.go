package d2skill

// Corpse targets and summon levels (batch 5, docs: d2-re-notes/skills-batch5.md).
// VERIFIED marks a rule read in the decompilation of Game.exe 1.14b.

// CorpseFacts are the facts about a unit the corpse tests look at.
type CorpseFacts struct {
	// Monster is true for a monster unit (unit type 1).
	Monster bool
	// Dead is monster mode 0xc (the corpse lying there); Dying is mode 0, the
	// first mode, which Raise Skeleton accepts too when asked to.
	Dead, Dying bool
	// InStateMask150 is STATE_UnitHasStateInMask150: a unit that is flagged
	// by the state group mask 0x150 (the exe tests the mask; which states it
	// holds was not read, so the engine reports its "already used" corpses
	// here, UNVERIFIED).
	InStateMask150 bool
	// Selectable is bit 7 of the monstats2 flag word. In the project's table
	// order that is the corpseSel column (UNVERIFIED mapping of the bit).
	Selectable bool
	// WalkSpeed is the Velocity column of monstats.
	WalkSpeed int
}

// CorpseExplodable is SKILL_CanReviveOrExplodeCorpse (0x646a10, VERIFIED) and the
// first half of the corpse search filter SKILL_ReviveCorpseSearchFilter
// (0x56c120): a monster in its dead mode, not in state mask 0x150, whose
// monstats2 row has flag bit 7. Revive, Corpse Explosion and Poison Explosion
// all start from it.
func CorpseExplodable(c CorpseFacts) bool {
	return c.Monster && c.Dead && !c.InStateMask150 && c.Selectable
}

// CorpseRaisable is MONSTER_IsCorpseUsableForRaise (0x6468b0, VERIFIED), the test
// of Raise Skeleton and Raise Skeletal Mage (SRVDO_031): the same rules, plus
// the monster must have a walking speed (a stationary monster such as a totem
// or a trap leaves no usable corpse). With allowDying the first mode (0)
// counts as a corpse too; both skills pass false.
func CorpseRaisable(c CorpseFacts, allowDying bool) bool {
	if !c.Monster || !(c.Dead || (allowDying && c.Dying)) {
		return false
	}

	return !c.InStateMask150 && c.Selectable && c.WalkSpeed != 0
}

// SummonLevel is SKILL_ComputeSummonLevel (0x5c2850, VERIFIED) for a skill that
// passes no fixed level (Raise Skeleton, Raise Skeletal Mage and the four
// golems): three quarters of the caster's level (rounded down) plus the skill
// level, but never above the caster's level and at least 1. The summoned
// unit gets that as its level and the armor class and attack rating of that
// level's row in monlvl.txt (d2summon.Templates.LevelBonus). A caster without
// a level (0) gives 0 and the unit keeps its own.
func SummonLevel(casterLevel, skillLevel int) int {
	if casterLevel <= 0 {
		return 0
	}

	lvl := casterLevel*3/4 + skillLevel
	if lvl < 1 {
		lvl = 1
	}

	if lvl > casterLevel {
		lvl = casterLevel
	}

	return lvl
}

// GroundItem is a ground item as the pipeline can target it: the item under
// the cursor of Iron Golem (SRVST_020, 0x5c1070 / client CLTST_023 0x4f1490).
type GroundItem struct {
	// ID is the engine handle of the item entity.
	ID string
	// X, Y is its subtile.
	X, Y int
	// Kind is the item's base type code (for the log).
	Kind string
	// Valid is the result of the engine's item test (see IronGolemTargetOK):
	// the base-table golem flag and the flag mask 0x10 / 0x100 test.
	Valid bool
}
