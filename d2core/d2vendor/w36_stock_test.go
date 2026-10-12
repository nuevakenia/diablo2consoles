package d2vendor

import (
	"fmt"
	"sort"
	"testing"
)

// w36 vendor stock sweep over the real tables (D2_TABLES): every modelled
// vendor, the three difficulties and four player levels.

func w36Opts(_ interface{}, v Vendor, diff, lvl int, resolve func(string) (Base, bool)) Options {
	return Options{PlayerLevel: lvl, Tier: ActIndex(v), Difficulty: diff, Resolve: resolve}
}

func TestW36VendorStockSweep(t *testing.T) {
	rec := loadRecords(t)
	resolve := Resolver(rec)

	for _, v := range All() {
		bases := BasesFor(rec, v)
		if len(bases) == 0 {
			t.Errorf("%s: no bases", v.Name)

			continue
		}

		perm := map[string]Base{}

		for _, b := range bases {
			if b.Permanent {
				perm[b.Code] = b
			}
		}

		for diff := 0; diff < 3; diff++ {
			for _, lvl := range []int{1, 15, 40, 80} {
				seed := StockSeed(12345, v.ClassID, 0, false)
				s := GenerateSeeded(seed, bases, w36Opts(nil, v, diff, lvl, resolve))

				if len(s.Items) == 0 {
					t.Errorf("%s diff %d lvl %d: empty stock", v.Name, diff, lvl)

					continue
				}

				got := map[string]bool{}
				q := map[int]int{}
				maxIlvl := 0

				for _, it := range s.Items {
					got[it.Code] = true
					q[int(it.Quality)]++

					if it.ILvl > maxIlvl {
						maxIlvl = it.ILvl
					}
				}

				// permanent items are always in the stock
				// (from player level 26 on nightmare and hell the potions are replaced by
				// their upgrade, VERIFIED in stock.go)
				for c, b := range perm {
					up := b.NightmareUpgrade
					if diff == 2 {
						up = b.HellUpgrade
					}

					if !got[c] && !(diff > 0 && lvl >= 26 && up != "" && got[up]) {
						t.Errorf("%s diff %d lvl %d: permanent %s missing", v.Name, diff, lvl, c)
					}
				}

				// no item above the area-less sanity bound of the item level (99)
				if maxIlvl > 99 {
					t.Errorf("%s diff %d lvl %d: item level %d", v.Name, diff, lvl, maxIlvl)
				}

				if diff == 0 && lvl == 40 {
					codes := make([]string, 0, len(got))
					for c := range got {
						codes = append(codes, c)
					}

					sort.Strings(codes)
					t.Logf("%s normal lvl 40: %d items, max ilvl %d, quality %v, perm %d: %v", v.Name, len(s.Items), maxIlvl, q, len(perm), codes)
				}
			}
		}
	}
}

// Tomes and scrolls: the caster vendors keep a town portal / identify scroll and tome in stock.
func TestW36TomesAndScrolls(t *testing.T) {
	rec := loadRecords(t)

	want := map[string][]string{"Akara": {"tbk", "ibk", "tsc", "isc"}, "Drognan": {"tbk", "ibk", "tsc", "isc"}, "Ormus": {"tbk", "ibk", "tsc", "isc"}, "Malah": {"tbk", "ibk", "tsc", "isc"}}

	for name, codes := range want {
		v, ok := ByName(name)
		if !ok {
			t.Errorf("vendor %s missing", name)

			continue
		}

		have := map[string]bool{}
		for _, b := range BasesFor(rec, v) {
			have[b.Code] = true
		}

		var missing []string

		for _, c := range codes {
			if !have[c] {
				missing = append(missing, c)
			}
		}

		if len(missing) > 0 {
			t.Logf("%s does not list %v among its bases", name, missing)
		}

		if !have["tsc"] && !have["isc"] {
			t.Errorf("%s sells no scrolls", name)
		}
	}
}

// A restock (restock counter + 1) gives a different stock; the same counter reproduces it.
func TestW36RestockAndLevelRoll(t *testing.T) {
	rec := loadRecords(t)
	resolve := Resolver(rec)

	for _, v := range All() {
		bases := BasesFor(rec, v)
		sig := func(restock uint32, lvl int) string {
			s := GenerateSeeded(StockSeed(777, v.ClassID, restock, false), bases, w36Opts(nil, v, 0, lvl, resolve))
			out := ""

			for _, it := range s.Items {
				out += fmt.Sprintf("%s/%d/%d,", it.Code, it.Quality, it.ILvl)
			}

			return out
		}

		if sig(0, 30) != sig(0, 30) {
			t.Errorf("%s: same seed not reproducible", v.Name)
		}

		rolled := 0

		for _, b := range bases {
			if !b.Permanent {
				rolled++
			}
		}

		// potion only vendors (Lysander, Alkor) have nothing but permanent items
		if rolled > 0 && sig(0, 30) == sig(1, 30) {
			t.Errorf("%s: restock did not change the stock", v.Name)
		}

		// A different player level changes the item level of the rolled stock (the level
		// is an input of the generator; whether the exe re-rolls on a level up is UNVERIFIED).
		if sig(0, 5) == sig(0, 60) {
			t.Errorf("%s: level 5 and 60 stocks equal", v.Name)
		}
	}
}
