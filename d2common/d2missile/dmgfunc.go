package d2missile

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"

// Server damage functions (pSrvDmgFunc, table 0x739b88, ids 1..14), run on the damage struct a missile hit
// built (MISSILE_ProcessHitOrExpire 0x5aba10). Only the ones read so far are modelled.
//
// Function 7, 0x5aaef0 (V, wave 20; Shock Wave's "shockwave" missile): the stun length of the hit becomes
// the missile record's word at +0x78 when that is above zero (the column behind it was not identified, so
// that override is not modelled and is 0 for the shockwave missile), else SKILLS_GetBasePlusPerLevelValue
// 0x4e3ce0 of the missile's skill at the missile's level, Param1 + (level - 1) * Param2 (zero below level
// 1): Shock Wave 40 frames plus 15 per level. The hit class word of the damage struct (+0x60) becomes
// 0x60. The damage itself is whatever the missile's damage descriptor rolled.

// dmgFuncHitClassStun is the hit class word function 7 stores.
const dmgFuncHitClassStun = 0x60

// StunLength is function 7's stun in frames.
func StunLength(param1, param2, level int) int {
	if level < 1 {
		return 0
	}

	return param1 + (level-1)*param2
}

// dmgFunc runs the missile's pSrvDmgFunc on the rolled damage.
func (s *Sim) dmgFunc(m *Missile, d *d2combat.Damage) {
	if m.Spec.SrvDmgFunc != 7 {
		return
	}

	d.StunLen = int32(StunLength(m.Skill.Param[1], m.Skill.Param[2], m.Level))
	d.HitClass = dmgFuncHitClassStun
}
