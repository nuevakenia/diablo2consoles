package d2skill

// levelUnit lets a monster skill run at the level its sentry has, while the owner's stats and skills stay the
// hero's (the sentry's skills are not in the hero's skill list).
type levelUnit struct {
	Unit
	id, lvl int
}

func (l levelUnit) SkillLevel(id int) int {
	if id == l.id {
		return l.lvl
	}

	return l.Unit.SkillLevel(id)
}

func (l levelUnit) BaseSkillLevel(id int) int {
	if id == l.id {
		return l.lvl
	}

	return l.Unit.BaseSkillLevel(id)
}

// CastSentrySkill runs the do function of a monster skill that a hero's sentry casts (Death Sentry's
// "mon death sentry", skills.txt srvdofunc 55) at the level of the sentry's skill, with the hero as the owner.
// It returns the result for the engine to apply like a hero's do. No mana, cooldown or ammo is charged. The
// unit that casts in the exe is the sentry (its level was set to the summon level, SRVDO_044/045); the port
// uses the hero's level for the level-based scaling (UNVERIFIED).
func (p *Pipeline) CastSentrySkill(u Unit, skillID, level int, tgt Target) DoResult {
	sk := p.Skills.ByID(skillID)
	if sk == nil || level < 1 {
		return DoResult{Reason: ReasonNoSkill}
	}

	lu := levelUnit{Unit: u, id: skillID, lvl: level}
	res := DoResult{OK: true, Level: level}
	env := p.env(sk, level, lu)

	p.runDo(lu, sk, level, tgt, env, &res)

	return res
}

// LinearValue is SKILLS_CalcLinearValueByLevel (0x4c9220, VERIFIED): (level-1)*Param4 + Param3, 0 for a level
// below 1 or an unknown skill. The Death Sentry AI reads it for Skill1 as the corpse range.
func (p *Pipeline) LinearValue(skillID, level int) int {
	sk := p.Skills.ByID(skillID)
	if sk == nil || level < 1 {
		return 0
	}

	return (level-1)*sk.Params[4] + sk.Params[3]
}
