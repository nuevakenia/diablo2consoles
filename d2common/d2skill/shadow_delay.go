package d2skill

// DelayCalc evaluates the skills.txt delay calc of a skill at a level for a caster, in frames (0 when the skill has
// none). The Assassin's Shadow Warrior uses it for its recast gate (calc / 3 + 18 frames, 0x5e9ff0).
func (p *Pipeline) DelayCalc(u Unit, id, lvl int) int {
	sk := p.Skills.ByID(id)
	if sk == nil || sk.Delay == nil {
		return 0
	}

	return p.env(sk, lvl, u).eval(sk.Delay)
}
