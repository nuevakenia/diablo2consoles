package d2skill

import (
	"math"
	"testing"
)

// Wave 30 wiki cross check, Amazon. Numbers are the LoD 1.14 values documented by the Amazon Basin wiki
// (theamazonbasin.com, LoD columns) and d2emu.com/db/skill/*; the engine side is the real patch_d2 skills.txt
// (D2_TABLES) driven through Skill.Descriptor / ElemMin / ElemMax / ElemLen / ToHitBonus / Calc. Where the
// d2emu page is a D2R page (Guided Arrow damage, Freezing Arrow mana, Strafe target count, Magic Arrow percent) the
// table and the Amazon Basin LoD page agree with the engine and the d2emu number is only noted in the
// notes file. Synergies are off (no other skill has a base level) unless a test says otherwise.

type wikiAmaRow struct {
	skill string
	lvl   int
	mana  float64 // true mana cost
	emin  float64 // elemental or per-frame poison minimum (8.8 shown as a float)
	emax  float64
	elen  int // frames
	th    int // attack rating bonus percent
}

func amaNear(a, b float64) bool { return math.Abs(a-b) < 0.006 }

func TestWikiAmazonDescriptors(t *testing.T) {
	reg := loadRealRegistry(t)

	rows := []wikiAmaRow{
		// Fire Arrow: 1-4 / 21-24 / 63-70 fire, 3 -> 5.375 mana, AR 10 / 91 / 181
		{"Fire Arrow", 1, 3, 1, 4, 0, 10}, {"Fire Arrow", 10, 4.125, 21, 24, 0, 91}, {"Fire Arrow", 20, 5.375, 63, 70, 0, 181},
		// Cold Arrow: length 4 s / 14.8 s / 26.8 s (70 + 30 lvl frames); L23 69-73 (d2emu)
		{"Cold Arrow", 1, 3.5, 3, 4, 100, 10}, {"Cold Arrow", 10, 4.625, 22, 23, 370, 91}, {"Cold Arrow", 20, 5.875, 53, 56, 670, 181},
		{"Cold Arrow", 23, 6.25, 69, 73.5, 760, 208},
		// Ice Arrow: freeze 45 + 5 lvl frames (2 s / 3.8 s / 5.8 s)
		{"Ice Arrow", 1, 4, 6, 10, 50, 20}, {"Ice Arrow", 10, 6.25, 72, 78, 95, 101}, {"Ice Arrow", 20, 8.75, 216, 232, 145, 191},
		{"Ice Arrow", 23, 9.5, 278, 297, 160, 218},
		// Freezing Arrow: mana 8.5 + 0.5 lvl (the d2emu mana column, 11.2 / 13.8, is not LoD)
		{"Freezing Arrow", 1, 9, 40, 50, 50, 40}, {"Freezing Arrow", 10, 13.5, 140, 150, 50, 121}, {"Freezing Arrow", 20, 18.5, 310, 320, 50, 211},
		{"Freezing Arrow", 23, 20, 372, 382, 50, 238},
		{"Exploding Arrow", 1, 5, 2, 6, 0, 20}, {"Exploding Arrow", 10, 9.5, 51, 57, 0, 101}, {"Exploding Arrow", 20, 14.5, 129, 149, 0, 191},
		// Immolation Arrow explosion (the pyre is a missile)
		{"Immolation Arrow", 1, 6, 12, 23, 0, 30}, {"Immolation Arrow", 10, 10.5, 142, 153, 0, 111}, {"Immolation Arrow", 20, 15.5, 416, 427, 0, 201},
		{"Immolation Arrow", 23, 17, 520, 531, 0, 228},
		// Poison Javelin: damage per frame, length 150 + 50 lvl frames (6 + 2 lvl seconds)
		{"Poison Javelin", 1, 4, 32.0 / 256, 48.0 / 256, 200, 0}, {"Poison Javelin", 10, 6.25, 208.0 / 256, 232.0 / 256, 650, 0},
		{"Poison Javelin", 20, 8.75, 592.0 / 256, 656.0 / 256, 1150, 0},
		// Plague Javelin: length 65 + 10 lvl frames
		{"Plague Javelin", 1, 7, 80.0 / 256, 128.0 / 256, 75, 30}, {"Plague Javelin", 10, 11.5, 608.0 / 256, 656.0 / 256, 165, 111},
		{"Plague Javelin", 20, 16.5, 1824.0 / 256, 1872.0 / 256, 265, 201},
		// lightning skills (minimum is always 1)
		{"Power Strike", 1, 2, 1, 16, 0, 20}, {"Power Strike", 10, 4.25, 1, 214, 0, 128}, {"Power Strike", 20, 6.75, 1, 646, 0, 248},
		{"Charged Strike", 1, 4, 1, 30, 0, 0}, {"Charged Strike", 10, 6.25, 1, 146, 0, 0}, {"Charged Strike", 20, 8.75, 1, 322, 0, 0},
		{"Lightning Strike", 1, 9, 1, 25, 0, 0}, {"Lightning Strike", 10, 9, 1, 125, 0, 0}, {"Lightning Strike", 20, 9, 1, 295, 0, 0},
		{"Lightning Fury", 1, 10, 1, 40, 0, 0}, {"Lightning Fury", 10, 14.5, 1, 240, 0, 0}, {"Lightning Fury", 20, 19.5, 1, 580, 0, 0},
		{"Lightning Bolt", 1, 6, 1, 40, 0, 0}, {"Lightning Bolt", 10, 8.25, 1, 160, 0, 0}, {"Lightning Bolt", 20, 10.75, 1, 380, 0, 0},
		// plain melee / arrow skills: only mana and attack rating
		{"Jab", 1, 2, 0, 0, 0, 10}, {"Jab", 10, 4.25, 0, 0, 0, 91}, {"Jab", 20, 6.75, 0, 0, 0, 181},
		{"Impale", 1, 3, 0, 0, 0, 100}, {"Impale", 10, 3, 0, 0, 0, 325}, {"Impale", 20, 3, 0, 0, 0, 575},
		{"Fend", 1, 5, 0, 0, 0, 40}, {"Fend", 10, 5, 0, 0, 0, 130}, {"Fend", 20, 5, 0, 0, 0, 230},
		{"Multiple Shot", 1, 4, 0, 0, 0, 0}, {"Multiple Shot", 10, 13, 0, 0, 0, 0}, {"Multiple Shot", 20, 23, 0, 0, 0, 0},
		{"Multiple Shot", 23, 26, 0, 0, 0, 0},
		{"Guided Arrow", 1, 8, 0, 0, 0, 0}, {"Guided Arrow", 10, 5.75, 0, 0, 0, 0}, {"Guided Arrow", 20, 3.25, 0, 0, 0, 0},
		{"Strafe", 1, 11, 0, 0, 0, 0}, {"Strafe", 20, 11, 0, 0, 0, 0},
		// Magic Arrow mana floors at 0 from level 14
		{"Magic Arrow", 1, 1.5, 0, 0, 0, 10}, {"Magic Arrow", 10, 0.375, 0, 0, 0, 91}, {"Magic Arrow", 20, 0, 0, 0, 0, 181},
	}

	for _, r := range rows {
		sk, e := wikiEnv(reg, r.skill, r.lvl, map[string]int{r.skill: r.lvl})
		if sk == nil {
			t.Fatalf("%s missing", r.skill)
		}

		if got := float64(sk.ManaCost(r.lvl)) / 256; !amaNear(got, r.mana) {
			t.Errorf("%s L%d mana %.3f, wiki %.3f", r.skill, r.lvl, got, r.mana)
		}

		if got := float64(sk.ElemMin(e, r.lvl)) / 256; !amaNear(got, r.emin) && sk.EType != "" {
			t.Errorf("%s L%d min %.3f, wiki %.3f", r.skill, r.lvl, got, r.emin)
		}

		if got := float64(sk.ElemMax(e, r.lvl)) / 256; !amaNear(got, r.emax) && sk.EType != "" {
			t.Errorf("%s L%d max %.3f, wiki %.3f", r.skill, r.lvl, got, r.emax)
		}

		if got := sk.ElemLen(e, r.lvl); got != r.elen {
			t.Errorf("%s L%d length %d frames, wiki %d", r.skill, r.lvl, got, r.elen)
		}

		if got := sk.ToHitBonus(e, r.lvl); got != r.th {
			t.Errorf("%s L%d attack rating %d, wiki %d", r.skill, r.lvl, got, r.th)
		}
	}
}

