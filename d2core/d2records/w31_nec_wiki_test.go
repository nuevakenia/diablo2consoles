package d2records

import "testing"

// Wave 31: the Fextralife Raise Skeleton damage column is the engine column moved up one level (its level 20 row
// repeats level 19). Wiki level N shows the engine value of level N+1: 9 -> 4-5, 10 -> 6-7, 13 -> 12-14 (engine
// level 14), 19 -> 37-39 (engine level 20). Notes: ~/git/d2-re-notes/w31-nec-wiki.md.
func TestW31SkeletonWikiDamageIsOffByOneLevel(t *testing.T) {
	tp := wikiTemplates(t)

	for _, c := range []struct{ engineLvl, lo, hi int }{{10, 4, 5}, {11, 6, 7}, {12, 8, 9}, {14, 12, 14}, {20, 37, 39}} {
		if _, s := necroSummon(t, tp, "Raise Skeleton", map[string]int{"Raise Skeleton": c.engineLvl}, 30); s.DmgMin != c.lo || s.DmgMax != c.hi {
			t.Errorf("skeleton L%d damage %d-%d, want %d-%d (wiki row %d)", c.engineLvl, s.DmgMin, s.DmgMax, c.lo, c.hi, c.engineLvl-1)
		}
	}
}
