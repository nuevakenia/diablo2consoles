package d2monsters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// TestW30WikiRankBonuses checks the rank bonuses against the Arreat Summit LoD page (classic.battle.net/diablo2exp/
// monsters/bonus.shtml, undated, 2019 footer): life x3/2.5/2 (champion), x4/3/2 (unique), x2/1.75/1.5 (minion);
// level +2 (champion), +3 (unique, minion); experience x3 (champion), x5 (unique, minion); damage +90/75/66 (champion),
// +135/112/99 (unique Extra Strong), +67/56/49 (minion Extra Strong); to-hit +67/56/49, +90/75/66, +45/37/33.
// The engine runs the exe callbacks; the numbers below are the wiki's. Skipped without D2_TABLES.
func TestW30WikiRankBonuses(t *testing.T) {
	root, rm := loadReal(t)

	buf, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "monumod.txt"))
	if err != nil {
		t.Skip(err)
	}

	if err = rm.Load(d2resource.MonsterUniqueModifier, d2txt.LoadDataDictionary(buf)); err != nil {
		t.Fatal(err)
	}

	st := rm.Monster.Stats["zombie1"]
	lifePct := map[string][3]int{"champ": {300, 250, 200}, "uniq": {400, 300, 200}, "minion": {200, 175, 150}}
	dmgPct := map[string][3]int{"champ": {90, 75, 66}, "uniq": {135, 112, 99}, "minion": {67, 56, 49}}
	thPct := map[string][3]int{"champ": {67, 56, 49}, "uniq": {90, 75, 66}, "minion": {45, 37, 33}}
	lvlAdd := map[string]int{"champ": 2, "uniq": 3, "minion": 3}
	xpMul := map[string]int{"champ": 3, "uniq": 5, "minion": 5}
	mods := map[string][]int{"champ": {36}, "uniq": {5}, "minion": {5}} // Ghostly champion, Extra Strong

	for d := 0; d < 3; d++ {
		diff := d2monster.Difficulty(d)
		dir := realDirector(rm, diff, 0, false)

		for _, rank := range []string{"champ", "uniq", "minion"} {
			b := d2monster.NewBrain(1, 1, diff, &d2monster.Profile{}, 1)
			base := dir.computeVitals(st, b)
			base.MaxHP, base.HP = 1000, 1000 // fixed life so the percent check is exact

			leader := &d2mapentity.Monster{Stat: st, Vitals: base}
			leader.TypeFlags = d2mapentity.MonTypeUnique

			if rank == "champ" {
				leader.TypeFlags = d2mapentity.MonTypeChampion
			}

			leader.Modifiers = mods[rank]
			m := leader

			if rank == "minion" {
				m = &d2mapentity.Monster{Stat: st, Vitals: base}
				dir.ApplyUModEffects(leader, []*d2mapentity.Monster{m})
			} else {
				dir.ApplyUModEffects(leader, nil)
			}

			v := m.Vitals
			if want := 1000 * lifePct[rank][d] / 100; v.MaxHP != want {
				t.Errorf("%s d%d life %d, wiki %d", rank, d, v.MaxHP, want)
			}

			if v.Level != base.Level+lvlAdd[rank] {
				t.Errorf("%s d%d level %d, wiki %d", rank, d, v.Level, base.Level+lvlAdd[rank])
			}

			// 5x for all, champions then lose two fifths (x3); integer steps allow a few points
			if want := base.Experience * xpMul[rank]; v.Experience < want-xpMul[rank] || v.Experience > want {
				t.Errorf("%s d%d xp %d, wiki %d", rank, d, v.Experience, want)
			}

			if want := base.A1.Max * (100 + dmgPct[rank][d]) / 100; v.A1.Max != want {
				t.Errorf("%s d%d A1 max %d, wiki %d (base %d)", rank, d, v.A1.Max, want, base.A1.Max)
			}

			if want := base.A1.ToHit * (100 + thPct[rank][d]) / 100; v.A1.ToHit != want {
				t.Errorf("%s d%d to-hit %d, wiki %d", rank, d, v.A1.ToHit, want)
			}
		}
	}
}
