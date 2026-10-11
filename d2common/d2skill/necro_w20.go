package d2skill

// Wave 20 Necromancer pieces read from Game.exe 1.14b (notes: w20-skills-necro.md). Every function here
// is a small pure rule; the engine hooks live in d2core/d2skills.

// Curse aura target states with a special rule in the exe. The state ids are the rows of States.txt:
// dimvision 23 (0x17), taunt 27 (0x1b), terror 56 (0x38).
const (
	stateDimVision = "dimvision"
	stateTaunt     = "taunt"
	stateTerror    = "terror"
)

// CurseDivisorState is the test SKILL_IsAmplifyClass17Or38 (0x5c11d0, VERIFIED): the aura target state is
// Dim Vision or Terror.
func CurseDivisorState(state string) bool { return state == stateDimVision || state == stateTerror }

// CurseFrames is the duration of a curse after the difficulty division (VERIFIED): SRVDO_030 (0x5c1590,
// the Amplify Damage family) divides auralencalc by DifficultyLevels AiCurseDivisor (0x5c1570 reads the
// difficulty byte of the game) only when the aura target state is Dim Vision or Terror; SRVDO_059 Attract
// (0x5c1990) and SRVDO_061 Confuse (0x5c1d60) divide always. A divisor of 0 leaves the length alone. The
// division does not look at who casts: a player's Terror is shorter in Hell as well. doFunc is the
// srvdofunc of the skill.
func CurseFrames(doFunc int, state string, frames, aiCurseDivisor int) int {
	divides := doFunc == 59 || doFunc == 61 || (doFunc == 30 && CurseDivisorState(state))
	if !divides || aiCurseDivisor <= 0 {
		return frames
	}

	return frames / aiCurseDivisor
}

// CurseMonstersOnly is the context flag +8 of SKILL_ApplyCurseToTarget (0x5c1380, VERIFIED): for Dim Vision
// and Terror (the same state test as the length division) the target must be a monster, not hostile-
// aligned to the good side (alignment 1 refuses) and a valid monster skill target (0x56c0a0). Players and
// mercenaries are never touched by these two curses. Every other curse works on players too.
func CurseMonstersOnly(state string) bool { return CurseDivisorState(state) }

// CursedAIState is SKILL_SetCursedMonsterAiState (0x5c1270, VERIFIED): the AI state a curse puts the monster
// in, from the aura target state of the skill: Dim Vision 10, Taunt 12, Terror 11, any other state 0. It is
// only called for the curses of CurseMonstersOnly (the context flag); Taunt (srvdofunc 71) uses its own
// path and reaches the same helper.
func CursedAIState(state string) int {
	switch state {
	case stateDimVision:
		return 10
	case stateTaunt:
		return 12
	case stateTerror:
		return 11
	}

	return 0
}

// SummonResistStat is the stat Summon Resist (skill 89) puts on its owner: passive_summon_resist, 0x15d.
const SummonResistStat = "passive_summon_resist"

// AttachesSummonMastery says whether a srvdofunc calls SKILL_AttachSummonMasteryStatList (0x5c1f20,
// VERIFIED, two callers): SRVDO_031 (Raise Skeleton and Raise Skeletal Mage) and SKILL_SummonGolemCommon
// 0x5c2f60 (SRVDO_056 Clay, Blood and Fire Golem, SRVDO_057 Iron Golem). Revive (58) does not.
func AttachesSummonMastery(doFunc int) bool { return doFunc == 31 || doFunc == 56 || doFunc == 57 }

// SummonResistMods is SKILL_AttachSummonMasteryStatList: when the owner's stat 0x15d is not zero, the
// new minion gets a stat list with that value as fire resistance (0x27) if its absorb-fire percent
// (stat 0x8e) is below 1, lightning resistance (0x29) if its absorb-lightning percent (0x90) is below 1,
// cold resistance (0x2b) if its absorb-cold percent (0x94) is below 1, and always poison resistance
// (0x2d). absorb gives the minion's absorb stats by name ("item_absorbfire_percent", ...).
func SummonResistMods(summonResist int, absorb func(name string) int) []StatMod {
	if summonResist == 0 {
		return nil
	}

	var out []StatMod

	if absorb("item_absorbfire_percent") < 1 {
		out = append(out, StatMod{Stat: "fireresist", Value: summonResist})
	}

	if absorb("item_absorblight_percent") < 1 {
		out = append(out, StatMod{Stat: "lightresist", Value: summonResist})
	}

	if absorb("item_absorbcold_percent") < 1 {
		out = append(out, StatMod{Stat: "coldresist", Value: summonResist})
	}

	return append(out, StatMod{Stat: "poisonresist", Value: summonResist})
}

// RaiseSkeletonLifeCost is SRVDO_NecromancerLifeCostEighthOfMax (0x5c0dc0, VERIFIED), which SRVDO_031
// calls before it spawns: a unit of type player and class index 3 loses one eighth of its maximum life
// (8.8 units, a plain subtraction floored at 0, hit-recovery result flag, friendly-fire flag 0x1000). The
// class index 3 is the PALADIN in 1.14b's class order, so a Necromancer (index 2) never pays it; the
// name of the function comes from the beta class order. Returns the life points (not 8.8) removed.
func RaiseSkeletonLifeCost(classIndex, maxLifePoints int) int {
	if classIndex != 3 {
		return 0
	}

	return maxLifePoints / 8
}

// CurseAlignmentOK is the alignment test of the curse targets (VERIFIED): Attract (SKILL_IsValidAttractTarget
// 0x5c0fd0) and Confuse (SKILL_IsValidConfuseTarget 0x5c1b70) accept only a unit of alignment 0 (the
// hostile side); Dim Vision and Terror (0x5c1380) refuse alignment 1 (the good side) and nothing else; the
// other curses have no alignment test there. The alignment is the monstats Align column (UNVERIFIED that the
// exe's live alignment, which a conversion changes, equals it for a monster nobody touched).
func CurseAlignmentOK(doFunc int, state string, alignment int) bool {
	switch {
	case doFunc == 59 || doFunc == 61:
		return alignment == 0
	case CurseMonstersOnly(state):
		return alignment != 1
	}

	return true
}
