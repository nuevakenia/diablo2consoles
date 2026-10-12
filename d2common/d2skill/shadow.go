package d2skill

// Shadow Warrior / Shadow Master bonus stats (w23). SRVDO_049 0x5d5850 and SKILL_ShadowWarriorApplyBonusStats
// 0x5d56d0, VERIFIED from the disassembly:
//
//   - The summoned shadow takes the caster's level (stat 0xc).
//   - Only when the skill level is above 1 does it get bonuses at all: its maximum life and life grow by
//     MulDiv(maxlife, (level-1)*Param1, 100) percent of the base, then a stat list is attached with
//     aurastat1..6 and passivestat1..5 of the skill.
//   - Every aurastat slot is evaluated with the SAME calc, aurastatcalc2 (record +0x6c, the instruction reloads
//     it from the same address in each loop pass; compare wAurastat2 / dwAurastatcalc2 at +0x56 / +0x6c in
//     SRVDO_TigerStrike_ApplyChargeStatList), and every passivestat slot with passivecalc2 (+0xa8). With the
//     real skills.txt the passive values are all alike, so only the aura stats of Shadow Warrior (tohit,
//     skill_armor_percent, strength, dexterity) change from "own calc each".
//   - The aurastate is set on the shadow, sumumod (42) is counted as a unique modifier of the region, and a
//     positive auralencalc schedules its expiry (none for the shadows).
//
// Not ported: the monequip template items of the shadow.

// shadowBonuses returns the aura and passive stat lists of a shadow at a skill level, nil below level 2.
func shadowBonuses(env *Env, sk *Skill, lvl int) (aura, passive []StatMod) {
	if lvl <= 1 {
		return nil, nil
	}

	av := env.eval(sk.AuraStatCalc[2])

	for i := 1; i <= 6; i++ {
		if sk.AuraStat[i] != "" {
			aura = append(aura, StatMod{Stat: sk.AuraStat[i], Value: av})
		}
	}

	pv := env.eval(sk.PassiveCalc[2])

	for i := 1; i <= 5; i++ {
		if sk.PassiveStat[i] != "" {
			passive = append(passive, StatMod{Stat: sk.PassiveStat[i], Value: pv})
		}
	}

	return aura, passive
}

// shadowHPPct is the life bonus percent: (level-1)*Param1 (MulDiv(maxlife, that, 100)).
func shadowHPPct(sk *Skill, lvl int) int {
	if lvl <= 1 {
		return 0
	}

	return (lvl - 1) * sk.Params[1]
}
