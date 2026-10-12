package d2skill

// MonsterCurse is what the Cursed monster modifier (monumod 7) puts on a hero: the exe's hook 0x5a0060 evaluates the
// Amplify Damage row (skill 0x42) at a given skill level and gives every hostile unit within Radius of the monster's
// target a timed state for Frames frames carrying Stats (0x59fef0: aurastat pairs 1..5, zero values skipped).
type MonsterCurse struct {
	State  string
	Radius int
	Frames int
	Stats  []StatMod
}

// MonsterCurseSkill is the skill id the Cursed hook is bound to (Amplify Damage).
const MonsterCurseSkill = 66

// MonsterCurseOf evaluates the curse skill sk at skill level lvl for a monster of level monLevel. The radius is the
// aurarangecalc as is (the caller clamps it with d2monreg.CurseRadius), Frames the auralencalc. nil for an unknown skill.
func MonsterCurseOf(reg *Registry, sk *Skill, lvl, monLevel int) *MonsterCurse {
	if reg == nil || sk == nil || lvl < 1 {
		return nil
	}

	env := NewEnv(sk, lvl, monAuraUnit{skill: sk.ID, lvl: lvl, level: monLevel}, reg)

	out := &MonsterCurse{State: sk.AuraTargetState, Radius: env.eval(sk.AuraRangeCalc), Frames: env.eval(sk.AuraLenCalc)}

	for i := 1; i <= 5; i++ {
		if sk.AuraStat[i] == "" {
			continue
		}

		if v := env.eval(sk.AuraStatCalc[i]); v != 0 {
			out.Stats = append(out.Stats, StatMod{Stat: sk.AuraStat[i], Value: v})
		}
	}

	return out
}
