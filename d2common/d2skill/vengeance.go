package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"

// vengeanceBase is SRVST_VengeanceGetWeaponDamageRange (0x5ce260, VERIFIED wave 23): one roll between the weapon's
// plain minimum and maximum damage stats (whole points shifted to 8.8; no enhanced damage, strength or skill
// percent). A minimum below one point becomes one point and a maximum not above the minimum becomes minimum + 1.
func vengeanceBase(r d2combat.Roller, wmin, wmax int) int32 {
	lo, hi := int32(wmin)<<8, int32(wmax)<<8
	if lo < 1 {
		lo = 1 << 8
	}

	if hi <= lo {
		hi = lo + 1<<8
	}

	return rollRange(r, lo, hi)
}

// vengeanceElem adds the Vengeance elements to the damage of a landed blow (SRVST_035, 0x5ce340, VERIFIED wave 23).
// Each of calc1 (fire), calc2 (cold) and calc3 (lightning) is raised by the unit's matching mastery stat (0x149,
// 0x14b, 0x14a: v + v*mastery/100, only when the calc is not zero) and taken as a percent of ONE bare weapon roll
// (vengeanceBase), not of the physical damage that already carries enhanced damage. The cold length (the skill's
// ELen) is added to the blow. The physical part stays untouched (the elements are added, nothing is converted).
func (p *Pipeline) vengeanceElem(u Unit, sk *Skill, lvl int, env *Env, o strikeOpt, dmg *d2combat.Damage) {
	wmin, wmax := u.WeaponDamage()
	base := vengeanceBase(u.Roller(), wmin, wmax)

	pct := func(c int, stat string) int32 {
		if c != 0 {
			if m := u.Stat(stat); m != 0 {
				c += mulDiv(c, m, 100)
			}
		}

		return int32(mulDiv(int(base), c, 100))
	}

	dmg.Fire += pct(o.elemPct[0], "passive_fire_mastery")
	dmg.Cold += pct(o.elemPct[1], "passive_cold_mastery")
	dmg.Lightning += pct(o.elemPct[2], "passive_ltng_mastery")
	dmg.ColdLen += int32(sk.ElemLen(env, lvl))
}
