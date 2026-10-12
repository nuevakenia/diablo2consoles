package d2skill

// auraDoFuncs are the do functions of the aura skills (SRVDO_065 / 066 / 081 / 082): the skills a summoned
// minion can be given as a running aura.
func auraDoFunc(f int) bool { return f == 65 || f == 66 || f == 81 || f == 82 }

// minionAura finds the aura among the monster skills of a summon (sumskill1..5 with a positive sumskNcalc) and
// returns its name and level. SKILL_SpawnSummonedMinion (0x5c1f20 neighbour, read wave 22) adds each skill to the
// minion and, when the skill has bit 0x20 of its flag word 4, makes it the minion's active skill, which runs
// SKILL_ServerActivateToggleStateSkill at once: the aura state goes on and the periodic evaluation starts.
// UNVERIFIED: which skills.txt column sets bit 0x20; the port takes "has an aura state, is not passive and runs
// an aura do function". Fire Golem: Holy Fire at sumsk1calc.
func (c *cast) minionAura() (string, int) {
	for i := 1; i <= 5; i++ {
		name := c.sk.SumSkill[i]
		if name == "" || c.sk.SumSkCalc[i] == nil {
			continue
		}

		ms := c.p.Skills.ByName(name)
		if ms == nil || ms.Passive || ms.AuraState == "" || !auraDoFunc(ms.SrvDoFunc) {
			continue
		}

		if lvl := c.env.eval(c.sk.SumSkCalc[i]); lvl >= 1 {
			return name, lvl
		}
	}

	return "", 0
}

// MinionAura evaluates the aura skill a minion runs (name, from SummonOrder.AuraSkill) at its level, with the
// summoner u as the numbers' owner (UNVERIFIED: the exe evaluates with the monster; the shipped Holy Fire calcs
// only read the skill level and the damage columns). It returns the aura effect, or nil.
func (p *Pipeline) MinionAura(u Unit, name string, lvl int) *Effect {
	sk := p.Skills.ByName(name)
	if sk == nil || lvl < 1 || !auraDoFunc(sk.SrvDoFunc) {
		return nil
	}

	res := DoResult{OK: true, Level: lvl}
	p.runDo(u, sk, lvl, Target{}, p.env(sk, lvl, u), &res)

	for i := range res.Effects {
		if res.Effects[i].Kind == "aura" {
			ef := res.Effects[i]
			ef.Level, ef.SkillID, ef.SkillName = lvl, sk.ID, sk.Name

			return &ef
		}
	}

	return nil
}

// CreateAtTargetInRange is the distance test of SKILL_ServerCreateMissileAtTarget (0x5c8560 callers: Meteor,
// Blizzard, Eruption, Volcano; VERIFIED wave 22): the missile is only created when the distance from the
// caster to the aim point (UNIT_GetDistanceToPoint, the octile metric max + min/2 on subtiles) is below 0x65.
func CreateAtTargetInRange(fromX, fromY, toX, toY int) bool {
	dx, dy := fromX-toX, fromY-toY
	if dx < 0 {
		dx = -dx
	}

	if dy < 0 {
		dy = -dy
	}

	if dx < dy {
		dx, dy = dy, dx
	}

	return dx+dy/2 < 0x65
}
