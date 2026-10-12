package d2player

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// OD2_AUTOBUILD is a test aid of the full-game playthroughs (wave 43): a player's plan for the points of a level-up,
// carried out through the same rules as the panels (the character panel's + buttons, the skill tree's spend). Format
//
//	OD2_AUTOBUILD="vit=3,str=1|Bash:1,Axe Mastery:20,Bash:20"
//
// The part before | gives the weights of the stat points (str, dex, vit, ene; a point goes to the attribute that is
// furthest behind its weight); the part after | is an ordered wish list of skills with a level cap each: points go
// into the first entry that is below its cap (a name with a trailing ! is an attack: the left button takes the last such
// skill that has points) and that the skills.txt rules allow, as a player clicking the tree would.
// It is a script aid only: nothing here is part of the original game.

type buildSpec struct {
	weights map[string]int
	skills  []buildSkill
}

type buildSkill struct {
	name   string
	cap    int
	attack bool // a trailing ! in the plan: the left button takes the last such skill that has points
}

// ParseAutoBuild parses an OD2_AUTOBUILD value; it returns nil for an empty one.
func ParseAutoBuild(spec string) *buildSpec {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}

	statPart, skillPart, _ := strings.Cut(spec, "|")
	b := &buildSpec{weights: map[string]int{}}

	for _, kv := range strings.Split(statPart, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok {
			continue
		}

		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			b.weights[strings.ToLower(k)] = n
		}
	}

	for _, e := range strings.Split(skillPart, ",") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}

		name, capText, found := strings.Cut(e, ":")
		sk := buildSkill{name: strings.TrimSpace(name), cap: 20}

		if strings.HasSuffix(sk.name, "!") {
			sk.attack, sk.name = true, strings.TrimSpace(strings.TrimSuffix(sk.name, "!"))
		}

		if n, err := strconv.Atoi(capText); found && err == nil {
			sk.cap = n
		}

		b.skills = append(b.skills, sk)
	}

	return b
}

// AutoBuild spends every unspent stat and skill point of the hero by the OD2_AUTOBUILD plan and returns how many of
// each it spent. It does nothing when the variable is unset.
func (g *GameControls) AutoBuild() (stats, skills int) {
	b := ParseAutoBuild(os.Getenv("OD2_AUTOBUILD"))
	if b == nil || g.hero == nil || g.hero.Stats == nil {
		return 0, 0
	}

	st := g.hero.Stats
	given := map[string]int{}

	attr := map[string]*int{"str": &st.Strength, "dex": &st.Dexterity, "vit": &st.Vitality, "ene": &st.Energy}

	for st.StatsPoints > 0 && len(b.weights) > 0 {
		best, bestScore := "", -1.0

		for _, k := range []string{"str", "dex", "vit", "ene"} {
			w := b.weights[k]
			if w == 0 {
				continue
			}

			if score := float64(w) / float64(given[k]+1); score > bestScore {
				best, bestScore = k, score
			}
		}

		if best == "" {
			break
		}

		*attr[best]++
		given[best]++
		st.StatsPoints--
		stats++
	}

	if stats > 0 && st.Recalc != nil {
		st.Recalc()
	}

	for st.SkillPoints > 0 {
		spent := false

		for _, w := range b.skills {
			id, err := g.skillIDByName(w.name)
			if err != nil || g.hero.Skills[id] == nil || g.hero.Skills[id].SkillPoints >= w.cap {
				continue
			}

			if g.SpendSkillPoint(id) == nil {
				skills++
				spent = true

				break
			}
		}

		if !spent {
			break
		}
	}

	if stats > 0 {
		g.heroStatsPanel.SetStatsPointsChanged()
	}

	// the left button: the last attack skill of the plan that has points
	for _, w := range b.skills {
		if id, err := g.skillIDByName(w.name); w.attack && err == nil && g.hero.Skills[id] != nil &&
			g.hero.Skills[id].SkillPoints > 0 && g.hero.LeftSkill != nil && g.hero.LeftSkill.ID != id {
			_ = g.SelectSkill(true, id)
		}
	}

	g.Infof("AUTOBUILD spent stats=%s skills=%d left stat_points=%d skill_points=%d recalc=%v maxlife=%d", fmt.Sprint(given), skills,
		st.StatsPoints, st.SkillPoints, st.Recalc != nil, st.MaxHealth)

	return stats, skills
}
