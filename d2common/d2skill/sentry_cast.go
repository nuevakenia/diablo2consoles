package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"

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

// CastTrapBolts is a Charged Bolt Sentry's shot (w23): the monster skill "BoltSentry" has do function 17
// (SRVDO_ChargedBolt 0x5c73a0), so each shot throws calc1 of that skill (at the sentry's level of it, evaluated for the
// hero as owner) bolts of its srvmissilea through the path type 10 hook, all from the sentry's cell. The damage is
// the placing skill's (Charged Bolt Sentry), as for the other sentries (U). It returns the bolts made.
func (p *Pipeline) CastTrapBolts(u Unit, placingID, monsterSkillID, monsterLevel, fromX, fromY int, tgt Target) []*d2missile.Missile {
	pl, ms := p.Skills.ByID(placingID), p.Skills.ByID(monsterSkillID)
	if pl == nil || ms == nil {
		return nil
	}

	name := ms.SrvMissileA
	if name == "" {
		name = ms.SrvMissile
	}

	lvl := u.SkillLevel(placingID)
	env := p.env(pl, lvl, u)
	n := p.env(ms, maxInt(monsterLevel, 1), u).eval(ms.Calc[1])

	ax, ay := tgt.X, tgt.Y
	if tgt.Unit != nil {
		ax, ay = tgt.UX, tgt.UY
	}

	var out []*d2missile.Missile

	for i := 0; i < n && ax != 0 && ay != 0; i++ {
		if m := p.castMissile(u, pl, lvl, env, name, tgt, castOpts{hasStart: true, startX: float64(fromX) + 0.5,
			startY: float64(fromY) + 0.5, pathType10: true, pathSeed: ChargedBoltSeed(ax, i)}); m != nil {
			out = append(out, m)
		}
	}

	return out
}
