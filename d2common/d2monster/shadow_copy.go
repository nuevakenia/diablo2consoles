package d2monster

// Skill copy, mana fatigue and buff upkeep of the Assassin's shadows (w25-shadows). VERIFIED against Game.exe 1.14b:
//
//	0x5e9fa0 MONAI_Pre_ShadowWarrior      Scratch[2] = level of the owner's skill monstats aip8 (normal column) names
//	0x5ea4f0 MONAI_Pre_ShadowMaster       Scratch[0..2] = 0, 0, that level; then the shadow learns every class skill of
//	                                      the owner's class that IsSkillCopyAllowed lets through, at ShadowMasterLevel
//	0x5ea620 MONAI_Pre_ShadowMasterNoInit Scratch[2] only (the other two stay), same skill copy minus skill 0 reset
//	0x5e9f50 ShadowWarriorComputeSkillLevel, 0x5ea490 ShadowMasterComputeSkillLevel (the two level formulas)
//	0x5e9b90 IsSkillCopyAllowed, 0x5e9c50 ValidateCopiedSkill, 0x5e9db0 CopySkillCast (the Warrior's fatigue roll)
//	0x5ea9f0 buff / left-skill upkeep loop (ShadowUpkeep)
//
// The mana of a shadow is not a pool: the skill's mana cost at the shadow's skill level (SKILL_GetAuraParamAtLevelWhole
// 0x646db0, the misnamed whole-point mana cost) feeds a fatigue counter (AI data +0x18, Scratch[1]) and lowers the
// chance that the Warrior casts the copied skill at all.

// ShadowKind tells the three shadow thinkers apart (their Pre-hooks differ).
type ShadowKind int

// The shadow AIs.
const (
	ShadowKindNone ShadowKind = iota
	ShadowKindWarrior
	ShadowKindMaster
	ShadowKindMasterNoInit
)

// ShadowKindOf names the shadow flavour of a brain by its AI definition.
func ShadowKindOf(b *Brain) ShadowKind {
	if b == nil || b.Def == nil {
		return ShadowKindNone
	}

	switch b.Def.Name {
	case "ShadowWarrior":
		return ShadowKindWarrior
	case "ShadowMaster":
		return ShadowKindMaster
	case "ShadowMasterNoInit":
		return ShadowKindMasterNoInit
	}

	return ShadowKindNone
}

// ShadowPre is the host side of the Pre-hooks: it copies the owner's skills to the shadow and fills Scratch. The think
// functions call it once, at the first think after SetAI (the exe runs the hooks from MONAI_SetAiState).
type ShadowPre interface {
	ShadowInit(b *Brain, kind ShadowKind)
}

// shadowPre runs the Pre-hook through the host, when it has one.
func shadowPre(c *Ctx) {
	h, ok := c.W.(ShadowPre)
	if !ok {
		return
	}

	kind := ShadowKindOf(c.B)

	c.pre(func() { h.ShadowInit(c.B, kind) })
}

// ShadowMasterLevel is MONAI_ShadowMasterComputeSkillLevel 0x5ea490: half the owner's own points in the skill (1 when
// the owner lacks it) plus half the owner's Shadow Master level, clamped to 1..24.
func ShadowMasterLevel(ownerBase, shadowSkill int) int {
	v := ownerBase/2 + shadowSkill/2

	switch {
	case v <= 1:
		return 1
	case v >= 0x18:
		return 0x18
	}

	return v
}

// ShadowWarriorLevel is MONAI_ShadowWarriorComputeSkillLevel 0x5e9f50: a third of the owner's Shadow Warrior level
// (AI data +0x1c) plus half the total level of the copied skill, at least 1.
func ShadowWarriorLevel(shadowSkill, ownerTotal int) int {
	v := shadowSkill/3 + ownerTotal/2
	if v < 2 {
		return 1
	}

	return v
}

// ShadowManaWhole is SKILL_GetAuraParamAtLevelWhole 0x646db0: ((lvlmana * (level-1) + mana) << manashift) >> 8, never
// negative. The shift count is masked to 5 bits like the exe's shift.
func ShadowManaWhole(mana, lvlMana, manaShift, level int) int {
	v := int32((lvlMana*(level-1)+mana)<<uint(manaShift&31)) >> 8
	if v < 0 {
		return 0
	}

	return int(v)
}

// ShadowCopyAllowed is MONAI_ShadowWarrior_IsSkillCopyAllowed 0x5e9b90. summons is skills.txt summon != 0,
// summonsOwnClass the skill's summon class equal to the shadow's class, petType the skill's pettype (-1 none) and
// shadowPetType the shadow's own pet type (-1 unknown). UNVERIFIED: which unit the pet-type lookup (0x?
// PETS_FindPetTypeByUnitId, register argument) is made for; the shadow itself is assumed.
func ShadowCopyAllowed(summons, summonsOwnClass bool, petType, shadowPetType int, hasOwner bool) bool {
	if !summons {
		return true
	}

	if summonsOwnClass {
		return false
	}

	if petType >= 0 && hasOwner {
		return shadowPetType != petType
	}

	return true
}

// ShadowValidateIn is the state MONAI_ShadowWarrior_ValidateCopiedSkill 0x5e9c50 reads.
type ShadowValidateIn struct {
	Skill       ShadowSkill
	Allowed     bool // ShadowCopyAllowed
	InRange     bool // AI data +0x18 (the target is within melee reach)
	HasTarget   bool
	SelfState   bool // the shadow has Skill.AuraState
	TargetHas   bool // the target has Skill.AuraTargetState
	StateStat   int  // value of Skill.AuraStat1 in the shadow's state list
	StateStatOK bool
}

