package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"

// monAuraUnit is the minimal caster the passive calcs of an aura skill need when a monster (Aura Enchanted,
// monumod 30) holds the skill: it has exactly the level the modifier gave it in that skill and no other skill
// levels or stats. Every other Unit method is left unset (the calcs of the aura skills do not reach them).
type monAuraUnit struct {
	Unit
	skill, lvl, level int
}

func (m monAuraUnit) ID() string     { return "monster" }
func (m monAuraUnit) IsPlayer() bool { return false }
func (m monAuraUnit) Level() int     { return m.level }

func (m monAuraUnit) SkillLevel(id int) int {
	if id == m.skill {
		return m.lvl
	}

	return 0
}

func (m monAuraUnit) BaseSkillLevel(id int) int { return m.SkillLevel(id) }
func (m monAuraUnit) Stat(string) int           { return 0 }
func (m monAuraUnit) Roller() d2combat.Roller   { return nil }

// MonsterAuraStats are the stats a monster that holds the aura skill sk at level lvl gets on its own state
// (SRVDO_065 / 066 / 081, see auraPassiveStats). monLevel is the monster's level. Skills that are not auras of
// those three kinds give nothing. For a friendly aura the aurastat pairs are included as well.
func MonsterAuraStats(reg *Registry, sk *Skill, lvl, monLevel int) []StatMod {
	if sk == nil || lvl < 1 {
		return nil
	}

	env := NewEnv(sk, lvl, monAuraUnit{skill: sk.ID, lvl: lvl, level: monLevel}, reg)

	out := auraPassiveStats(env, sk)

	// A friendly aura (srvdofunc 65) also puts its aurastat pairs on every unit it covers; the caster is among
	// them. A monster takes them on itself (allies in range are not modelled, UNVERIFIED for monsters).
	if sk.SrvDoFunc == 65 {
		for i := 1; i <= 6; i++ {
			if sk.AuraStat[i] == "" {
				continue
			}

			dup := false

			for _, o := range out { // the caster's own passive of the same stat already covers it
				dup = dup || o.Stat == sk.AuraStat[i]
			}

			if v := env.eval(sk.AuraStatCalc[i]); v != 0 && !dup {
				out = append(out, StatMod{Stat: sk.AuraStat[i], Value: v})
			}
		}
	}

	return out
}
