package d2skill

// MonsterCalc evaluates calc column n (1..4) of skill sk at skill level lvl for a monster caster of level monLevel
// (wave 36 boss skills: Charged Bolt's bolt count, the Poison Nova ring step, Diablo's wall count). The monster has
// exactly the one skill level and no other skills or stats (monAuraUnit); an empty column gives 0, like the exe's
// SKILL_EvalCalcBytecode on an empty program.
func MonsterCalc(reg *Registry, sk *Skill, lvl, monLevel, n int) int {
	if sk == nil || n < 1 || n >= len(sk.Calc) {
		return 0
	}

	env := NewEnv(sk, lvl, monAuraUnit{skill: sk.ID, lvl: lvl, level: monLevel}, reg)

	return env.eval(sk.Calc[n])
}

// MonsterCalcEmpty reports whether calc column n of sk has no program.
func MonsterCalcEmpty(sk *Skill, n int) bool {
	return sk == nil || n < 1 || n >= len(sk.Calc) || sk.Calc[n].Empty()
}
