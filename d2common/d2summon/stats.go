package d2summon

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// ResistCap is the highest resistance a minion can reach through a skill
// (skills.txt writes min(...,85) into the calcs; the cap is applied here too
// so a stray formula cannot exceed it).
const ResistCap = 85

// Mods are the level-scaled numbers a summoning skill adds to the template.
// Aura holds the evaluated aurastat1..6 (these apply to the minion), Passive
// the evaluated passivestat1..5 of the skill (UNVERIFIED which of the two
// groups the exe attaches to the pet: both lists of the real skills.txt only
// use stats that make sense on a minion, so both are applied alike).
type Mods struct {
	// HPPct is the life bonus in percent (calc1 of golems, Raise Skeleton and
	// the vines; d2skill.SummonOrder.HPPct).
	HPPct   int
	Aura    []d2skill.StatMod
	Passive []d2skill.StatMod
	// LevelAC and LevelAR are the armor class and attack rating that
	// SKILL_ComputeSummonLevel (0x5c2850) adds as stats 0x1f / 0x13 from the
	// MonLvl row of the summon's level (VERIFIED); see Templates.LevelBonus.
	LevelAC, LevelAR int
	// SummonResist is the owner's passive_summon_resist: resistances for the minion (d2skill.SummonResistMods).
	SummonResist int
	// SkillName, SkillMin, SkillMax and SkillAR are the summoning skill's physical damage (whole points) and
	// to-hit bonus at its level; Compute adds them when the template's SkillDamage names that skill.
	SkillName                   string
	SkillMin, SkillMax, SkillAR int
}

// FromOrder takes the modifiers of a d2skill summon order plus the skill's
// evaluated passive stats.
func FromOrder(o *d2skill.SummonOrder, passive []d2skill.StatMod) Mods {
	return Mods{HPPct: o.HPPct, Aura: o.Stats, Passive: passive, SummonResist: o.SummonResist,
		SkillName: o.SkillName, SkillMin: o.SkillDmgMin, SkillMax: o.SkillDmgMax, SkillAR: o.SkillToHit}
}

// Stats are the combat numbers of one minion.
type Stats struct {
	MaxHP     int // whole points
	Defense   int
	AR        int
	DmgMin    int
	DmgMax    int
	Res       [numRes]int
	Str, Dex  int
	WalkPct   int // velocitypercent added to Walk and Run
	Walk, Run int // after WalkPct
	Thorns    int // thorns_percent: damage reflected, percent
	FireMin   int // flat fire damage added to every hit (Fire Golem)
	FireMax   int
	// Absorb maps item_absorb*_percent stats to their value.
	Absorb map[string]int
	// Other collects the stats that have no dedicated field, by name.
	Other map[string]int
}

