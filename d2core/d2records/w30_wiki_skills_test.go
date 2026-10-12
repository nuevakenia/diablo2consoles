package d2records

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// w30 wiki cross-check, sorceress / druid / assassin / Teleport. Each expectation below was compared with a community
// page (the verdicts are in ~/git/d2-re-notes/w30-wiki-skills-b.md). Values are whole points except where a .5 is
// kept; frames are 25 per second. The tables of the owner's 1.14b install are the ground truth: several wiki pages
// carry Resurrected 2.4 numbers (Blizzard +15 percent, Meteor +10 percent, Hydra, Raven, wolves), they differ from
// these and are NOT engine errors.

// The cast delay (skills.txt delay) is what Do puts on the cooldown. Wiki: Blizzard 1.8 s, Meteor 1.2 s, Frozen Orb
// 1 s, Hydra 1.6 s, Shadow Warrior and Shadow Master 6 s (2.4 reduced it to 0.6 s), Armageddon and Hurricane 6 s,
// Volcano 4 s, Fissure 2 s, Firestorm 0.6 s, Molten Boulder 2 s, Fire Wall 1.4 s, Teleport none.
func TestW30CastDelays(t *testing.T) {
	for _, tc := range []struct {
		skill  string
		frames int
	}{
		{"Blizzard", 45}, {"Meteor", 30}, {"Frozen Orb", 25}, {"Hydra", 40}, {"Teleport", 0}, {"Fire Wall", 35},
		{"Shadow Warrior", 150}, {"Shadow Master", 150}, {"Armageddon", 150}, {"Hurricane", 150}, {"Volcano", 100},
		{"Eruption", 50}, {"Firestorm", 15}, {"Molten Boulder", 50}, {"Blade Sentinel", 50},
	} {
		r := realCast(t, tc.skill, 10)
		if r.res.Cooldown != tc.frames {
			t.Errorf("%s: cooldown %d frames, want %d", tc.skill, r.res.Cooldown, tc.frames)
		}
	}
}

// Mana actually paid by Do (8.8): Teleport 24, 15, 5 at levels 1, 10, 20 (wiki: exact), Blizzard 23, 32, 42, Frozen Orb
// 25, 29.5, 34.5 (wiki pages round down to 25, 29, 34), Meteor 17, 21.5, 26.5, Hydra 20, 24.5, 29.5.
func TestW30ManaPaid(t *testing.T) {
	for _, tc := range []struct {
		skill string
		lvl   int
		mana  int
	}{
		{"Teleport", 1, 24 * 256}, {"Teleport", 10, 15 * 256}, {"Teleport", 20, 5 * 256},
		{"Blizzard", 1, 23 * 256}, {"Blizzard", 10, 32 * 256}, {"Blizzard", 20, 42 * 256},
		{"Frozen Orb", 1, 25 * 256}, {"Frozen Orb", 10, 29*256 + 128}, {"Frozen Orb", 20, 34*256 + 128},
		{"Meteor", 1, 17 * 256}, {"Meteor", 10, 21*256 + 128}, {"Meteor", 20, 26*256 + 128},
		{"Hydra", 1, 20 * 256}, {"Hydra", 10, 24*256 + 128}, {"Hydra", 20, 29*256 + 128},
	} {
		r := realCast(t, tc.skill, tc.lvl)
		// the unit starts with 5000 mana; Start or Do takes the cost depending on the skill (Hydra pays at the start)
		if paid := 5000*256 - r.u.Mana(); paid != tc.mana {
			t.Errorf("%s L%d: paid %d (8.8), want %d", tc.skill, tc.lvl, paid, tc.mana)
		}
	}
}

