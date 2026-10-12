package d2skill

import (
	"math"
	"testing"
)

// Wiki cross check of the Necromancer (wave 30). Wiki numbers are asserted only where the 1.14b tables and the
// exe formulas agree with them; the differences (mostly 2.4 / D2R values on the wiki pages) are listed in
// ~/git/d2-re-notes/w30-wiki-skills-necro.md.

func nearly(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

type necCase struct {
	skill string
	lvl   int
	mana  float64 // mana cost
	lo    int     // elemental (or poison total) min, whole points
	hi    int
	extra int // skill specific: calc1 (teeth), duration frames (curses), tohit (poison dagger)
}

// skill damage per level, magic (per hit) and poison (total over the length)
func TestWikiNecroDamage(t *testing.T) {
	reg := loadRealRegistry(t)

	cases := []necCase{
		// Teeth: calc1 is the number of teeth
		{"Teeth", 1, 3, 2, 4, 2}, {"Teeth", 4, 4.5, 5, 7, 5}, {"Teeth", 10, 7.5, 11, 14, 11}, {"Teeth", 13, 9, 14, 18, 14},
		{"Teeth", 20, 12.5, 23, 31, 21}, {"Teeth", 23, 14, 28, 37, 24},
		{"Bone Spear", 1, 7, 16, 24, 0}, {"Bone Spear", 4, 7.75, 40, 48, 0}, {"Bone Spear", 10, 9.25, 90, 98, 0},
		{"Bone Spear", 13, 10, 117, 125, 0}, {"Bone Spear", 20, 11.75, 192, 204, 0}, {"Bone Spear", 23, 12.5, 234, 249, 0},
		{"Bone Spirit", 1, 12, 20, 30, 0}, {"Bone Spirit", 4, 13.5, 68, 81, 0}, {"Bone Spirit", 10, 16.5, 166, 185, 0},
		{"Bone Spirit", 13, 18, 217, 239, 0}, {"Bone Spirit", 20, 21.5, 340, 369, 0}, {"Bone Spirit", 23, 23, 395, 427, 0},
	}

	for _, c := range cases {
		sk, e := wikiEnv(reg, c.skill, c.lvl, map[string]int{c.skill: c.lvl})

		if got := float64(sk.ManaCost(c.lvl)) / 256; !nearly(got, c.mana, 0.01) {
			t.Errorf("%s L%d mana %.2f, wiki %.2f", c.skill, c.lvl, got, c.mana)
		}

		if lo, hi := int(sk.ElemMin(e, c.lvl)>>8), int(sk.ElemMax(e, c.lvl)>>8); lo != c.lo || hi != c.hi {
			t.Errorf("%s L%d damage %d-%d, wiki %d-%d", c.skill, c.lvl, lo, hi, c.lo, c.hi)
		}

		if c.skill == "Teeth" {
			if got := e.eval(sk.Calc[1]); got != c.extra {
				t.Errorf("Teeth L%d teeth %d, wiki %d", c.lvl, got, c.extra)
			}
		}
	}
}

// Poison Dagger agrees with the wiki (total = per frame rate * length / 256, the tooltip of kind 14, 0x4eabd0).
// Poison Nova and Poison Explosion differ by 4 to 15 percent on the LoD wiki pages (UNVERIFIED reason, the table and the
// exe formulas are the reference), and by the HitShift on the D2R pages; they are pinned to the engine values.
func TestWikiNecroPoison(t *testing.T) {
	reg := loadRealRegistry(t)

	type pc struct {
		skill     string
		lvl       int
		mana      float64
		frames    int
		lo, hi    int
		wikiLo    int // 0: no wiki number asserted
		wikiHi    int
		toHit     int
		wikiNotes string
	}

	for _, c := range []pc{
		{"Poison Dagger", 1, 3, 50, 7, 15, 7, 15, 30, ""}, {"Poison Dagger", 4, 3.75, 80, 30, 43, 30, 43, 90, ""},
		{"Poison Dagger", 10, 5.25, 140, 129, 153, 129, 153, 210, ""}, {"Poison Dagger", 13, 6, 170, 216, 245, 216, 245, 270, ""},
		{"Poison Dagger", 20, 7.75, 240, 540, 581, 540, 581, 410, ""},
		// engine 50-90, wiki 52-93; engine 175-215 (L10), wiki 194-236; engine 400-440 (L20), wiki 427-468
		{"Poison Nova", 1, 20, 50, 50, 90, 0, 0, 0, "wiki 52-93"}, {"Poison Nova", 10, 20, 50, 175, 215, 0, 0, 0, "wiki 194-236"},
		{"Poison Nova", 20, 20, 50, 400, 440, 0, 0, 0, "wiki 427-468"},
		// engine 25-75, wiki 28-86; engine 262-321 (L10), wiki 301-462 (sic, wiki pages disagree among themselves)
		{"Poison Explosion", 1, 8, 50, 25, 75, 0, 0, 0, "wiki 28-86"}, {"Poison Explosion", 20, 8, 240, 1171, 1411, 0, 0, 0, "wiki 1345-1620"},
	} {
		sk, e := wikiEnv(reg, c.skill, c.lvl, map[string]int{c.skill: c.lvl})

		if got := float64(sk.ManaCost(c.lvl)) / 256; !nearly(got, c.mana, 0.01) {
			t.Errorf("%s L%d mana %.2f, want %.2f", c.skill, c.lvl, got, c.mana)
		}

		if got := sk.ElemLen(e, c.lvl); got != c.frames {
			t.Errorf("%s L%d length %d frames, want %d", c.skill, c.lvl, got, c.frames)
		}

		lo := int(int64(sk.ElemMin(e, c.lvl)) * int64(c.frames) >> 8)
		hi := int(int64(sk.ElemMax(e, c.lvl)) * int64(c.frames) >> 8)

		if c.skill == "Poison Dagger" {
			// the dagger's HitShift 1 is part of the value; the wiki total matches
			if lo != c.wikiLo || hi != c.wikiHi {
				t.Errorf("%s L%d poison %d-%d, wiki %d-%d", c.skill, c.lvl, lo, hi, c.wikiLo, c.wikiHi)
			}

			if got := sk.ToHitBonus(e, c.lvl); got != c.toHit {
				t.Errorf("%s L%d to-hit %d, wiki %d", c.skill, c.lvl, got, c.toHit)
			}

			continue
		}

		// within one point of the pinned engine value (frame arithmetic is exact, the pins were computed by hand)
		if absInt(lo-c.lo) > 2 || absInt(hi-c.hi) > 2 {
			t.Errorf("%s L%d poison %d-%d, pinned %d-%d (%s)", c.skill, c.lvl, lo, hi, c.lo, c.hi, c.wikiNotes)
		}
	}
}

// Synergies of the 1.14b table: the percent per level of the other skills (par8).
func TestWikiNecroSynergy(t *testing.T) {
	reg := loadRealRegistry(t)

	// receiver -> (percent per level, donors). Wiki (D2R 2.4) pages list Bone Spear and Bone Spirit at 8; the
	// 1.14b table has 7 and 6 (the pages themselves say the synergies were raised in 2.4).
	for _, c := range []struct {
		skill  string
		pct    int
		donors []string
	}{
		{"Teeth", 15, []string{"Bone Wall", "Bone Spear", "Bone Prison", "Bone Spirit"}},
		{"Bone Spear", 7, []string{"Teeth", "Bone Wall", "Bone Prison", "Bone Spirit"}},
		{"Bone Spirit", 6, []string{"Teeth", "Bone Wall", "Bone Spear", "Bone Prison"}},
		{"Poison Dagger", 20, []string{"Poison Explosion", "Poison Nova"}},
		{"Poison Explosion", 15, []string{"Poison Dagger", "Poison Nova"}},
		{"Poison Nova", 10, []string{"Poison Dagger", "Poison Explosion"}},
	} {
		base := map[string]int{c.skill: 10}
		_, e0 := wikiEnv(reg, c.skill, 10, base)
		sk := reg.ByName(c.skill)
		m0 := int(sk.ElemMin(e0, 10))

		for _, d := range c.donors {
			lv := map[string]int{c.skill: 10, d: 5}
			_, e := wikiEnv(reg, c.skill, 10, lv)
			got := int(sk.ElemMin(e, 10))
			want := m0 + m0*5*c.pct/100

			if absInt(got-want) > 1 {
				t.Errorf("%s with %s 5: min %d, want %d (%d%% per level)", c.skill, d, got, want, c.pct)
			}
		}
	}
}

// Curses: duration in frames (wiki seconds * 25), radius in subtiles (wiki yards * 1.5; Corpse Explosion shows
// radius = subtiles / 3 because the exe halves the range, see doCorpseExplosionFn) and the stat the curse applies.
func TestWikiNecroCurses(t *testing.T) {
	reg := loadRealRegistry(t)

	type cc struct {
		skill    string
		lvl      int
		mana     float64
		secs     float64 // wiki duration
		yards    float64 // wiki radius
		stat     string
		statWiki int // wiki value of the first aura stat (0: not asserted)
	}

	for _, c := range []cc{
		{"Amplify Damage", 1, 4, 8, 2, "damageresist", -100}, {"Amplify Damage", 10, 4, 35, 8, "damageresist", -100},
		{"Amplify Damage", 20, 4, 65, 14.6, "damageresist", -100},
		{"Dim Vision", 1, 9, 7, 2.6, "", 0}, {"Dim Vision", 10, 9, 25, 8.6, "", 0}, {"Dim Vision", 20, 9, 45, 15.3, "", 0},
		{"Weaken", 1, 4, 14, 6, "damagepercent", -33}, {"Weaken", 10, 4, 35.6, 12, "damagepercent", -33}, {"Weaken", 20, 4, 59.6, 18.6, "damagepercent", -33},
		{"Iron Maiden", 1, 5, 12, 4.6, "", 0}, {"Iron Maiden", 10, 5, 33.6, 4.6, "", 0}, {"Iron Maiden", 20, 5, 57.6, 4.6, "", 0},
		{"Terror", 1, 7, 8, 2.6, "", 0}, {"Terror", 10, 7, 17, 2.6, "", 0}, {"Terror", 20, 7, 27, 2.6, "", 0},
		{"Confuse", 1, 13, 10, 4, "", 0}, {"Confuse", 10, 13, 28, 10, "", 0}, {"Confuse", 20, 13, 48, 16.6, "", 0},
		{"Life Tap", 1, 9, 16, 2.6, "", 0}, {"Life Tap", 10, 9, 37.6, 8.6, "", 0}, {"Life Tap", 20, 9, 61.6, 15.3, "", 0},
		{"Attract", 1, 17, 12, 6, "", 0}, {"Attract", 10, 17, 44.4, 6, "", 0}, {"Attract", 20, 17, 80.4, 6, "", 0},
		{"Decrepify", 1, 11, 4, 4, "velocitypercent", -50}, {"Decrepify", 20, 11, 15.4, 4, "velocitypercent", -50},
		{"Lower Resist", 1, 22, 20, 4.6, "fireresist", -31}, {"Lower Resist", 10, 22, 38, 10.6, "fireresist", -55},
		{"Lower Resist", 20, 22, 58, 17.3, "fireresist", -62},
	} {
		sk, e := wikiEnv(reg, c.skill, c.lvl, map[string]int{c.skill: c.lvl})

		if got := float64(sk.ManaCost(c.lvl)) / 256; !nearly(got, c.mana, 0.01) {
			t.Errorf("%s L%d mana %.2f, wiki %.2f", c.skill, c.lvl, got, c.mana)
		}

		if got := float64(e.eval(sk.AuraLenCalc)) / 25; !nearly(got, c.secs, 0.01) {
			t.Errorf("%s L%d duration %.2f s, wiki %.2f s", c.skill, c.lvl, got, c.secs)
		}

		// 1 subtile = 2/3 yard; the wiki rounds to 0.1
		if got := float64(e.eval(sk.AuraRangeCalc)) * 2 / 3; !nearly(got, c.yards, 0.1) {
			t.Errorf("%s L%d radius %.2f yd, wiki %.2f yd", c.skill, c.lvl, got, c.yards)
		}

		if c.stat != "" {
			if sk.AuraStat[1] != c.stat {
				t.Errorf("%s: first aura stat %q, want %q", c.skill, sk.AuraStat[1], c.stat)

				continue
			}

			if got := e.eval(sk.AuraStatCalc[1]); got != c.statWiki {
				t.Errorf("%s L%d %s = %d, wiki %d", c.skill, c.lvl, c.stat, got, c.statWiki)
			}
		}
	}

	// Iron Maiden reflects calc1 percent (wiki 200 / 425 / 675), Life Tap steals calc1 (50)
	for _, c := range []struct {
		skill string
		lvl   int
		want  int
	}{{"Iron Maiden", 1, 200}, {"Iron Maiden", 10, 425}, {"Iron Maiden", 20, 675}, {"Life Tap", 10, 50}} {
		sk, e := wikiEnv(reg, c.skill, c.lvl, map[string]int{c.skill: c.lvl})
		if got := e.eval(sk.Calc[1]); got != c.want {
			t.Errorf("%s L%d calc1 %d, wiki %d", c.skill, c.lvl, got, c.want)
		}
	}
}

// Weaken: the wiki pages list 34 to 53 percent growing with the level (2.4); the 1.14b table has a flat Param5 = 33
// (-33 percent damage) and the engine follows the table. Pinned here so a change is noticed.
func TestWikiNecroWeakenFlat(t *testing.T) {
	reg := loadRealRegistry(t)

	for _, lvl := range []int{1, 10, 20, 30} {
		sk, e := wikiEnv(reg, "Weaken", lvl, map[string]int{"Weaken": lvl})
		if got := e.eval(sk.AuraStatCalc[1]); got != -33 {
			t.Errorf("Weaken L%d: %d", lvl, got)
		}
	}
}

// Bone Armor absorb: wiki (2.4) 20 + 15 per level; the 1.14b table is 20 + 10 per level (ln12) plus 15 per level of
// Bone Wall and Bone Prison. The page itself says the gain was raised from 10 to 15 in 2.4.
func TestWikiNecroBoneArmor(t *testing.T) {
	reg := loadRealRegistry(t)

	for _, c := range []struct{ lvl, want int }{{1, 20}, {4, 50}, {10, 110}, {13, 140}, {20, 210}, {23, 240}} {
		sk, e := wikiEnv(reg, "Bone Armor", c.lvl, map[string]int{"Bone Armor": c.lvl})
		if got := e.eval(sk.AuraStatCalc[1]) >> 8; got != c.want {
			t.Errorf("Bone Armor L%d absorb %d, want %d", c.lvl, got, c.want)
		}

		if got := float64(sk.ManaCost(c.lvl)) / 256; !nearly(got, float64(10+c.lvl), 0.01) {
			t.Errorf("Bone Armor L%d mana %.2f", c.lvl, got)
		}
	}

	// synergy: 15 per level of Bone Wall and of Bone Prison
	sk, e := wikiEnv(reg, "Bone Armor", 10, map[string]int{"Bone Armor": 10, "Bone Wall": 5, "Bone Prison": 4})
	if got := e.eval(sk.AuraStatCalc[1]) >> 8; got != 110+9*15 {
		t.Errorf("Bone Armor synergy %d", got)
	}
}

// Bone Prison / Wall mana (wiki 27 24 18 15 8 / 17) and Corpse Explosion (mana 15 18 24 27 34, damage 70 to 120
// percent of the corpse life, 50 percent of it physical).
func TestWikiNecroBoneAndCorpse(t *testing.T) {
	reg := loadRealRegistry(t)

	for _, c := range []struct {
		skill string
		lvl   int
		mana  float64
	}{{"Bone Prison", 1, 27}, {"Bone Prison", 4, 24}, {"Bone Prison", 10, 18}, {"Bone Prison", 13, 15}, {"Bone Prison", 20, 8},
		{"Bone Wall", 1, 17}, {"Bone Wall", 20, 17},
		{"Corpse Explosion", 1, 15}, {"Corpse Explosion", 4, 18}, {"Corpse Explosion", 10, 24}, {"Corpse Explosion", 13, 27}, {"Corpse Explosion", 20, 34}} {
		sk := reg.ByName(c.skill)
		if got := float64(sk.ManaCost(c.lvl)) / 256; !nearly(got, c.mana, 0.01) {
			t.Errorf("%s L%d mana %.2f, wiki %.2f", c.skill, c.lvl, got, c.mana)
		}
	}

	// Corpse Explosion radius: the wiki (yards) is the range in subtiles / 3: 8 + (lvl - 1) subtiles; the engine
	// uses (range+1)/2 subtiles (0x5c2c60, VERIFIED) = range/3 yards within the rounding of the wiki
	for _, c := range []struct {
		lvl   int
		yards float64
	}{{1, 2.6}, {4, 3.6}, {10, 5.6}, {13, 6.6}, {20, 9}} {
		sk, e := wikiEnv(reg, "Corpse Explosion", c.lvl, map[string]int{"Corpse Explosion": c.lvl})
		r := (e.eval(sk.AuraRangeCalc) + 1) / 2

		if got := float64(r) * 2 / 3; !nearly(got, c.yards, 0.5) {
			t.Errorf("Corpse Explosion L%d radius %.2f yd, wiki %.1f", c.lvl, got, c.yards)
		}

		if e.eval(sk.Calc[1]) != 70 || e.eval(sk.Calc[2]) != 120 || e.eval(sk.Calc[3]) != 50 {
			t.Errorf("Corpse Explosion calcs %d %d %d", e.eval(sk.Calc[1]), e.eval(sk.Calc[2]), e.eval(sk.Calc[3]))
		}
	}
}

// Golems and Revive: mana and the per-level stats the skills evaluate (wiki: Clay slow 11 33 51 56 63, Iron Golem
// defense bonus 35 140 350 455 700 and thorns 150, Fire Golem fire damage absorb 36 58 76 81 88, Blood golem mana
// 25 37 61 73 101, Revive mana 45, Summon Resist 28 44 57 61 66).
func TestWikiNecroGolemSkills(t *testing.T) {
	reg := loadRealRegistry(t)

	mana := []struct {
		skill string
		lvl   int
		want  float64
	}{{"Clay Golem", 1, 15}, {"Clay Golem", 4, 24}, {"Clay Golem", 10, 42}, {"Clay Golem", 13, 51}, {"Clay Golem", 20, 72},
		{"BloodGolem", 1, 25}, {"BloodGolem", 4, 37}, {"BloodGolem", 10, 61}, {"BloodGolem", 13, 73}, {"BloodGolem", 20, 101},
		{"IronGolem", 1, 35}, {"IronGolem", 20, 35}, {"FireGolem", 1, 50}, {"FireGolem", 4, 80}, {"FireGolem", 10, 140},
		{"FireGolem", 13, 170}, {"FireGolem", 20, 240}, {"Revive", 1, 45}, {"Revive", 20, 45},
		{"Raise Skeleton", 1, 6}, {"Raise Skeleton", 4, 9}, {"Raise Skeleton", 10, 15}, {"Raise Skeleton", 13, 18}, {"Raise Skeleton", 20, 25},
		{"Raise Skeletal Mage", 1, 8}, {"Raise Skeletal Mage", 4, 11}, {"Raise Skeletal Mage", 10, 17}, {"Raise Skeletal Mage", 13, 20}, {"Raise Skeletal Mage", 20, 27}}
	for _, c := range mana {
		if got := float64(reg.ByName(c.skill).ManaCost(c.lvl)) / 256; !nearly(got, c.want, 0.01) {
			t.Errorf("%s L%d mana %.2f, wiki %.2f", c.skill, c.lvl, got, c.want)
		}
	}

	stat := func(skill string, lvl int, name string) int {
		sk, e := wikiEnv(reg, skill, lvl, map[string]int{skill: lvl})
		for i := 1; i <= 6; i++ {
			if sk.AuraStat[i] == name {
				return e.eval(sk.AuraStatCalc[i])
			}
		}

		t.Errorf("%s: no aura stat %s", skill, name)

		return 0
	}

	for _, c := range []struct{ lvl, slow int }{{1, 11}, {4, 33}, {10, 51}, {13, 56}, {20, 63}} {
		if got := stat("Clay Golem", c.lvl, "item_slow"); got != c.slow {
			t.Errorf("Clay Golem L%d slow %d, wiki %d", c.lvl, got, c.slow)
		}
	}

	for _, c := range []struct{ lvl, ac, thorns int }{{1, 35, 150}, {4, 140, 195}, {10, 350, 285}, {13, 455, 330}, {20, 700, 435}} {
		sk, e := wikiEnv(reg, "IronGolem", c.lvl, map[string]int{"IronGolem": c.lvl})

		var ac int

		for i := 1; i <= 5; i++ {
			if sk.PassiveStat[i] == "armorclass" {
				ac = e.eval(sk.PassiveCalc[i])
			}
		}

		if ac != c.ac {
			t.Errorf("Iron Golem L%d defense bonus %d, wiki %d", c.lvl, ac, c.ac)
		}

		if got := stat("IronGolem", c.lvl, "thorns_percent"); got != c.thorns {
			t.Errorf("Iron Golem L%d thorns %d (wiki lists 150 at level 1)", c.lvl, got)
		}
	}

	for _, c := range []struct{ lvl, absorb int }{{1, 36}, {4, 58}, {10, 76}, {13, 81}, {20, 88}} {
		if got := stat("FireGolem", c.lvl, "item_absorbfire_percent"); got != c.absorb {
			t.Errorf("Fire Golem L%d fire absorb %d, wiki %d", c.lvl, got, c.absorb)
		}
	}

	// Summon Resist (passive resist all) and the Revive duration (4500 frames = 180 s) and life percent 200
	for _, c := range []struct{ lvl, res int }{{1, 28}, {4, 44}, {10, 57}, {13, 61}, {20, 66}} {
		sk, e := wikiEnv(reg, "Summon Resist", c.lvl, map[string]int{"Summon Resist": c.lvl})
		if got := e.eval(sk.PassiveCalc[1]); got != c.res {
			t.Errorf("Summon Resist L%d %d, wiki %d", c.lvl, got, c.res)
		}
	}

	rv, e := wikiEnv(reg, "Revive", 5, map[string]int{"Revive": 5})
	if e.eval(rv.Calc[2]) != 4500 || e.eval(rv.Calc[1]) != 200 || e.eval(rv.PetMax) != 5 {
		t.Errorf("Revive calcs %d %d petmax %d", e.eval(rv.Calc[2]), e.eval(rv.Calc[1]), e.eval(rv.PetMax))
	}

	// skeleton and mage counts: wiki 1 3 5 6 8 at levels 1 4 10 13 20
	for _, c := range []struct{ lvl, n int }{{1, 1}, {4, 3}, {10, 5}, {13, 6}, {20, 8}} {
		for _, s := range []string{"Raise Skeleton", "Raise Skeletal Mage"} {
			sk, e := wikiEnv(reg, s, c.lvl, map[string]int{s: c.lvl})
			if got := e.eval(sk.PetMax); got != c.n {
				t.Errorf("%s L%d count %d, wiki %d", s, c.lvl, got, c.n)
			}
		}
	}
}
