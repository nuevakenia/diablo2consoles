package d2skill

// sentryLife gives the level of a sentry's fire skill and how many shots the
// sentry may fire. VERIFIED (MONAI_Sentry_FollowLeaderOrIdle 0x5e9360): on its
// first think the sentry evaluates calc4 of the skill in monstats slot 1, at
// that skill's level on the unit, and counts it down by one for each shot it
// is about to take; at 0 it dies. The level of the monster skill is
// sumsk1calc of the placing skill (SKILL_SpawnSummonedMinion, as for the
// passives). Not known (no trap skill, or an empty calc4) gives 0, 0: the
// engine then keeps its old fixed cadence for that sentry (UNVERIFIED).
func (c *cast) sentryLife() (level, life int) {
	ts := c.p.Skills.ByName(c.sk.SumSkill[1])
	if ts == nil || ts.Calc[4].Empty() {
		return 0, 0
	}

	level = 1
	if c.sk.SumSkCalc[1] != nil {
		level = maxInt(c.env.eval(c.sk.SumSkCalc[1]), 1)
	}

	return level, c.p.env(ts, level, c.u).eval(ts.Calc[4])
}

// calcOrState is SRVDO_EvalSkillCalcOrStateValue (0x5d4c80 area, VERIFIED
// read): for a skill with mask 4 of its flag word 4 set whose aurastate the unit
// currently holds, the value is prgcalc1, 2 or 3 chosen by the base value of
// the state's aurastat1 (at most 1 gives prgcalc1, 2 gives prgcalc2, 3 or more
// prgcalc3); every other case is prgcalc1. Mask 4 of dwBits4 is the skills.txt
// "progressive" column (DATATBL_LoadSkillsTable 0x613450, VERIFIED), the charge-up
// skills, which have their own path (the burst code). Mind Blast and Blade Fury are not progressive, so
// prgcalc1, which this returns, is exact for them.
func (c *cast) calcOrState() int {
	return c.env.eval(c.sk.PrgCalc[1])
}