// Elemental ranges as the exe computes them (8.8 values divided by 256). Wiki agreement noted per row.
func TestW30WikiMatches(t *testing.T) {
	reg := loadRealRecords(t).SkillTable()

	unit := func(levels map[string]int, stats map[string]int) *w30Unit {
		return &w30Unit{reg: reg, levels: levels, stats: stats}
	}

	for _, tc := range []struct {
		skill      string
		lvl        int
		syn        map[string]int
		min, max   float64
		elen       int // frames
		wikiAgrees string
	}{
		// Fire Bolt: wiki 3-6, 17-22, 45-60 (truncated), same on the old and the new pages
		{"Fire Bolt", 1, nil, 3, 6, 0, "yes"},
		{"Fire Bolt", 10, nil, 17.5, 22.5, 0, "yes"},
		{"Fire Bolt", 20, nil, 45.5, 60.5, 0, "yes"},
		// Meteor: d2emu and the table 80-100, 319-357, 869-927; Resurrected pages 88-110, 350-392, 955-1019
		{"Meteor", 1, nil, 80, 100, 0, "d2emu yes, 2.4 pages +10%"},
		{"Meteor", 20, nil, 869, 927, 0, "d2emu yes, 2.4 pages +10%"},
		// Meteor synergies +5 percent per Fire Bolt and Fire Ball level (wiki: same)
		{"Meteor", 1, map[string]int{"Fire Bolt": 10, "Fire Ball": 10}, 160, 200, 0, "yes"},
		// Blizzard: 45-75, 210-249, 570-619 (d2emu); 51-86, 241-286, 655-711 on the 2.4 pages (+15 percent)
		{"Blizzard", 1, nil, 45, 75, 100, "d2emu yes, 2.4 pages +15%"},
		{"Blizzard", 10, nil, 210, 249, 100, "d2emu yes"},
		{"Blizzard", 20, nil, 570, 619, 100, "d2emu yes"},
		// Blizzard synergy +5 percent per Ice Bolt, Ice Blast, Glacial Spike level
		{"Blizzard", 1, map[string]int{"Ice Bolt": 4, "Ice Blast": 4, "Glacial Spike": 2}, 67.5, 112.5, 100, "yes"},
		// Frozen Orb: cold 40-45 / 134-143 / 262-276, length 8 / 17 / 27 s (the Arreat table prints max = min + 5)
		{"Frozen Orb", 1, nil, 40, 45, 200, "yes"},
		{"Frozen Orb", 10, nil, 134, 143.5, 425, "min yes, max: Arreat prints 139 (doubtful)"},
		{"Frozen Orb", 20, nil, 262, 276.5, 675, "min yes, max: Arreat prints 267 (doubtful)"},
		// Hydra: 14-19.5, 67.5-82, 150.5-175 (d2emu: 14-19, 67-82, 150-175; 2.4 pages 159-185)
		{"Hydra", 1, nil, 14, 19.5, 0, "d2emu yes"},
		{"Hydra", 20, nil, 150.5, 175, 0, "d2emu yes, 2.4 pages +6%"},
		// Ice Bolt, Glacial Spike: table only (wiki pages not compared)
		{"Ice Bolt", 20, nil, 38, 49.5, 815, "table"},
		{"Glacial Spike", 20, nil, 225, 242.5, 0, "table"},
		// Druid: Fissure (Eruption) 15-25, 81-91, 217-227; Volcano 8-10 (fire) per rock; Armageddon 25-75 fire
		{"Eruption", 1, nil, 15, 25, 0, "table"},
		{"Eruption", 20, nil, 217, 227, 0, "table"},
		{"Armageddon", 20, nil, 390, 471, 0, "table"},
		// Assassin: Death Sentry lightning 1-50 / 1-134 / 1-306 (d2emu: same), Lightning Sentry 1-20 / 1-122 / 1-314
		{"Death Sentry", 1, nil, 1, 50, 0, "d2emu yes"},
		{"Death Sentry", 10, nil, 1, 134, 0, "d2emu yes"},
		{"Death Sentry", 20, nil, 1, 306, 0, "d2emu yes"},
		{"Lightning Sentry", 20, nil, 1, 314, 0, "table"},
	} {
		sk := reg.ByName(tc.skill)
		u := unit(map[string]int{tc.skill: tc.lvl}, nil)

		for k, v := range tc.syn {
			u.levels[k] = v
		}

		e := d2skill.NewEnv(sk, tc.lvl, u, reg)

		if got := float64(sk.ElemMin(e, tc.lvl)) / 256; got != tc.min {
			t.Errorf("%s L%d %v: min %v, want %v (wiki %s)", tc.skill, tc.lvl, tc.syn, got, tc.min, tc.wikiAgrees)
		}

		if got := float64(sk.ElemMax(e, tc.lvl)) / 256; got != tc.max {
			t.Errorf("%s L%d %v: max %v, want %v (wiki %s)", tc.skill, tc.lvl, tc.syn, got, tc.max, tc.wikiAgrees)
		}

		if got := sk.ElemLen(e, tc.lvl); got != tc.elen {
			t.Errorf("%s L%d: length %d frames, want %d", tc.skill, tc.lvl, got, tc.elen)
		}
	}
}