// Calc columns that carry projectile counts, hit counts and melee damage percents.
func TestWikiAmazonCalcs(t *testing.T) {
	reg := loadRealRegistry(t)

	rows := []struct {
		skill string
		lvl   int
		calc  [4]int // calc1..calc4
	}{
		// Multiple Shot arrows 1 + lvl, capped at 24 (L23 = 24 arrows, L30 = 24)
		{"Multiple Shot", 1, [4]int{2, 1, 2, 0}}, {"Multiple Shot", 10, [4]int{11, 1, 2, 0}}, {"Multiple Shot", 20, [4]int{21, 1, 2, 0}},
		{"Multiple Shot", 23, [4]int{24, 1, 2, 0}}, {"Multiple Shot", 30, [4]int{24, 1, 2, 0}},
		// Strafe: max targets min(lvl + 3, 10), damage percent 5 lvl, minimum targets 2 + lvl / 4
		{"Strafe", 1, [4]int{4, 5, 2, 0}}, {"Strafe", 10, [4]int{10, 50, 4, 0}}, {"Strafe", 20, [4]int{10, 100, 7, 0}},
		// Charged Strike bolts 3 + lvl / 5; Lightning Strike chain 1 + lvl hits; Lightning Fury 1 + lvl bolts
		{"Charged Strike", 1, [4]int{3}}, {"Charged Strike", 10, [4]int{5}}, {"Charged Strike", 20, [4]int{7}}, {"Charged Strike", 30, [4]int{9}},
		{"Lightning Strike", 1, [4]int{20, 2}}, {"Lightning Strike", 10, [4]int{20, 11}}, {"Lightning Strike", 30, [4]int{20, 31}},
		{"Lightning Fury", 1, [4]int{2}}, {"Lightning Fury", 10, [4]int{11}}, {"Lightning Fury", 30, [4]int{31}},
		// Guided Arrow damage bonus 5 lvl - 5 (LoD; the d2emu value 7 lvl - 7 is D2R)
		{"Guided Arrow", 1, [4]int{0}}, {"Guided Arrow", 10, [4]int{45}}, {"Guided Arrow", 20, [4]int{95}}, {"Guided Arrow", 30, [4]int{145}},
		// Jab damage -15 + 3 lvl; Impale damage 275 + 25 lvl and durability chance 46 / 30 / 25; Fend damage 60 + 10 lvl
		{"Jab", 1, [4]int{-15}}, {"Jab", 10, [4]int{12}}, {"Jab", 20, [4]int{42}}, {"Jab", 30, [4]int{72}},
		{"Impale", 1, [4]int{300, 46, 1}}, {"Impale", 10, [4]int{525, 30, 1}}, {"Impale", 20, [4]int{775, 25, 1}},
		{"Fend", 1, [4]int{12, 70}}, {"Fend", 10, [4]int{12, 160}}, {"Fend", 20, [4]int{12, 260}},
		// Dopplezon: calc1 life bonus 10 lvl percent, calc2 duration frames 250 + 125 (lvl - 1), calc3 50 percent of the
		// owner's life (wiki: 5 + 5 lvl seconds, 50 + 5 lvl percent of the life)
		{"Dopplezon", 1, [4]int{10, 250, 50}}, {"Dopplezon", 10, [4]int{100, 1375, 50}}, {"Dopplezon", 20, [4]int{200, 2625, 50}},
	}

	for _, r := range rows {
		sk, e := wikiEnv(reg, r.skill, r.lvl, map[string]int{r.skill: r.lvl})

		for i := 0; i < 4; i++ {
			if got := e.eval(sk.Calc[i+1]); got != r.calc[i] {
				t.Errorf("%s L%d calc%d = %d, wiki %d", r.skill, r.lvl, i+1, got, r.calc[i])
			}
		}
	}
}

