package d2hireling

import "testing"

// Wave 31: hire prices and the per level columns (HP, defense, attack rating, damage, resist) of the four
// mercenary kinds, checked against the table and the exe formulas (HIRE_GenerateOfferStats 0x666190,
// MERC_SetLevelStats 0x570690).

func TestHireCostFormula(t *testing.T) {
	// the price is (15 percent of the base per level above the row's base level, plus 100 percent) of the Gold
	// column, never below the Gold column
	r := &Record{Level: 3, Gold: 100}
	for _, tc := range []struct{ level, want int }{{2, 100}, {3, 100}, {4, 115}, {13, 250}, {43, 700}} {
		if got := HireCost(r, tc.level); got != tc.want {
			t.Errorf("rogue-like gold 100 level %d: %d, want %d", tc.level, got, tc.want)
		}
	}

	// the integer division is on the product: 15 * 7 + 100 = 205 of 150 gold is 307
	if got := HireCost(&Record{Level: 3, Gold: 150}, 10); got != 307 {
		t.Errorf("gold 150 level 10: %d", got)
	}
}

func TestHireCostTable(t *testing.T) {
	tab := wikiMercTable(t)

	// the Gold column of the first row of each kind (Version 100, the expansion rows): normal difficulty
	// prices of the four mercenary kinds; nightmare and hell start much higher.
	type key struct {
		name string
		diff int
	}

	first := map[key]*Record{}

	for _, r := range tab.Rows {
		if r.Version != 100 {
			continue
		}

		k := key{r.Hireling, r.Difficulty}
		if first[k] == nil || r.Level < first[k].Level {
			first[k] = r
		}
	}

	want := map[key][2]int{ // gold, base level of the first row
		{"Rogue Scout", 1}:      {100, 3},
		{"Desert Mercenary", 1}: {350, 9},
		{"Eastern Sorceror", 1}: {1000, 15},
		{"Barbarian", 1}:        {9000, 28},
		{"Rogue Scout", 2}:      {6000, 25},
		{"Desert Mercenary", 2}: {7900, 31},
		{"Eastern Sorceror", 2}: {9500, 37},
		{"Barbarian", 2}:        {18000, 42},
		{"Rogue Scout", 3}:      {12500, 49},
		{"Desert Mercenary", 3}: {15000, 55},
		{"Eastern Sorceror", 3}: {21000, 61},
		{"Barbarian", 3}:        {32000, 75},
	}

	for k, w := range want {
		r := first[k]
		if r == nil {
			t.Errorf("%v: no row", k)
			continue
		}

		if r.Gold != w[0] || r.Level != w[1] {
			t.Errorf("%v: gold %d level %d, want %d %d", k, r.Gold, r.Level, w[0], w[1])
		}

		// the price one band above the base level
		if got, exp := HireCost(r, r.Level+10), w[0]*250/100; got != exp {
			t.Errorf("%v: cost at +10 levels %d, want %d", k, got, exp)
		}
	}
}

// The level bands of a kind are designed to join up: the stats a band reaches at the base level of the next band
// equal that band's starting values (within one point of rounding), which pins the per level columns, the 1/8
// scale of strength, dexterity and damage per level and the 1/4 scale of the resistance.
func TestMercBandsJoin(t *testing.T) {
	tab := wikiMercTable(t)

	type key struct {
		name          string
		diff, id, ver int
	}

	byKey := map[key][]*Record{}

	for _, r := range tab.Rows {
		k := key{r.Hireling, r.Difficulty, r.ID, r.Version}
		byKey[k] = append(byKey[k], r)
	}

	joined := 0

	for k, rows := range byKey {
		for i := 0; i+1 < len(rows); i++ {
			a, b := rows[i], rows[i+1]
			if b.Level <= a.Level {
				continue
			}

			got, want := a.StatsAt(b.Level), b.StatsAt(b.Level)

			near := func(x, y, tol int) bool { return x-y <= tol && y-x <= tol }

			// the table itself is not perfectly continuous in a few nightmare bands (hp up to 8, defense up to 3 off)
			if !near(got.MaxHP, want.MaxHP, 8) || !near(got.Defense, want.Defense, 3) {
				t.Errorf("%v level %d: hp %d/%d defense %d/%d", k, b.Level, got.MaxHP, want.MaxHP, got.Defense, want.Defense)
			}

			if !near(got.AR, want.AR, 1) || !near(got.DmgMin, want.DmgMin, 1) || !near(got.DmgMax, want.DmgMax, 1) ||
				!near(got.Resist, want.Resist, 1) || !near(got.Str, want.Str, 1) || !near(got.Dex, want.Dex, 1) {
				t.Errorf("%v level %d: engine %+v next band %+v", k, b.Level, got, want)
			}

			joined++
		}
	}

	if joined < 20 {
		t.Errorf("only %d band joins checked", joined)
	}
}

// Exact per level numbers of the four kinds in normal difficulty at a few levels (hand checked against the
// table columns: HP per level 8 / 10 / 6 / 12, AR per level, damage per level / 8).
func TestMercStatColumns(t *testing.T) {
	tab := wikiMercTable(t)

	find := func(name string, id, level int) *Record {
		for _, r := range tab.Rows {
			if r.Hireling == name && r.ID == id && r.Version == 100 && r.Level == level {
				return r
			}
		}

		return nil
	}

	// Desert Mercenary (Combat, id 6): base row level 9: hp 120+10/lvl, AR 20+4/lvl, damage 7-14 (+4 per 8 levels)
	d := find("Desert Mercenary", 6, 9)
	if d == nil {
		t.Fatal("no desert mercenary row")
	}

	s := d.StatsAt(31)
	if s.MaxHP != 340 || s.AR != 108 || s.DmgMin != 18 || s.DmgMax != 25 {
		t.Errorf("desert level 31: %+v", s)
	}

	// Barbarian (id 24): base row level 28: hp 288+12/lvl, AR 150+6/lvl
	b := find("Barbarian", 24, 28)
	if b == nil {
		t.Fatal("no barbarian row")
	}

	s = b.StatsAt(42)
	if s.MaxHP != 456 || s.AR != 234 {
		t.Errorf("barbarian level 42: %+v", s)
	}

	// Rogue and Eastern Sorceror have no attack rating column (their weapon's bonus comes from items only)
	if r := find("Rogue Scout", 0, 3); r == nil || r.StatsAt(30).AR != 0 {
		t.Errorf("rogue AR")
	}

	if r := find("Eastern Sorceror", 15, 15); r == nil || r.StatsAt(50).AR != 0 {
		t.Errorf("sorceror AR")
	}
}