// The three passive masteries (wiki: Fire Mastery 30 + 7 per level, Lightning Mastery 50 + 12 per level, Cold
// Mastery lowers the enemy cold resistance by 20 + 5 per level) and how a mastery percent enters spell damage:
// min and max both grow by the percent (SKILL_ElementalMasteryBonus 0x646040).
func TestW30Masteries(t *testing.T) {
	reg := loadRealRecords(t).SkillTable()

	for _, tc := range []struct {
		skill, stat string
		lvl, want   int
	}{
		{"Fire Mastery", "passive_fire_mastery", 1, 30}, {"Fire Mastery", "passive_fire_mastery", 10, 93},
		{"Fire Mastery", "passive_fire_mastery", 20, 163},
		{"Lightning Mastery", "passive_ltng_mastery", 1, 50}, {"Lightning Mastery", "passive_ltng_mastery", 10, 158},
		{"Lightning Mastery", "passive_ltng_mastery", 20, 278},
		{"Cold Mastery", "passive_cold_pierce", 1, 20}, {"Cold Mastery", "passive_cold_pierce", 10, 65},
		{"Cold Mastery", "passive_cold_pierce", 20, 115},
		{"Warmth", "manarecoverybonus", 1, 30}, {"Warmth", "manarecoverybonus", 10, 138}, {"Warmth", "manarecoverybonus", 20, 258},
	} {
		sk := reg.ByName(tc.skill)
		u := &w30Unit{reg: reg, levels: map[string]int{tc.skill: tc.lvl}}
		e := d2skill.NewEnv(sk, tc.lvl, u, reg)
		found := false

		for i := 1; i <= 5; i++ {
			if sk.PassiveStat[i] == tc.stat {
				found = true

				if got := e.Eval(sk.PassiveCalc[i]); got != tc.want {
					t.Errorf("%s L%d %s = %d, want %d", tc.skill, tc.lvl, tc.stat, got, tc.want)
				}
			}
		}

		if !found {
			t.Errorf("%s has no %s", tc.skill, tc.stat)
		}
	}

	// Fire Bolt L20 with Fire Mastery 20 (163 percent): 45.5-60.5 becomes x2.63 (integer 8.8 arithmetic)
	fb := reg.ByName("Fire Bolt")
	e := d2skill.NewEnv(fb, 20, &w30Unit{reg: reg, levels: map[string]int{"Fire Bolt": 20}}, reg)
	d := fb.Descriptor(e, 20, 0, 0, 163)

	if want := int32(11648 + 11648*163/100); d.Fire.Min != want {
		t.Errorf("mastery min %d, want %d", d.Fire.Min, want)
	}

	if want := int32(15488 + 15488*163/100); d.Fire.Max != want {
		t.Errorf("mastery max %d, want %d", d.Fire.Max, want)
	}

	// Lightning Mastery reaches lightning skills only through the lightning damage type: Lightning L20 with 278
	// percent: max 272 x 3.78
	lt := reg.ByName("Lightning")
	e = d2skill.NewEnv(lt, 20, &w30Unit{reg: reg, levels: map[string]int{"Lightning": 20}}, reg)

	if got := lt.Descriptor(e, 20, 0, 0, 278).Lightning.Max; got != int32(272*256+272*256*278/100) {
		t.Errorf("lightning mastery max %d", got)
	}
}

// w30Unit is a minimal caster for Env evaluations: skill levels by name, stats by name, character level 30.
type w30Unit struct {
	auditUnit
	reg    *d2skill.Registry
	levels map[string]int
	stats  map[string]int
}

func (u *w30Unit) SkillLevel(id int) int     { return u.levels[u.reg.ByID(id).Name] }
func (u *w30Unit) BaseSkillLevel(id int) int { return u.levels[u.reg.ByID(id).Name] }
func (u *w30Unit) Stat(n string) int         { return u.stats[n] }
