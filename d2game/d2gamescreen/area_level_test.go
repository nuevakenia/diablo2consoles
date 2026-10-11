package d2gamescreen

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestAreaMonsterLevel(t *testing.T) {
	l := &d2records.LevelDetailRecord{
		MonsterLevelNormal: 1, MonsterLevelNightmare: 36, MonsterLevelHell: 67,
		MonsterLevelNormalEx: 2, MonsterLevelNightmareEx: 37, MonsterLevelHellEx: 68,
	}

	for _, c := range []struct {
		diff int
		ex   bool
		want int
	}{
		{0, false, 1}, {1, false, 36}, {2, false, 67},
		{0, true, 2}, {1, true, 37}, {2, true, 68},
		{-1, true, 0}, {3, false, 0},
	} {
		if got := areaMonsterLevel(l, c.diff, c.ex); got != c.want {
			t.Errorf("difficulty %d expansion %v: got %d, want %d", c.diff, c.ex, got, c.want)
		}
	}

	blank := &d2records.LevelDetailRecord{MonsterLevelNormal: 5}
	if got := areaMonsterLevel(blank, 0, true); got != 5 {
		t.Errorf("blank Ex column: got %d, want the classic 5", got)
	}

	if got := areaMonsterLevel(nil, 0, false); got != 0 {
		t.Errorf("nil record: got %d, want 0", got)
	}
}
