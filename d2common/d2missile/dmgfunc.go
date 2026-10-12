package d2missile

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2calc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
)

// Server damage functions (pSrvDmgFunc, table 0x739b88, ids 1..14), run on the damage struct a missile hit
// built (MISSILE_ProcessHitOrExpire 0x5aba10). Only the ones read so far are modelled. Table entries (V, read from
// the table bytes): 1 0x5aaba0, 2 0x5aacf0, 3 0x5aad50, 4 0x5aadb0, 5 0x5aae10, 6 0x5aaec0, 7 0x5aaef0, 8 0x5aaf50,
// 9 0x5aafa0, 10 0x5aadd0, 11 0x5ab030, 12 0x5aac30, 13 0x5ab0c0, 14 0x5ab120.
//
// Function 7, 0x5aaef0 (V, wave 20; Shock Wave's "shockwave" missile): the stun length of the hit becomes
// the missile record's word at +0x78 when that is above zero (that word is the dParam1 column, wave 22), else
// SKILLS_GetBasePlusPerLevelValue 0x4e3ce0 of the missile's skill at the missile's level, Param1 + (level - 1)
// * Param2 (zero below level 1): Shock Wave 40 frames plus 15 per level. The hit class word of the damage struct
// (+0x60) becomes 0x60. The damage itself is whatever the missile's damage descriptor rolled.
//
// Function 8, 0x5aaf50 (V, wave 22; the erruption cracks): when the missile record's word at +0x36 (the Skill
// column) is above zero it only adds the overlay with that id to the hit unit (UNIT_AddOverlayEffectStat
// 0x622020): a visual, nothing for the simulation. Eruption has no server side sub missiles at all (the crack
// is the center's SubMissile1, the smoke is client only).
//
// Function 14, 0x5ab120 (V, wave 22; the molten boulder; the exe's own name for it is
// MISSILE_RollDamageBonusVsTargetKind, but it never touches damage): a knockback roll. It sets result bit 8 of the
// damage struct when a roll of 100 on the MISSILE's seed is below the chance BoulderKnockChance gives.

// Functions 1, 2, 3 and 12 (V, wave 23; read from the disassembly, the functions are not defined in the project):
//
// Function 1, 0x5aaba0 (Magic, Fire and Cold Arrow): the missile's DmgCalc1 (dl12 = dParam1 + (level - 1) *
// dParam2, the missile's own skill level) is a percent, clamped to 100; a percent of zero or less ends the
// function. That percent of the rolled physical damage (never more than all of it) moves from the physical
// field to the field of the missile's EType (0x5a6c70: none physical, fire, lightning, magic, cold, poison,
// life leech, mana leech, stamina leech, stun, burn; frze SETS the cold field).
//
// Function 12, 0x5aac30 (Lightning Bolt, the javelin): the same percent of the physical damage, plus what the
// missile's EType would roll on its own (0x5a6690: the lightning field the descriptor rolled), but every damage
// field and the three ignore-flags (0x100, 0x200, 0x400) are cleared first (0x5a6bd0); the sum then goes to the
// field of the EType. With the table's dParam1 100 the whole weapon blow becomes lightning.
//
// Function 2, 0x5aacf0 (Ice Arrow, the chaos ice): the freeze length of the hit is the missile's cold length
// times dParam1 percent (a negative dParam1 counts as 0), and the chill length is cleared.
//
// Function 3, 0x5aad50 (Fire Wall, Blaze, the burning grounds): one roll of the MISSILE's own seed, (value &
// 0x7f) below dParam1 sets result bit 0x4000 (a soft hit: the target is not staggered).

// ResultSoftHit is result bit 0x4000 of a hit: the target is not staggered (function 3, a blocked hit).
const ResultSoftHit uint32 = 0x4000

// missEnv evaluates the misscalc fields a damage function needs (dl12, dd12, dpa1, dpa2, lvl) for a missile.
type missEnv struct {
	d2calc.ZeroEnv
	sp  *Spec
	lvl int
}

// Field implements d2calc.Env.
func (e missEnv) Field(code string) int {
	a, b := e.sp.DParam[0], e.sp.DParam[1]

	switch code {
	case "dl12":
		return d2calc.LN(a, b, e.lvl)
	case "dd12":
		return d2calc.DM78(a, b, e.lvl)
	case "dpa1":
		return a
	case "dpa2":
		return b
	case "lvl":
		return e.lvl
	}

	return 0
}

// convertPercent is the percent DmgCalc1 gives for a missile (0x64c9b0 with the missile's own skill and level),
// clamped to 100; 0 when the missile has no calc.
func convertPercent(m *Missile) int {
	if m.Spec.DmgCalc1.Empty() {
		return 0
	}

	pct := m.Spec.DmgCalc1.Eval(missEnv{sp: m.Spec, lvl: m.Level})
	if pct > 100 {
		pct = 100
	}

	return pct
}

