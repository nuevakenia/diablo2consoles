package d2skill

// auraPassiveStats returns the passivestat1..5 values that ride on the caster's aura state.
//
// VERIFIED in the exe: SRVDO_065 (0x5cd4b0) evaluates the passivestat/passivecalc pairs onto the aura's own
// stat list when the aura state was applied, the mana covers the upkeep and the skill has no passivestate
// (Fanaticism's damagepercent). SRVDO_066 (0x5cd880, Holy Fire / Holy Shock / Sanctuary) and SRVDO_081
// (0x5cee90, Holy Freeze) put the five pairs on the caster's state unconditionally, for a player only while the
// mana covers the upkeep (the elemental damage added to the hero's blows). Zero values and unset stats are
// skipped (0x5cd4b0 does; for 66 / 81 that is UNVERIFIED but harmless). Skills with a passivestate (Resist x,
// Blessed Aim) keep their passive stats on the permanent passive path instead.
func auraPassiveStats(env *Env, sk *Skill) []StatMod {
	switch sk.SrvDoFunc {
	case 65:
		if sk.PassiveState != "" {
			return nil
		}
	case 66, 81:
	default:
		return nil
	}

	var out []StatMod

	for i := 1; i <= 5; i++ {
		if sk.PassiveStat[i] == "" {
			continue
		}

		if v := env.eval(sk.PassiveCalc[i]); v != 0 {
			out = append(out, StatMod{Stat: sk.PassiveStat[i], Value: v})
		}
	}

	return out
}