// Passive percents, auras and curse-like durations.
func TestWikiAmazonPassivesAndAuras(t *testing.T) {
	reg := loadRealRegistry(t)

	pass := []struct {
		skill string
		stat  string
		vals  map[int]int
	}{
		{"Critical Strike", "passive_critical_strike", map[int]int{1: 16, 10: 56, 20: 68, 30: 73}}, // wiki table 74, its formula 73.75: truncation
		{"Dodge", "passive_dodge", map[int]int{1: 18, 10: 47, 20: 56, 30: 60}},
		{"Evade", "passive_evade", map[int]int{1: 18, 10: 47, 20: 56, 30: 60}},
		// the wiki table lists 24 / 55 / 65 / 69 for Avoid (its own formula gives 24.4 / 56.3 / 65.8 / 70.0): the
		// engine value is the table's (dm12 integer arithmetic), so the formula line of the wiki is the rounded one
		{"Avoid", "passive_avoid", map[int]int{1: 24, 10: 55, 20: 65, 30: 69}},
		{"Penetrate", "item_tohit_percent", map[int]int{1: 35, 10: 125, 20: 225, 30: 325}},
		{"Pierce", "skill_pierce", map[int]int{1: 23, 10: 71, 20: 85, 30: 91}},
	}

	for _, p := range pass {
		for lvl, want := range p.vals {
			sk, e := wikiEnv(reg, p.skill, lvl, map[string]int{p.skill: lvl})

			found := false

			for i := 1; i <= 5; i++ {
				if sk.PassiveStat[i] == p.stat {
					found = true

					if got := e.eval(sk.PassiveCalc[i]); got != want {
						t.Errorf("%s L%d %s = %d, wiki %d", p.skill, lvl, p.stat, got, want)
					}
				}
			}

			if !found {
				t.Errorf("%s has no %s", p.skill, p.stat)
			}
		}
	}

	// Inner Sight: defense -(25 lvl + 15 ...) = -40 / -305 / -815 / -1615, duration 4 + 4 lvl seconds, radius 20 subtiles
	for lvl, want := range map[int][3]int{1: {40, 200, 20}, 10: {305, 1100, 20}, 20: {815, 2100, 20}, 30: {1615, 3100, 20}} {
		sk, e := wikiEnv(reg, "Inner Sight", lvl, map[string]int{"Inner Sight": lvl})
		got := [3]int{-e.eval(sk.AuraStatCalc[1]), e.eval(sk.AuraLenCalc), e.eval(sk.AuraRangeCalc)}

		if got != want {
			t.Errorf("Inner Sight L%d defense/len/range %v, wiki %v", lvl, got, want)
		}
	}

	// Slow Missiles: 33 percent speed, duration 6 + 6 lvl seconds
	for lvl, frames := range map[int]int{1: 300, 10: 1650, 20: 3150, 30: 4650} {
		sk, e := wikiEnv(reg, "Slow Missiles", lvl, map[string]int{"Slow Missiles": lvl})

		if got := e.eval(sk.AuraLenCalc); got != frames {
			t.Errorf("Slow Missiles L%d duration %d frames, wiki %d", lvl, got, frames)
		}

		if got := e.eval(sk.AuraStatCalc[1]); got != 33 {
			t.Errorf("Slow Missiles L%d speed %d", lvl, got)
		}
	}
}

