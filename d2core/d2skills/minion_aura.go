package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// startMinionAura turns on the aura skill a summoned golem is given (Fire Golem: Holy Fire). The exe adds the
// monster skill to the golem and, for a toggle-state (aura) skill, makes it the golem's active skill, which
// activates it at once (SKILL_SpawnSummonedMinion -> SERVER_SetPlayerActiveSkill 0x56e070 ->
// SKILL_ServerActivateToggleStateSkill, VERIFIED wave 22): the aura state goes on the golem and the periodic
// evaluation runs on the aura period grid for as long as the golem lives. The port pulses the damage aura from
// the golem's position: every enemy within the aura radius takes one damage roll of the aura skill at its
// level, credited to the summoner. UNVERIFIED: the damage is rolled with the summoner's numbers, not the
// golem's; non-damage auras of a minion (none in the shipped table) only log.
func (e *Engine) startMinionAura(p *d2mapentity.Player, u *heroUnit, o *d2skill.SummonOrder, golem *d2mapentity.Monster) {
	ef := e.pipe.MinionAura(u, o.AuraSkill, o.AuraLevel)
	if ef == nil {
		return
	}

	e.emit("state", "STATE minion aura skill=%q level=%d mode=%s radius=%d golem=%s", ef.SkillName, ef.Level, ef.Mode, ef.Radius,
		golem.Label())

	if ef.Mode != "damage" || ef.Desc == nil {
		return
	}

	period := auraPeriodOf(ef)

	var pulse func()

	pulse = func() {
		if !golem.Alive() {
			return
		}

		gx, gy := golem.SubtilePos()
		for _, m := range e.monstersWithin(gx, gy, ef.Radius) {
			if m == golem || e.monsters.IsAlly(m) {
				continue
			}

			mt := e.target(m)
			if !d2skill.FilterAllowsMonster(ef.Filter, mt.IsUndead(), mt.KnockClass() == d2skill.KnockBoss) {
				continue
			}

			dmg := e.rollDesc(u, ef.Desc)
			e.hurt(m, p, &dmg, ef.SkillName)
		}

		e.after(d2skill.NextPeriodicTick(e.frame, period)-e.frame, pulse)
	}

	e.after(d2skill.NextPeriodicTick(e.frame, period)-e.frame, pulse)
}
