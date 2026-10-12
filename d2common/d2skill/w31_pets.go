package d2skill

// SkillPhysNow is the physical damage (whole points) and the to-hit bonus of the named skill at the owner's skill
// level as it stands now, the numbers MONSTER_FillSkillDamageInfo 0x56bdb0 hands to a pet with a monstats
// SkillDamage before every blow (VERIFIED, wave 31). ok is false for an unknown skill or one the owner has not
// learned. The values are those SummonOrder.SkillDmgMin/Max/ToHit carry at cast time.
func (p *Pipeline) SkillPhysNow(u Unit, name string) (dmgMin, dmgMax, toHit int, ok bool) {
	sk := p.Skills.ByName(name)
	if sk == nil {
		return 0, 0, 0, false
	}

	lvl := u.SkillLevel(sk.ID)
	if lvl < 1 {
		return 0, 0, 0, false
	}

	env := p.env(sk, lvl, u)
	wmin, wmax := u.WeaponDamage()

	return int(sk.PhysMin(env, lvl, wmin, true)) >> 8, int(sk.PhysMax(env, lvl, wmax, true)) >> 8, sk.ToHitBonus(env, lvl), true
}