// Valkyrie: life bonus calc1 (percent of the template life), armor percent, attack rating, strength and dexterity
// from the aura stats, monster level calc2 and mana.
func TestWikiAmazonValkyrie(t *testing.T) {
	reg := loadRealRegistry(t)

	for _, tc := range []struct {
		lvl                       int
		mana                      float64
		hpPct, armor, th, str, dx int
		res, mlvl                 int
	}{
		{1, 25, 0, 0, 40, 25, 12, 2, 25}, {10, 34, 180, 90, 400, 250, 120, 20, 52}, {20, 44, 380, 190, 800, 500, 240, 40, 82},
	} {
		sk, e := wikiEnv(reg, "Valkyrie", tc.lvl, map[string]int{"Valkyrie": tc.lvl})

		if got := float64(sk.ManaCost(tc.lvl)) / 256; !amaNear(got, tc.mana) {
			t.Errorf("L%d mana %.2f, wiki %.2f", tc.lvl, got, tc.mana)
		}

		// wiki: life 400 + 80 (lvl - 1) = template life * (100 + calc1) / 100 for a template of 400
		if got := e.eval(sk.Calc[1]); got != tc.hpPct {
			t.Errorf("L%d calc1 %d, want %d", tc.lvl, got, tc.hpPct)
		}

		if got := 400 * (100 + e.eval(sk.Calc[1])) / 100; got != 400+80*(tc.lvl-1) {
			t.Errorf("L%d life %d, wiki %d", tc.lvl, got, 400+80*(tc.lvl-1))
		}

		if got := e.eval(sk.PassiveCalc[1]); got != tc.armor {
			t.Errorf("L%d armor percent %d, wiki %d", tc.lvl, got, tc.armor)
		}

		if got := e.eval(sk.PassiveCalc[2]); got != tc.th {
			t.Errorf("L%d attack rating %d, wiki 40 * lvl = %d", tc.lvl, got, tc.th)
		}

		if got := e.eval(sk.AuraStatCalc[1]); got != tc.str {
			t.Errorf("L%d strength %d, wiki damage percent %d", tc.lvl, got, tc.str)
		}

		if got := e.eval(sk.AuraStatCalc[2]); got != tc.dx {
			t.Errorf("L%d dexterity %d, wiki %d", tc.lvl, got, tc.dx)
		}

		if got := e.eval(sk.AuraStatCalc[3]); got != tc.res {
			t.Errorf("L%d resistance %d, wiki %d", tc.lvl, got, tc.res)
		}

		if got := e.eval(sk.Calc[2]); got != tc.mlvl {
			t.Errorf("L%d monster level %d, wiki %d", tc.lvl, got, tc.mlvl)
		}
	}

	// Decoy synergy: Dopplezon base levels add 20 percent life per level and 2 percent resist
	sk, e := wikiEnv(reg, "Valkyrie", 10, map[string]int{"Valkyrie": 10, "Dopplezon": 5})
	if got := e.eval(sk.Calc[1]); got != 180+100 {
		t.Errorf("Valkyrie life with Decoy 5: %d", got)
	}

	if got := e.eval(sk.AuraStatCalc[3]); got != 30 {
		t.Errorf("Valkyrie resist with Decoy 5: %d", got)
	}
}

