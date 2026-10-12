package d2herostats

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
)

// Wiki cross-check (wave 30). Wiki numbers are the community class tables and
// breakpoint tables (Arreat Summit / Maxroll, 1.14; recalled, WIKI-ONLY
// confidence). The engine side is computed from D2_TABLES.

func wikiTables(t *testing.T, name string) []byte {
	t.Helper()

	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES not set")
	}

	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Skip(err)
	}

	return b
}

func TestWikiClassStartAndGrowth(t *testing.T) {
	classes, err := d2statlist.ParseClasses(wikiTables(t, "CharStats.txt"))
	if err != nil {
		t.Fatal(err)
	}

	type row struct {
		str, dex, vit, ene int
		life, mana, stam   int
		lifeLvl4, manaLvl4 int
		stamLvl4           int
		lifeVit4, stamVit4 int
		manaEne4           int
	}
	// quarter points for the per-level and per-point numbers (8 = 2.0).
	want := map[string]row{
		"Amazon":      {20, 25, 20, 15, 50, 15, 84, 8, 6, 4, 12, 4, 6},
		"Sorceress":   {10, 25, 10, 35, 40, 35, 74, 4, 8, 4, 8, 4, 8},
		"Necromancer": {15, 25, 15, 25, 45, 25, 79, 6, 8, 4, 8, 4, 8},
		"Paladin":     {25, 20, 25, 15, 55, 15, 89, 8, 6, 4, 12, 4, 6},
		"Barbarian":   {30, 20, 25, 10, 55, 10, 92, 8, 4, 4, 16, 4, 4},
		"Druid":       {15, 20, 25, 20, 55, 20, 84, 6, 8, 4, 8, 4, 8},
		"Assassin":    {20, 20, 20, 25, 50, 25, 95, 8, 6, 5, 12, 5, 7},
	}

	for name, w := range want {
		c := classes[name]
		life, mana, stam := c.BaseMax(1, c.InitVit, c.InitEne)

		// The table lists vit before ene in different order than the wiki, so
		// compare starting stats through the class record.
		got := row{c.InitStr, c.InitDex, c.InitVit, c.InitEne, life, mana, stam,
			c.LifePerLevel, c.ManaPerLevel, c.StaminaPerLevel, c.LifePerVit, c.StaminaPerVit, c.ManaPerEne}

		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s: engine %+v wiki %+v", name, got, w)
		}
	}
}

func TestWikiLevelUpAndPoints(t *testing.T) {
	// wiki: 5 stat points and 1 skill point per level for every class.
	if StatPointsPerLevel != 5 || SkillPointsPerLevel != 1 {
		t.Fatalf("points per level %d/%d", StatPointsPerLevel, SkillPointsPerLevel)
	}
	// wiki: vitality on a Barbarian gives 4 life, 1 stamina; energy on a
	// Sorceress gives 2 mana.
	classes, err := d2statlist.ParseClasses(wikiTables(t, "CharStats.txt"))
	if err != nil {
		t.Fatal(err)
	}

	b := classes["Barbarian"]
	life, _, stam := b.BaseMax(1, b.InitVit+10, b.InitEne)
	l0, _, s0 := b.BaseMax(1, b.InitVit, b.InitEne)

	if life-l0 != 40 || stam-s0 != 10 {
		t.Errorf("barbarian +10 vit: life +%d stamina +%d", life-l0, stam-s0)
	}

	s := classes["Sorceress"]
	_, mana, _ := s.BaseMax(1, s.InitVit, s.InitEne+10)
	_, m0, _ := s.BaseMax(1, s.InitVit, s.InitEne)

	if mana-m0 != 20 {
		t.Errorf("sorceress +10 ene: mana +%d", mana-m0)
	}
	// wiki: Barbarian level 99 with no vit points: 55 + 98*2 = 251 life.
	life, _, _ = b.BaseMax(99, b.InitVit, b.InitEne)
	if life != 55+98*2 {
		t.Errorf("barbarian level 99 base life %d", life)
	}
}

func TestWikiExperienceTable(t *testing.T) {
	tabs, err := ParseExperience(wikiTables(t, "Experience.txt"))
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range ClassNames {
		tab := tabs[c]
		// wiki: level 2 at 500, 3 at 1500, 4 at 3750, 5 at 7875, 6 at 14175,
		// level 99 at 3520485254 (also the experience cap).
		for lvl, exp := range map[int]int64{2: 500, 3: 1500, 4: 3750, 5: 7875, 6: 14175, 99: 3520485254} {
			if got := tab.Threshold[lvl-1]; got != exp {
				t.Errorf("%s level %d needs %d, wiki %d", c, lvl, got, exp)
			}
		}

		if tab.LevelFor(3520485254) != 99 || tab.LevelFor(3520485253) != 98 {
			t.Errorf("%s level 98/99 edge", c)
		}

		if tab.ExpCap() != 3520485254 {
			t.Errorf("%s cap %d", c, tab.ExpCap())
		}
	}
}

