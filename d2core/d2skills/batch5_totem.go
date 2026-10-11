package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// totemPulse is one second of a druid totem aura (Oak Sage, Heart of Wolverine, Spirit of Barbs). The aura skill
// (Oak Sage Aura 298, Wolverine Aura 297, Barbs Aura 296) runs SRVDO_065 with aurafilter 65795 = 0x10103, which
// SKILL_UnitMatchesTargetFilter (0x569100, VERIFIED) reads as: players and monsters, not in a town room, allied to
// the caster. So the aura reaches the hero, the summoned minions and the mercenary alike, each inside the
// circle of aurarangecalc (squared distance <= radius squared, 0x569510). Every one that passes gets the state,
// the ones outside lose it. Minions turn the stats into an AuraBoost (damage, attack rating and life percent);
// the hero and the mercenary read the state like any other.
func (e *Engine) totemPulse(p *d2mapentity.Player, u *heroUnit, sk *d2skill.Skill, ef *d2skill.Effect, filter int, name string,
	radius int, mods []d2state.StatMod, totem *d2mapentity.Monster) {
	tx, ty := totem.SubtilePos()
	until := e.frame + auraPulse + 3
	inst := d2state.Instance{Name: name, Until: until, Mods: mods, Source: p.ID(), SkillID: sk.ID, Level: ef.Level}

	reach := func(x, y int, kind d2skill.UnitKind) bool {
		if radius > 0 && !inAuraRange(tx, ty, x, y, radius) {
			return false
		}

		return d2skill.FilterMatch(filter, d2skill.FilterFacts{Kind: kind, Allied: true})
	}

	hx, hy := u.Pos()
	if reach(hx, hy, d2skill.KindPlayer) {
		e.setOf(p.ID()).Apply(e.frame, inst)
	} else {
		e.setOf(p.ID()).Remove(name)
	}

	if tok := e.monsters.MercToken(p); tok != 0 && e.monsters.MercAlive(p, tok) {
		mx, my := e.monsters.MercPosition(p)
		id := mercID(p, tok)

		if reach(mx, my, d2skill.KindMonster) {
			e.setOf(id).Apply(e.frame, inst)
		} else {
			e.setOf(id).Remove(name)
		}
	}

	for _, pet := range e.pets[p.ID()] {
		if !pet.Alive() || pet == totem {
			continue
		}

		if kind, _ := e.monsters.MinionKind(pet); kind != "minion" {
			continue
		}

		px, py := pet.SubtilePos()
		if reach(px, py, d2skill.KindMonster) {
			e.setOf(pet.ID()).Apply(e.frame, inst)
			e.monsters.SetAuraBoost(pet, boostOf(mods))
		} else {
			e.setOf(pet.ID()).Remove(name)
			e.monsters.SetAuraBoost(pet, d2monsters.AuraBoost{})
		}
	}
}

// boostOf turns the stats of a totem aura into what a minion can use.
func boostOf(mods []d2state.StatMod) d2monsters.AuraBoost {
	var b d2monsters.AuraBoost

	for _, m := range mods {
		switch m.Stat {
		case "damagepercent":
			b.DamagePct += m.Value
		case "item_tohit_percent":
			b.ToHitPct += m.Value
		case "item_maxhp_percent":
			b.MaxHPPct += m.Value
		}
	}

	return b
}

// totemClear takes the aura state off everyone when its totem is gone.
func (e *Engine) totemClear(p *d2mapentity.Player, name string) {
	e.setOf(p.ID()).Remove(name)

	if tok := e.monsters.MercToken(p); tok != 0 {
		e.setOf(mercID(p, tok)).Remove(name)
	}

	for _, pet := range e.pets[p.ID()] {
		e.setOf(pet.ID()).Remove(name)
		e.monsters.SetAuraBoost(pet, d2monsters.AuraBoost{})
	}
}

// allyAuraPulse extends a hero's friendly aura (SRVDO_065, aurafilter 73731 = players, monsters, allied; Meditation's
// 73729 is players only) to the mercenary and the summoned minions inside aurarangecalc, the same reach rule the
// druid totems use (SKILL_UnitMatchesTargetFilter 0x569100 + SKILL_ForEachUnitInRadius 0x569510, VERIFIED). The
// ones inside get the state (a minion turns damagepercent / item_tohit_percent / item_maxhp_percent into an
// AuraBoost), the ones outside lose it. UNVERIFIED: that a minion reads only those three stats; hitpoints healing
// of Prayer is not given to minions.
func (e *Engine) allyAuraPulse(p *d2mapentity.Player, hx, hy int, ef *d2skill.Effect, mods []d2state.StatMod, until int, on bool) {
	inst := d2state.Instance{Name: ef.State, Until: until, Mods: mods, Source: p.ID(), SkillID: ef.SkillID, Level: ef.Level}

	reach := func(x, y int) bool {
		if ef.Radius > 0 && !inAuraRange(hx, hy, x, y, ef.Radius) {
			return false
		}

		return d2skill.FilterMatch(ef.Filter, d2skill.FilterFacts{Kind: d2skill.KindMonster, Allied: true})
	}

	for _, pet := range e.pets[p.ID()] {
		if !pet.Alive() {
			continue
		}

		if kind, _ := e.monsters.MinionKind(pet); kind != "minion" {
			continue
		}

		px, py := pet.SubtilePos()
		if on && reach(px, py) {
			e.setOf(pet.ID()).Apply(e.frame, inst)
			e.monsters.SetAuraBoost(pet, boostOf(mods))
		} else {
			e.setOf(pet.ID()).Remove(ef.State)
			e.monsters.SetAuraBoost(pet, d2monsters.AuraBoost{})
		}
	}
}
