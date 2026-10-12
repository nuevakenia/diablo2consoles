package d2skills

// Landed tallies, per skill name, what a skill's own strikes, missiles and area effects reached (w29). The scenario
// harness (OD2_AUTOCAST "!!") uses it to cast a skill again until the skill ITSELF landed something: the global Hits
// counter also counts the late hits of an earlier skill's missile (a Molten Boulder hit that arrives while Twister is
// the current skill made Twister look successful).

// noteLanded records n reached targets for the skill.
func (e *Engine) noteLanded(skill string, n int) {
	if n <= 0 || skill == "" {
		return
	}

	if e.landed == nil {
		e.landed = map[string]int{}
	}

	e.landed[skill] += n
}

// Landed returns how many targets the named skill's strikes, missiles and area effects have reached so far.
func (e *Engine) Landed(skill string) int { return e.landed[skill] }