func TestWikiKillXPLevelPenalty(t *testing.T) {
	// wiki (Arreat level-difference table, character above monster): diff 0-5
	// 100%, 6 81%, 7 62%, 8 43%, 9 24%, 10+ 5%. Engine uses 207/256 etc.
	wantPct := []int{100, 100, 100, 100, 100, 100, 81, 62, 43, 24, 5, 5, 5}

	for diff, w := range wantPct {
		got := LevelScaleXP(10000, 50, 50+diff) // monster 50, char above by diff
		if p := got / 100; p < w-1 || p > w+1 {
			t.Errorf("char above by %d: %d%%, wiki %d%%", diff, got/100, w)
		}
	}
	// wiki: char level 25+ against a stronger monster: xp * clvl / mlvl.
	if got := LevelScaleXP(10000, 60, 30); got != 5000 {
		t.Errorf("monster above clvl 30: %d", got)
	}
	// wiki: below clvl 25 the table is 100%, 100.., then 88%,68%,36%,15%,2%.
	for diff, w := range []int{100, 100, 100, 100, 100, 100, 88, 68, 36, 15, 2} {
		if got := LevelScaleXP(10000, 10+diff, 10) / 100; got < w-1 || got > w+1 {
			t.Errorf("monster above by %d: %d%% wiki %d%%", diff, got, w)
		}
	}
	// wiki: a level 99 character earns nothing.
	if KillXP(1000, 90, 99, 99, 0) != 0 {
		t.Errorf("level 99 gains xp")
	}
}

func TestWikiBreakpoints(t *testing.T) {
	// wiki breakpoint rows (FCR/FHR/FBR); engine tables are derived from the
	// exe rules plus animdata frame counts.
	want := map[Kind]map[string][]int{
		FCR: {
			"Amazon": {0, 7, 14, 22, 32, 48, 68, 99, 152}, "Sorceress": {0, 9, 20, 37, 63, 105, 200},
			"Necromancer": {0, 9, 18, 30, 48, 75, 125}, "Paladin": {0, 9, 18, 30, 48, 75, 125},
			"Barbarian": {0, 9, 20, 37, 63, 105, 200}, "Druid": {0, 4, 10, 19, 30, 46, 68, 99, 163},
			"Assassin": {0, 8, 16, 27, 42, 65, 102, 174},
		},
		FHR: {
			"Amazon": {0, 6, 13, 20, 32, 52, 86, 174, 600}, "Sorceress": {0, 5, 9, 14, 20, 30, 42, 60, 86, 142, 280},
			"Necromancer": {0, 5, 10, 16, 26, 39, 56, 86, 152, 377}, "Paladin": {0, 7, 15, 27, 48, 86, 200},
			"Barbarian": {0, 7, 15, 27, 48, 86, 200}, "Assassin": {0, 7, 15, 27, 48, 86, 200},
			"Druid": {0, 3, 7, 13, 19, 29, 42, 63, 99, 174, 456},
		},
		FBR: {
			"Amazon": {0, 13, 32, 86, 600}, "Sorceress": {0, 7, 15, 27, 48, 86, 200},
			"Necromancer": {0, 6, 13, 20, 32, 52, 86, 174, 600}, "Barbarian": {0, 9, 20, 42, 86, 280},
			"Druid": {0, 6, 13, 20, 32, 52, 86, 174, 600}, "Assassin": {0, 13, 32, 86, 600},
		},
	}

	for kind, m := range want {
		for class, w := range m {
			e, ok := Lookup(class, kind)
			if !ok {
				t.Errorf("%s %s missing", class, kind)
				continue
			}

			if !reflect.DeepEqual(e.Table, w) {
				t.Errorf("%s %s: engine %v wiki %v", class, kind, e.Table, w)
			}
		}
	}
	// wiki: diminishing returns 120*p/(120+p) for IAS/FCR/FHR/FBR.
	if got := EffectivePct(60); got != 40 {
		t.Errorf("effective 60 -> %d", got)
	}
}

func TestWikiRegenAndStamina(t *testing.T) {
	// wiki: mana regenerates fully in 120 seconds (charstats ManaRegen) with
	// no bonus; +100% regenerate mana halves that. Life has no natural regen.
	maxRaw := 1000 * 256 // truncation of maxRaw/3000 makes small pools slower (exe-confirmed)
	per := ManaRegenPerFrame(maxRaw, 120, 0, 0, false)

	frames := maxRaw / per
	if frames < 2900 || frames > 3100 {
		t.Errorf("full mana in %d frames, wiki 3000", frames)
	}

	if got := LifeRegenStep(1000, 5000, 0); got != 1000 {
		t.Errorf("natural life regen %d", got)
	}
	// wiki: the run drain doubles with each 10 armor weight? (UNVERIFIED wiki
	// number); engine: runDrain*2 per step, times weight/10+1 with body armor.
	if got := StaminaDrainPerStep(20, -1, 0, false); got != 40 {
		t.Errorf("drain %d", got)
	}
	// wiki: no drain in town.
	if StaminaDrainPerStep(20, 50, 0, true) != 0 {
		t.Errorf("town drain")
	}
}
