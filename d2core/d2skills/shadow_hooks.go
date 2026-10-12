package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// Engine side of the Assassin's shadows (w25-shadows): the shadows cast through the hireling path (castMerc), and the
// AI reads their state lists and the skill delay through these two hooks.

// shadowState answers MercSkillHooks.State: the state list of the shadow with that token, or of the monster m.
func (e *Engine) shadowState(owner *d2mapentity.Player, token uint32, m *d2mapentity.Monster, state string) d2monsters.MercStateView {
	var id string

	switch {
	case m != nil:
		id = m.ID()
	case owner != nil:
		id = mercID(owner, token)
	default:
		return d2monsters.MercStateView{}
	}

	set := e.setOf(id)
	in := set.Get(e.frame, state)
	view := d2monsters.MercStateView{Active: in != nil}

	if in != nil {
		view.Stats = map[string]int{}
		for _, mod := range in.Mods {
			view.Stats[mod.Stat] += mod.Value
		}
	}

	defs := e.stateDefs()
	if d, ok := defs[state]; ok && d.Group != 0 {
		for _, n := range set.Names(e.frame) {
			if x, ok := defs[n]; ok && n != state && x.Group == d.Group {
				view.OtherInGroup = true

				break
			}
		}
	}

	return view
}

// shadowDelay answers MercSkillHooks.Delay: the delay calc of the skill of a cast at its level.
func (e *Engine) shadowDelay(c d2monsters.MercCast) int {
	sk := e.pipe.Skills.ByName(c.Skill)
	if sk == nil {
		return 0
	}

	h := e.mercCaster(c)
	h.merc.skills[sk.ID] = c.SkillLevel

	return e.pipe.DelayCalc(h, sk.ID, c.SkillLevel)
}
