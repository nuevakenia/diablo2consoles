package d2state

// Slow clamp of the aura applier SRVDO_ApplyAuraTimedStateAndStats 0x5cd240 (V, wave 22; Holy Freeze). After the curse
// reduction (curse_reduction.go) the six aura stat slots are put on the target one by one in slot order, and
// for the stats 0x43 (velocitypercent) and 0x44 (attackrate) the value is raised to at least the target's cold
// effect (COMBAT_GetMonsterColdResistByDifficulty: monstats ColdEffect of the game's difficulty
// for a monster, -50 for any other unit): a slow can never go below the ColdEffect, and a monster with ColdEffect 0
// is not slowed at all. Stat 0x44 also writes the same value into stat 0x45 (other_animrate); a later slot that
// names other_animrate (Holy Freeze's third stat) overwrites it with its own unclamped value, because the clamp
// only covers the ids 0x43 and 0x44. Other stats pass unchanged (a zero slot is skipped by the exe; keeping it changes nothing).

// Aura stat names the clamp looks at.
const (
	statVelocity  = "velocitypercent"
	statAttack    = "attackrate"
	statAnimSpeed = "other_animrate"
)

// ClampAuraSlow returns the stat list the aura applier really writes for a target whose cold effect is coldEffect
// (a copy; mods is not changed). Entries are kept in slot order, a stat that is written twice keeps the first
// position and the last value.
func ClampAuraSlow(mods []StatMod, coldEffect int) []StatMod {
	out := make([]StatMod, 0, len(mods)+1)

	set := func(name string, v int) {
		for i := range out {
			if out[i].Stat == name {
				out[i].Value = v

				return
			}
		}

		out = append(out, StatMod{Stat: name, Value: v})
	}

	for _, m := range mods {
		v := m.Value

		switch m.Stat {
		case statVelocity:
			set(statVelocity, maxOf(v, coldEffect))
		case statAttack:
			c := maxOf(v, coldEffect)
			set(statAttack, c)
			set(statAnimSpeed, c)
		default:
			set(m.Stat, v)
		}
	}

	return out
}

func maxOf(a, b int) int {
	if a > b {
		return a
	}

	return b
}
