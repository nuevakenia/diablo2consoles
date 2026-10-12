package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// TestW30WikiBossLife compares story-boss level, life and experience with community pages (LoD 1.14 numbers):
// Andariel life 1024/24800/60031 (diablo2.diablowiki.net, purediablo d2wiki); Diablo level 40/62/94, life
// 13818/90749/113812, experience 44902/465362/2195808 (diablo2.wiki.fextralife.com); Baal normal 26484 level 60 and
// hell 493701 level 99, Mephisto normal 6036 level 26 and hell 94320 level 87 (rankedboost, D2R, one source). The
// engine reads the same numbers from the tables (wiki-confirmed, table-confirmed). Skipped without D2_TABLES.
func TestW30WikiBossLife(t *testing.T) {
	_, rm := loadReal(t)

	type row struct {
		key       string
		diff, lvl int
		life, xp  int // xp 0: not checked
	}

	rows := []row{
		{"andariel", 0, 12, 1024, 0}, {"andariel", 1, 49, 24800, 0},
		{"andariel", 2, 75, 60031, 0},
		{"diablo", 0, 40, 13818, 44902}, {"diablo", 1, 62, 90749, 465362},
		{"diablo", 2, 94, 113812, 2195808},
		{"baalcrab", 0, 60, 26484, 0}, {"baalcrab", 2, 99, 493701, 0},
		{"mephisto", 0, 26, 6036, 0}, {"mephisto", 2, 87, 94320, 0},
	}

	for _, r := range rows {
		diff := d2monster.Difficulty(r.diff)
		dir := realDirector(rm, diff, 0, false)
		st := rm.Monster.Stats[r.key]
		b := d2monster.NewBrain(1, 1, diff, &d2monster.Profile{}, 1)
		v := dir.computeVitals(st, b)

		if v.Level != r.lvl || v.MaxHP != r.life || (r.xp != 0 && v.Experience != r.xp) {
			t.Errorf("%s d%d: level %d life %d xp %d, wiki %d/%d/%d", r.key, r.diff, v.Level, v.MaxHP, v.Experience,
				r.lvl, r.life, r.xp)
		}
	}
}
