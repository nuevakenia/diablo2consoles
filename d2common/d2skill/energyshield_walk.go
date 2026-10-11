package d2skill

// EnergyShieldWalk is the per-type loop of the Energy Shield absorb callback (SKILL_ApplyManaShieldAbsorption
// 0x5c8840, VERIFIED). dmg holds the damage-struct values in the order of EnergyShieldTypes (physical, fire,
// lightning, cold, magic, then the three leech-style fields at struct offsets 0x38 / 0x3c / 0x40 that only count
// when the attacker is a monster that is not a hireling, or there is no attacker). Each type with a positive
// value loses min(value*pct/100, mana*16/ratio) where mana is what is left after the earlier types, and the mana
// drops by absorbed*ratio/16 (floor 0). It returns the absorbed amount per type, the mana left and whether the
// shield is gone (mana left below 1). The caller has already checked pct > 0. ratio below 1 is 1.
func EnergyShieldWalk(dmg [8]int, pct, ratio, mana int, attackerIsPlayerOrHireling bool) (absorbed [8]int, manaLeft int, ended bool) {
	if ratio < 1 {
		ratio = 1
	}

	for i, t := range EnergyShieldTypes {
		if t.NotVsPlayer && attackerIsPlayerOrHireling || dmg[i] <= 0 {
			continue
		}

		a := mulDiv(dmg[i], pct, 100)
		if limit := mulDiv(mana, 16, ratio); limit < a {
			a = limit
		}

		if a < 0 {
			a = 0
		}

		absorbed[i] = a

		if mana -= mulDiv(a, ratio, 16); mana < 0 {
			mana = 0
		}
	}

	return absorbed, mana, mana < 1
}