// ShadowValidateCopy ports 0x5e9c50: whether a copied skill may be used now.
func ShadowValidateCopy(in ShadowValidateIn) bool {
	sk := in.Skill
	t := sk.AIType

	if !in.Allowed {
		return false
	}

	// melee kinds only in reach, everything else only out of it
	if !in.InRange {
		if t == 4 || t == 0xd {
			return false
		}
	} else if t != 4 && t != 0xd {
		return false
	}

	// a buff the shadow already carries
	if t == 1 && sk.AuraState > 0 && in.SelfState {
		return false
	}

	if !in.HasTarget {
		switch t {
		case 2, 4, 5, 0xb, 0xc, 0xd:
			return false
		}
	}

	if t == 2 && (sk.AuraState > 0 && in.SelfState || sk.AuraTargetState > 0 && in.TargetHas) {
		return false
	}

	// progressive skills stop at 3 charges
	if sk.Bits4&4 != 0 && sk.AuraState > 0 && in.SelfState && in.StateStatOK && in.StateStat >= 3 {
		return false
	}

	return true
}

// shadowClampLow is the exe's clamp of aip8 normal/nightmare for the fatigue floor: below 2 -> 1, 2..0x7f as is,
// from 0x80 -> 0x80.
func shadowFatigueFloor(w int) int {
	switch {
	case w < 2:
		return 1
	case w < 0x80:
		return w
	}

	return 0x80
}

// shadowFatigueCap is the cap of the same counter (aip8 Hell): below 2 -> 1, 2..0xff as is, from 0x100 -> 0x100.
func shadowFatigueCap(w int) int {
	switch {
	case w < 2:
		return 1
	case w <= 0xff:
		return w
	}

	return 0x100
}

// ShadowCopyIn is everything MONAI_ShadowWarriorCopySkillCast 0x5e9db0 reads besides the brain.
type ShadowCopyIn struct {
	Frame      int
	OwnerClass int // the owner's class index (UnitAny.dwTxtFileNo)
	SkillClass int // SKILL_GetSkillClass of the candidate (7 for a skill with no class)
	SkillID    int
	Valid      bool // ShadowValidateCopy
	Cost       int  // ShadowManaWhole at the shadow's own level of the candidate
}

// ShadowCopyCast ports 0x5e9db0. It draws from the shadow's seed in the exe's order: the mana roll first, then (past the
// cooldown gate) the fatigue roll and the 100 roll. On success the fatigue counter Scratch[1] grows by
// (0x140 - level) * cost / (level + 100). Plain attack (skill 0) passes right after validation, but its skill class
// (7) never equals the owner's, so the exe's chain reaches the second skill first; the class test is kept literally.
func ShadowCopyCast(b *Brain, in ShadowCopyIn) bool {
	if in.SkillClass != in.OwnerClass {
		return false
	}

	if !in.Valid {
		return false
	}

	if in.SkillID == 0 {
		return true
	}

	if 100-(in.Cost*0xa0)/100 < b.Roll(100) {
		return false
	}

	if in.Frame < b.Scratch[0] {
		return false
	}

	lo := shadowFatigueFloor(shadowWord(b, 8, 1))
	hi := shadowFatigueCap(shadowWord(b, 8, 2))

	if b.Scratch[1] < lo || hi*32 < b.Scratch[1] {
		b.Scratch[1] = lo
	}

	r := b.Roll(b.Scratch[1])
	if b.Roll(100) < r {
		return false
	}

	b.Scratch[1] += ((0x140 - b.Scratch[2]) * in.Cost) / (b.Scratch[2] + 100)

	return true
}

// ShadowCooldown is the Warrior's recast gate after a copied cast: calc / 3 + 18 frames from now (the calc is the
// skill's delay, evaluated at the shadow's level; AI data +0x14).
func ShadowCooldown(delayCalc, frame int) int { return delayCalc/3 + 0x12 + frame }

// ShadowUpkeepEnv is what the upkeep loop asks of the host.
type ShadowUpkeepEnv interface {
	SelfHasState(b *Brain, state int) bool
	OtherInStateGroup(b *Brain, state int) bool
	LeftSkillSet(b *Brain) bool
	SelectActiveSkill(b *Brain, skillID int)
}

// ShadowUpkeep ports the loop at 0x5eab13 of MONAI_Think_ShadowMaster that runs when the shadow has no target. Over the
// shadow's skills in list order: a buff (aitype 1) with an aura state the shadow lacks is cast on a 60 percent roll,
// but only when no other state of its group is active or a 4 percent roll lets it through (the 100 roll is drawn only
// when another state of the group is active); a skill of aitype 6 picks the active left skill with 20 percent while none
// is set. cast queues the skill (CastChosenSkill without target); the loop ends with the first cast that queued.
func ShadowUpkeep(b *Brain, env ShadowUpkeepEnv, skills []ShadowSkill, cast func(skill int) bool) bool {
	for _, sk := range skills {
		switch sk.AIType {
		case 1:
			if sk.AuraState <= 0 || env.SelfHasState(b, sk.AuraState) {
				continue
			}

			if env.OtherInStateGroup(b, sk.AuraState) && b.Roll(100) >= 4 {
				continue
			}

			if b.Roll(100) >= 0x3c {
				continue
			}

			if cast(sk.ID) {
				return true
			}
		case 6:
			if !env.LeftSkillSet(b) && b.Roll(100) < 0x14 {
				env.SelectActiveSkill(b, sk.ID)
			}
		}
	}

	return false
}