// Synergy percents of the damage skills (base levels of the partner skills, never the skill's own level).
func TestWikiAmazonSynergies(t *testing.T) {
	reg := loadRealRegistry(t)

	lightning := []string{"Power Strike", "Charged Strike", "Lightning Strike", "Lightning Fury", "Lightning Bolt"}

	cases := []struct {
		skill   string
		lvl     int
		partner map[string]int
		pct     int // total percent added
	}{
		{"Fire Arrow", 10, map[string]int{"Exploding Arrow": 5}, 60},
		{"Exploding Arrow", 10, map[string]int{"Fire Arrow": 5}, 60},
		{"Immolation Arrow", 10, map[string]int{"Exploding Arrow": 5}, 50},
		{"Cold Arrow", 10, map[string]int{"Ice Arrow": 5}, 60},
		{"Ice Arrow", 10, map[string]int{"Cold Arrow": 5}, 40},
		{"Freezing Arrow", 10, map[string]int{"Cold Arrow": 5}, 60},
		{"Poison Javelin", 10, map[string]int{"Plague Javelin": 5}, 60},
		{"Plague Javelin", 10, map[string]int{"Poison Javelin": 5}, 50},
	}

	for _, w := range []struct {
		skill string
		per   int
	}{{"Power Strike", 10}, {"Charged Strike", 10}, {"Lightning Strike", 8}, {"Lightning Fury", 1}, {"Lightning Bolt", 3}} {
		partners := map[string]int{}
		sum := 0

		for _, n := range lightning {
			if n != w.skill {
				partners[n] = 4
				sum += 4
			}
		}

		cases = append(cases, struct {
			skill   string
			lvl     int
			partner map[string]int
			pct     int
		}{w.skill, 10, partners, sum * w.per})
	}

	for _, c := range cases {
		_, e0 := wikiEnv(reg, c.skill, c.lvl, map[string]int{c.skill: c.lvl})
		base := map[string]int{c.skill: c.lvl}

		for k, v := range c.partner {
			base[k] = v
		}

		sk, e1 := wikiEnv(reg, c.skill, c.lvl, base)
		max0, max1 := float64(sk.ElemMax(e0, c.lvl)), float64(sk.ElemMax(e1, c.lvl))

		if want := max0 * float64(100+c.pct) / 100; math.Abs(max1-want) > 2 {
			t.Errorf("%s L%d max with synergy %.2f, want %.2f (+%d%%)", c.skill, c.lvl, max1/256, want/256, c.pct)
		}

		// the wiki says the lightning synergy never lifts the minimum of 1
		if sk.EType == "ltng" && sk.ElemMin(e1, c.lvl) != sk.ElemMin(e0, c.lvl) {
			t.Errorf("%s L%d lightning minimum moved: %d", c.skill, c.lvl, sk.ElemMin(e1, c.lvl))
		}
	}

	// Ice Arrow and Freezing Arrow: +5 percent freeze length per level of the partner
	for skill, partner := range map[string]string{"Ice Arrow": "Freezing Arrow", "Freezing Arrow": "Ice Arrow"} {
		sk, e0 := wikiEnv(reg, skill, 10, map[string]int{skill: 10})
		_, e1 := wikiEnv(reg, skill, 10, map[string]int{skill: 10, partner: 10})
		l0, l1 := sk.ElemLen(e0, 10), sk.ElemLen(e1, 10)

		if want := l0 + l0*50/100; l1 != want {
			t.Errorf("%s freeze length with %s 10: %d, want %d", skill, partner, l1, want)
		}
	}
}