// addByEType is MISSILE_AddToHitDataFieldByType (0x5a6c70).
func addByEType(d *d2combat.Damage, etype string, v int32) {
	switch etype {
	case "", "none", "phys":
		d.Physical += v
	case "fire":
		d.Fire += v
	case "ltng":
		d.Lightning += v
	case "mag":
		d.Magic += v
	case "cold":
		d.Cold += v
	case "pois":
		d.Poison += v
	case "life":
		d.LifeLeech += v
	case "mana":
		d.ManaLeech += v
	case "stam":
		d.StaminaLeech += v
	case "stun":
		d.StunLen += v
	case "burn":
		d.Burn += v
	case "frze":
		d.Cold = v
	}
}

// convertedPhysical is the part of the physical damage functions 1 and 12 move: percent of it, at most all of it.
func convertedPhysical(phys int32, pct int) int32 {
	e := int32(int64(phys) * int64(pct) / 100)
	if e > phys {
		e = phys
	}

	return e
}

// dmgFuncHitClassStun is the hit class word function 7 stores.
const dmgFuncHitClassStun = 0x60

// ResultKnockback is bit 8 of the result word of a hit: the target is pushed back (Sanctuary, Smite and the
// molten boulder carry it).
const ResultKnockback uint32 = 0x8

// MonStats2 flag bits the molten boulder reads through MONSTATS2_TestFlagBit 0x45f0b0. The bit numbers are the
// positions in the flag array of the monstats2 loader table (0x659bd0..0x65a130 in the exe, V) and are confirmed
// by SRVDO_075_GrimWard 0x5d7430, which tests the same array bytewise (bit 3 / 2 of the second byte).
const (
	ClassBitSmall = 0xa // the "small" column of monstats2
	ClassBitLarge = 0xb // the "large" column of monstats2
)

// ClassBitter is optionally implemented by a monster Target: ClassBit reports a MonStats2 flag bit.
type ClassBitter interface{ ClassBit(bit int) bool }

// StunLength is function 7's stun in frames.
func StunLength(param1, param2, level int) int {
	if level < 1 {
		return 0
	}

	return param1 + (level-1)*param2
}

// BoulderKnockChance is the percent chance of function 14 (0x5ab120, V) for a target, from the missile record's
// dParam1 (mode) and dParam2 (base chance): nothing without a positive dParam2. A player is pushed with the base
// chance when the mode is above zero. A monster: mode 0 only the small ones (class bit 0xa) with the base chance;
// mode 1 small x2, large (bit 0xb) never, the rest the base chance; any higher mode small x3, large the base
// chance, the rest x2. Unit types other than player and monster are never pushed.
func BoulderKnockChance(mode, base int, player, small, large bool) int {
	if base <= 0 {
		return 0
	}

	if player {
		if mode < 1 {
			return 0
		}

		return base
	}

	switch {
	case mode < 1:
		if !small {
			return 0
		}

		return base
	case mode == 1:
		switch {
		case small:
			return base * 2
		case large:
			return 0
		}

		return base
	}

	switch {
	case small:
		return base * 3
	case large:
		return base
	}

	return base * 2
}

// StunLength override (function 7): a positive dParam1 replaces the computed stun.
func stunFor(m *Missile) int {
	if p := m.Spec.DParam[0]; p > 0 {
		return p
	}

	return StunLength(m.Skill.Param[1], m.Skill.Param[2], m.Level)
}

// dmgFunc runs the missile's pSrvDmgFunc on the rolled damage.
func (s *Sim) dmgFunc(m *Missile, t Target, d *d2combat.Damage) {
	switch m.Spec.SrvDmgFunc {
	case 1:
		pct := convertPercent(m)
		if pct <= 0 {
			return
		}

		e := convertedPhysical(d.Physical, pct)
		d.Physical -= e
		addByEType(d, m.Spec.EType, e)
	case 12:
		pct := convertPercent(m)
		if pct <= 0 {
			return
		}

		e := convertedPhysical(d.Physical, pct)
		if m.Spec.EType == "ltng" {
			e += d.Lightning
		}

		flags := d.Flags &^ (d2combat.DamageFlagIgnoreUndead | d2combat.DamageFlagIgnoreDemon | d2combat.DamageFlagIgnoreBeast)
		*d = d2combat.Damage{Flags: flags, Result: d.Result, HitClass: d.HitClass, ForcedClass: d.ForcedClass}
		addByEType(d, m.Spec.EType, e)
	case 2:
		pct := m.Spec.DParam[0]
		if pct < 0 {
			pct = 0
		}

		d.FreezeLen = int32(int64(d.ColdLen) * int64(pct) / 100)
		d.ColdLen = 0
	case 3:
		if int(m.seed.Step()&0x7f) < m.Spec.DParam[0] {
			d.Result |= ResultSoftHit
		}
	case 7:
		d.StunLen = int32(stunFor(m))
		d.HitClass = dmgFuncHitClassStun
	case 14:
		if t == nil {
			return
		}

		var small, large bool

		if cb, ok := t.(ClassBitter); ok && !t.IsPlayer() {
			small, large = cb.ClassBit(ClassBitSmall), cb.ClassBit(ClassBitLarge)
		}

		chance := BoulderKnockChance(m.Spec.DParam[0], m.Spec.DParam[1], t.IsPlayer(), small, large)
		if chance > 0 && int(m.seed.Roll(100)) < chance {
			d.Result |= ResultKnockback
		}
	}
}
