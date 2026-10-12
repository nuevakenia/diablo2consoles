package d2records

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// Wave 31: the skills the wiki checks did not cover. Numbers come from the shipped tables (D2_TABLES) and are tied to
// the exe reads in ~/git/d2-re-notes/w31-skills-rest.md. All tests skip without D2_TABLES.

// DamageRate (missile stat 0x147): 0x579ef0 scales the defender's flat reduction by rate / 1024 per hit. The standing
// fires carry it so that 25 hits a second pay the reduction once (41 / 1024 is about 1 / 25).
func TestW31DamageRateColumn(t *testing.T) {
	mt := loadRealRecords(t).MissileTable()

	for name, want := range map[string]int{
		"firewall": 41, "armageddonfire": 41, "infernoflame1": 82, "infernoflame2": 82, "inferno sentry 1": 82,
		"wake of destruction": 205, "wake of destruction maker": 205, "nova": 0, "volcano small fire": 0,
	} {
		sp := mt.ByName(name)
		if sp == nil {
			t.Fatalf("no missile %q", name)
		}

		if sp.DamageRate != want {
			t.Errorf("%s: DamageRate %d, want %d", name, sp.DamageRate, want)
		}
	}
}

// SRVDO_124 (0x5c6200, Armageddon and Hurricane): the state lasts auralencalc frames, at least 1: ln12 (Param1 250,
// Param2 0) plus 50 frames (2 s) per level of the synergy skill; Blade Shield (ln12 500 + 125 per level) and
// Blade Sentinel (calc4 ln12, 100 + 12 per level) read their durations the same way.
func TestW31Durations(t *testing.T) {
	reg := loadRealRecords(t).SkillTable()

	for _, tc := range []struct {
		skill  string
		lvl    int
		syn    map[string]int
		which  string
		frames int
	}{
		{"Armageddon", 1, nil, "len", 250}, {"Armageddon", 20, nil, "len", 250},
		{"Armageddon", 20, map[string]int{"Eruption": 10}, "len", 750},
		{"Hurricane", 1, nil, "len", 250}, {"Hurricane", 20, map[string]int{"Cyclone Armor": 5}, "len", 500},
		{"Blade Shield", 1, nil, "len", 500}, {"Blade Shield", 10, nil, "len", 1625}, {"Blade Shield", 20, nil, "len", 2875},
		{"Blade Sentinel", 1, nil, "calc4", 100}, {"Blade Sentinel", 10, nil, "calc4", 208}, {"Blade Sentinel", 20, nil, "calc4", 328},
		{"Volcano", 20, nil, "calc4", 2},
	} {
		sk := reg.ByName(tc.skill)
		u := &w30Unit{reg: reg, levels: map[string]int{tc.skill: tc.lvl}}

		for k, v := range tc.syn {
			u.levels[k] = v
		}

		e := d2skill.NewEnv(sk, tc.lvl, u, reg)

		got := 0
		if tc.which == "len" {
			got = e.Eval(sk.AuraLenCalc)
		} else {
			got = e.Eval(sk.Calc[4])
		}

		if got != tc.frames {
			t.Errorf("%s L%d %v %s = %d, want %d", tc.skill, tc.lvl, tc.syn, tc.which, got, tc.frames)
		}
	}

	// the radii: Armageddon drops rocks within Param3 (8), Hurricane hits within Param3 (9), Volcano's debris range is
	// Param1 (12), Blade Shield strikes within Param4 (6)
	for name, want := range map[string]int{"Armageddon": 8, "Hurricane": 9, "Volcano": 12, "Blade Shield": 6} {
		sk := reg.ByName(name)
		u := &w30Unit{reg: reg, levels: map[string]int{name: 10}}

		if got := d2skill.NewEnv(sk, 10, u, reg).Eval(sk.AuraRangeCalc); got != want {
			t.Errorf("%s radius %d, want %d", name, got, want)
		}
	}
}

// MONAI_Sentry_FollowLeaderOrIdle 0x5e9360: a sentry has a SHOT budget, calc4 of the monstats slot 1 skill at the
// level sumsk1calc of the placing skill. The numbers are the table's: BoltSentry bolts per shot ln12 + Shock Field / 3,
// shots par8 + Lightning Sentry / 4.
func TestW31SentryShotBudgets(t *testing.T) {
	reg := loadRealRecords(t).SkillTable()

	for _, tc := range []struct {
		skill string // the monster skill in monstats slot 1
		which int    // calc index
		syn   map[string]int
		want  int
	}{
		{"BoltSentry", 4, nil, 5}, {"BoltSentry", 4, map[string]int{"Lightning Sentry": 12}, 8},
		{"BoltSentry", 1, nil, 5}, {"BoltSentry", 1, map[string]int{"Shock Field": 9}, 8},
		{"sentry lightning", 4, nil, 10},
		{"mon inferno sentry", 4, nil, 10}, {"mon inferno sentry", 1, map[string]int{"Wake of Fire Sentry": 4}, 24},
		{"mon death sentry", 4, nil, 5}, {"mon death sentry", 4, map[string]int{"Fire Trauma": 9}, 8},
		{"Wake Of Destruction Sentry", 4, nil, 5},
	} {
		sk := reg.ByName(tc.skill)
		if sk == nil {
			t.Fatalf("no skill %q", tc.skill)
		}

		u := &w30Unit{reg: reg, levels: map[string]int{tc.skill: 10}}

		for k, v := range tc.syn {
			u.levels[k] = v
		}

		if got := d2skill.NewEnv(sk, 10, u, reg).Eval(sk.Calc[tc.which]); got != tc.want {
			t.Errorf("%s calc%d %v = %d, want %d (%s)", tc.skill, tc.which, tc.syn, got, tc.want, sk.Calc[tc.which].Source())
		}
	}
}

// Blade Fury: the gap between throws is prgcalc1 = par4 = 5 frames (SRVDO_048 0x5d5260); Inferno's flame range calc1
// is ln12 / 2 (10 / 23 / 38, SRVDO_019).
func TestW31FuryAndInfernoCalcs(t *testing.T) {
	reg := loadRealRecords(t).SkillTable()

	bf := reg.ByName("Blade Fury")
	if got := d2skill.NewEnv(bf, 10, &w30Unit{reg: reg, levels: map[string]int{"Blade Fury": 10}}, reg).Eval(bf.PrgCalc[1]); got != 5 {
		t.Errorf("Blade Fury prgcalc1 = %d, want 5", got)
	}

	inf := reg.ByName("Inferno")

	for lvl, want := range map[int]int{1: 10, 10: 23, 20: 38} {
		e := d2skill.NewEnv(inf, lvl, &w30Unit{reg: reg, levels: map[string]int{"Inferno": lvl}}, reg)
		if got := e.Eval(inf.Calc[1]); got != want {
			t.Errorf("Inferno L%d calc1 = %d, want %d", lvl, got, want)
		}
	}
}
