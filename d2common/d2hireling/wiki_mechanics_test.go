package d2hireling

import (
	"os"
	"path/filepath"
	"testing"
)

// Wiki cross-check (wave 30): mercenary numbers from the community pages
// (recalled, WIKI-ONLY confidence) against hireling.txt (D2_TABLES) through
// the engine's StatsFor.

func wikiMercTable(t *testing.T) *Table {
	t.Helper()

	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES not set")
	}

	f, err := os.Open(filepath.Join(dir, "hireling", "hireling.txt"))
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()

	tab, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}

	return tab
}

func TestWikiMercExperience(t *testing.T) {
	// wiki: experience for merc level L is (L+1)*L*L*ExpPerLevel(%).
	if got := ExpThreshold(10, 100); got != 11*100*100 {
		t.Errorf("threshold %d", got)
	}
	// wiki: a merc cannot reach level 99 by experience (cap 98).
	if MaxLevel != 98 {
		t.Errorf("max merc level %d", MaxLevel)
	}
}

func TestWikiMercPerLevelGrowth(t *testing.T) {
	tab := wikiMercTable(t)

	// per level life: Rogue 8, Desert Mercenary 10, Eastern Sorcerer 6,
	// Barbarian 12 (normal difficulty; wiki). Id order: Rogue 0, Desert 1,
	// Sorcerer 2, Barbarian 3 in hireling.txt rows of difficulty 1.
	for _, tc := range []struct {
		name  string
		hp    int
		first int // a level inside the first row's range
	}{
		{"Rogue Scout", 8, 3}, {"Desert Mercenary", 10, 9},
		{"Eastern Sorceror", 6, 15}, {"Barbarian", 12, 28},
	} {
		var r *Record

		for _, x := range tab.Rows {
			if x.Hireling == tc.name && x.Difficulty == 1 && x.Level == tc.first {
				r = x
				break
			}
		}

		if r == nil {
			t.Errorf("%s: no row", tc.name)
			continue
		}

		a, b := r.StatsAt(tc.first), r.StatsAt(tc.first+10)
		if b.MaxHP-a.MaxHP != tc.hp*10 {
			t.Errorf("%s: life/level %d, wiki %d", tc.name, (b.MaxHP-a.MaxHP)/10, tc.hp)
		}
	}
}

func TestWikiMercCosts(t *testing.T) {
	// wiki: revive cost grows with level squared and is capped at 50000.
	if ReviveCost(1) != 0 || ReviveCost(30) != 6750 || ReviveCost(98) != 50000 {
		t.Errorf("revive %d %d %d", ReviveCost(1), ReviveCost(30), ReviveCost(98))
	}
	// wiki: hire offers are the owner's level minus 1..5, at least 2.
	if OfferLevel(1, 0) != 2 || OfferLevel(40, 4) != 39 || OfferLevel(40, 0) != 35 {
		t.Errorf("offer levels")
	}
}
