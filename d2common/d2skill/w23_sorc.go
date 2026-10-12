package d2skill

// StaticFieldResistComp is the tail of the Static Field callback (0x5c7750,
// VERIFIED wave 23). After the damage is known (8.8 fixed point), the callback
// reads the target's resistance of the skill's damage type; when it is below
// zero the damage is divided by (100 - res) / 100 before the damage struct is
// built, so the later resist step (which multiplies by the same factor) gives
// back the intended amount. A negative resistance therefore never raises
// Static Field; zero and positive resistances are left to the normal resist
// step.
func StaticFieldResistComp(dmg8, res int) int {
	if res >= 0 {
		return dmg8
	}

	return dmg8 * 100 / (100 - res)
}

// Calc1 evaluates calc1 of a skill for a unit at a level (the exe evaluates it
// at the wearer's level when an armor event fires, e.g. the freeze length of
// Frozen Armor, handler 0x5c8e20).
func (p *Pipeline) Calc1(u Unit, skillID, lvl int) int {
	sk := p.Skills.ByID(skillID)
	if sk == nil {
		return 0
	}

	return p.env(sk, lvl, u).eval(sk.Calc[1])
}

// SrvMissileA is the srvmissilea column of a skill ("" when unknown).
func (p *Pipeline) SrvMissileA(skillID int) string {
	sk := p.Skills.ByID(skillID)
	if sk == nil {
		return ""
	}

	return sk.SrvMissileA
}
