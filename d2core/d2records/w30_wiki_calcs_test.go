package d2records

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// Calc columns that carry counts and radii (wiki: Chain Lightning hits 5 / 7 / 9, Charged Bolt 3 / 12 / 22 bolts,
// Dragon Talon kicks 1 / 2 / 4). Compared with d2emu / wikiwiki rows, see w30-wiki-skills-b.md.
func TestW30CalcColumns(t *testing.T) {
	reg := loadRealRecords(t).SkillTable()

	for _, tc := range []struct {
		skill string
		calc  int // 1-based
		lvl   int
		want  int
	}{
		{"Chain Lightning", 1, 1, 5}, {"Chain Lightning", 1, 10, 7}, {"Chain Lightning", 1, 20, 9},
		{"Charged Bolt", 1, 1, 3}, {"Charged Bolt", 1, 10, 12}, {"Charged Bolt", 1, 20, 22},
		{"Dragon Talon", 1, 1, 1}, {"Dragon Talon", 1, 10, 2}, {"Dragon Talon", 1, 20, 4},
		// Meteor and Blizzard radius in subtiles: 6 (4 yards) and 7 (4.6 yards)
		{"Meteor", 1, 20, 6}, {"Blizzard", 1, 20, 7},
		// Raven monster level = ulvl + par1 + lvl (ulvl 30 in the unit used here)
		{"Raven", 2, 1, 29}, {"Raven", 2, 20, 48},
		// vines and totems: life percent (lvl-1) * par
		{"Plague Poppy", 1, 10, 225}, {"Vines", 1, 20, 380}, {"Cycle of Life", 1, 20, 475},
		{"Oak Sage", 1, 10, 270}, {"Oak Sage", 1, 20, 570},
	} {
		sk := reg.ByName(tc.skill)
		u := &w30Unit{reg: reg, levels: map[string]int{tc.skill: tc.lvl}}
		e := d2skill.NewEnv(sk, tc.lvl, u, reg)

		if got := e.Eval(sk.Calc[tc.calc]); got != tc.want {
			t.Errorf("%s calc%d L%d = %d, want %d (%s)", tc.skill, tc.calc, tc.lvl, got, tc.want, sk.Calc[tc.calc].Source())
		}
	}
}

// The totem auras the Oak Sage family carries (monster skills 298, 297, 296): wiki max life 30 / 75 / 125 percent,
// damage 20 / 83 / 153 and attack rating 25 / 88 / 158 percent, aura range 30 / 48 / 68 subtiles (20 / 32 / 45.3
// yards). Thorns 50 / 140 / 240 percent are the 1.14b numbers of Spirit of Barbs; patch 2.4 made it flat damage,
// so wiki rows of that skill differ and are not engine errors.
func TestW30TotemAuras(t *testing.T) {
	reg := loadRealRecords(t).SkillTable()

	for _, tc := range []struct {
		skill string
		stat  string
		vals  [3]int
	}{
		{"Oak Sage Aura", "item_maxhp_percent", [3]int{30, 75, 125}},
		{"Wolverine Aura", "damagepercent", [3]int{20, 83, 153}},
		{"Wolverine Aura", "item_tohit_percent", [3]int{25, 88, 158}},
		{"Barbs Aura", "thorns_percent", [3]int{50, 140, 240}},
	} {
		sk := reg.ByName(tc.skill)

		for i, lvl := range []int{1, 10, 20} {
			e := d2skill.NewEnv(sk, lvl, &w30Unit{reg: reg, levels: map[string]int{tc.skill: lvl}}, reg)
			found := false

			for j := 1; j <= 6; j++ {
				if sk.AuraStat[j] == tc.stat {
					found = true

					if got := e.Eval(sk.AuraStatCalc[j]); got != tc.vals[i] {
						t.Errorf("%s L%d %s = %d, want %d", tc.skill, lvl, tc.stat, got, tc.vals[i])
					}
				}
			}

			if !found {
				t.Errorf("%s has no %s", tc.skill, tc.stat)
			}

			if got := e.Eval(sk.AuraRangeCalc); got != [3]int{30, 48, 68}[i] {
				t.Errorf("%s L%d aura range %d", tc.skill, lvl, got)
			}
		}
	}
}

// Static Field radius: wiki 3.3 / 9.3 / 16 yards = 5 / 14 / 24 subtiles. The effect carries the radius.
func TestW30StaticFieldRadius(t *testing.T) {
	for lvl, want := range map[int]int{1: 5, 10: 14, 20: 24} {
		r := realCast(t, "Static Field", lvl)
		got := -1

		for _, e := range r.res.Effects {
			if e.Kind == "area_damage" {
				got = e.Radius
			}
		}

		if got != want {
			t.Errorf("Static Field L%d radius %d subtiles, want %d", lvl, got, want)
		}
	}
}