// Compute builds the stats of a minion of template t on difficulty diff with
// the skill modifiers m. r rolls the life between minHP and maxHP; with a nil
// r the maximum is taken. The order of the steps is UNVERIFIED (the exe was
// not traced for it):
//
//	life   = roll(minHP..maxHP) * (100 + HPPct) / 100 + maxhp/256
//	damage = A1 min/max * (100 + damagepercent) / 100
//	AC     = AC + armorclass, then * (100 + item_armor_percent) / 100
//	AR     = A1TH + tohit
//
// maxhp comes in 8.8 fixed point (skills.txt multiplies by 256).
func Compute(t *Template, diff Difficulty, m Mods, r *d2rand.Seed) Stats {
	if diff < Normal || diff > Hell {
		diff = Normal
	}

	b := t.Diff[diff]

	hp := b.MaxHP
	if r != nil && b.MaxHP > b.MinHP {
		hp = b.MinHP + int(r.Roll(int32(b.MaxHP-b.MinHP+1)))
	}

	s := Stats{Walk: t.Walk, Run: t.Run, Res: b.Res, Absorb: map[string]int{}, Other: map[string]int{}}
	s.Defense = b.AC + m.LevelAC
	s.AR = b.A1TH + m.LevelAR
	s.DmgMin, s.DmgMax = b.A1Min, b.A1Max

	// MONSTER_SetupLevelScaledStats 0x5a2960 adds the SkillDamage skill's min, max and to-hit to the level scaled
	// A1 numbers (stats 0x15, 0x16, 0x13) before any damagepercent (VERIFIED).
	if t.DamageSkill != "" && strings.EqualFold(t.DamageSkill, m.SkillName) {
		s.DmgMin += m.SkillMin
		s.DmgMax += m.SkillMax
		s.AR += m.SkillAR
	}

	var dmgPct, armorPct, flatHP int

	for _, st := range append(append([]d2skill.StatMod{}, m.Aura...), m.Passive...) {
		switch name := strings.ToLower(st.Stat); name {
		case "damagepercent":
			dmgPct += st.Value
		case "tohit":
			s.AR += st.Value
		case "armorclass":
			s.Defense += st.Value
		case "item_armor_percent", "skill_armor_percent":
			armorPct += st.Value
		case "maxhp":
			flatHP += st.Value
		case "strength":
			s.Str += st.Value
		case "dexterity":
			s.Dex += st.Value
		case "velocitypercent":
			s.WalkPct += st.Value
		case "thorns_percent":
			s.Thorns += st.Value
		case "firemindam":
			s.FireMin += st.Value
		case "firemaxdam":
			s.FireMax += st.Value
		case "fireresist":
			s.Res[ResFire] += st.Value
		case "lightresist":
			s.Res[ResLightning] += st.Value
		case "coldresist":
			s.Res[ResCold] += st.Value
		case "poisonresist":
			s.Res[ResPoison] += st.Value
		case "damageresist":
			s.Res[ResPhysical] += st.Value
		case "magicresist":
			s.Res[ResMagic] += st.Value
		default:
			if strings.HasPrefix(name, "item_absorb") {
				s.Absorb[name] += st.Value
			} else {
				s.Other[name] += st.Value
			}
		}
	}

	// SKILL_AttachSummonMasteryStatList 0x5c1f20 (VERIFIED): resistances from Summon Resist, set on a separate
	// stat list after the minion exists, so they add to the template's and the skill's.
	for _, rm := range d2skill.SummonResistMods(m.SummonResist, func(n string) int { return s.Absorb[n] }) {
		switch rm.Stat {
		case "fireresist":
			s.Res[ResFire] += rm.Value
		case "lightresist":
			s.Res[ResLightning] += rm.Value
		case "coldresist":
			s.Res[ResCold] += rm.Value
		case "poisonresist":
			s.Res[ResPoison] += rm.Value
		}
	}

	// SKILL_SpawnSummonedMinion 0x5c22c0 (VERIFIED, wave 23): the passive and aura stats go on the unit first
	// (maxhp is 8.8 fixed point), then the unit's maximum life as it stands, flat bonus included, grows by
	// calc1 percent (MulDiv on the 8.8 value) and is written to both stat 7 (max) and stat 6 (current). So the
	// life bonus of Skeleton Mastery is multiplied by the Raise Skeleton percent too.
	total := hp<<8 + flatHP
	total += total * m.HPPct / 100
	s.MaxHP = total >> 8
	if s.MaxHP < 1 {
		s.MaxHP = 1
	}

	// item_normaldamage (stat 0x6f, Skeleton Mastery's passivestat2) is a flat amount added to both ends of the
	// physical roll (COMBAT_BuildAttackerDamage, skills-combat.md). The order against damagepercent is VERIFIED
	// (COMBAT_RollPhysicalDamage 0x579120, wave 27): stat 0x6f is added to both ends first, then the percent is
	// applied to each end; flat first.
	flat := s.Other["item_normaldamage"]
	s.DmgMin += flat
	s.DmgMax += flat

	s.DmgMin = s.DmgMin * (100 + dmgPct) / 100
	s.DmgMax = s.DmgMax * (100 + dmgPct) / 100

	if armorPct != 0 {
		s.Defense = s.Defense * (100 + armorPct) / 100
	}

	for i := range s.Res {
		if s.Res[i] > ResistCap {
			s.Res[i] = ResistCap
		}
	}

	if s.WalkPct != 0 {
		s.Walk = s.Walk * (100 + s.WalkPct) / 100
		s.Run = s.Run * (100 + s.WalkPct) / 100
	}

	return s
}
