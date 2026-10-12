package d2skill

import "testing"

// Wave 31: the Fextralife Necromancer damage tables that sat 1 to 15 percent above the engine are the 1.14b
// formulas evaluated with the prerequisite skills at one hard point (their synergy donors): Bone Spear has Teeth 1
// (7 percent), Bone Spirit Teeth 1 + Bone Spear 1 (12), Poison Explosion Poison Dagger 1 (15). Notes:
// ~/git/d2-re-notes/w31-nec-wiki.md.
func TestW31NecroWikiIncludesPrerequisiteSynergy(t *testing.T) {
	reg := loadRealRegistry(t)

	type c struct {
		skill    string
		lvl      int
		donors   map[string]int
		wlo, whi int // wiki
		frames   int // 0: per hit
		tol      int
	}

	teeth := map[string]int{"Teeth": 1}
	spirit := map[string]int{"Teeth": 1, "Bone Spear": 1}
	dagger := map[string]int{"Poison Dagger": 1}

	for _, w := range []c{
		{"Bone Spear", 1, teeth, 17, 25, 0, 0}, {"Bone Spear", 10, teeth, 96, 104, 0, 0}, {"Bone Spear", 20, teeth, 205, 218, 0, 0},
		{"Bone Spirit", 1, spirit, 22, 33, 0, 0}, {"Bone Spirit", 10, spirit, 185, 207, 0, 0}, {"Bone Spirit", 20, spirit, 380, 413, 0, 0},
		{"Poison Explosion", 1, dagger, 28, 86, 50, 0}, {"Poison Explosion", 20, dagger, 1345, 1620, 240, 3},
	} {
		sk, e := wikiEnv(reg, w.skill, w.lvl, w.donors)
		lo, hi := int(sk.ElemMin(e, w.lvl)), int(sk.ElemMax(e, w.lvl))

		if w.frames > 0 {
			lo, hi = int(int64(lo)*int64(w.frames)>>8), int(int64(hi)*int64(w.frames)>>8)
		} else {
			lo, hi = lo>>8, hi>>8
		}

		if absInt(lo-w.wlo) > w.tol || absInt(hi-w.whi) > w.tol {
			t.Errorf("%s L%d with %v: %d-%d, wiki %d-%d", w.skill, w.lvl, w.donors, lo, hi, w.wlo, w.whi)
		}
	}
}
