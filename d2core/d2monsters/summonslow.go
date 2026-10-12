package d2monsters

// slowMeleeAttacker is the Clay Golem's damage event (auraevent damagedinmelee, function 27 = 0x5c46d0, VERIFIED
// wave 23): the monster u that landed a melee hit of dmg on the summon tu is slowed by the summon's item_slow stat
// (its aurastat1). Missile hits and blocked blows do nothing.
func (d *Director) slowMeleeAttacker(u, tu *unit, dmg int, ranged bool) {
	if d.SummonSlow == nil || dmg <= 0 || ranged || tu.ally == nil || tu.ally.opt.Stats == nil {
		return
	}

	if pct := tu.ally.opt.Stats.Other["item_slow"]; pct > 0 {
		d.SummonSlow(u.m, pct)
	}
}
