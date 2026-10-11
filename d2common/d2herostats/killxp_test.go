package d2herostats

import "testing"

func TestLevelScaleXP(t *testing.T) {
	for _, tc := range []struct {
		name         string
		xp, mlv, clv int
		want         int
	}{
		{"same level", 1000, 30, 30, 1000},
		{"char 5 above", 1000, 25, 30, 1000},
		{"char 6 above", 2560, 24, 30, 2070},
		{"char 7 above", 2560, 23, 30, 1590},
		{"char 10 above", 2560, 20, 30, 130},
		{"char 40 above caps at 10", 2560, 1, 41, 130},
		{"low char, monster 6 above", 2560, 16, 10, 2250},
		{"low char, monster 9 above", 2560, 19, 10, 380},
		{"low char, monster far above", 2560, 60, 10, 50},
		{"clvl 25 monster above is xp*c/m", 1000, 40, 25, 625},
		{"clvl 24 monster above uses table", 2560, 40, 24, 50},
		{"monster level 0 counts as 30 below", 100, 0, 30, 5},
	} {
		if got := LevelScaleXP(tc.xp, tc.mlv, tc.clv); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestKillXP(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		xp, m, c, max, pct, want int
	}{
		{"same level unchanged", 500, 40, 40, 99, 0, 500},
		{"item bonus", 500, 40, 40, 99, 20, 600},
		{"cap", 1 << 30, 40, 40, 99, 0, 0x7fffff},
		{"zero counts as one", 0, 40, 40, 99, 0, 1},
		{"max level gets nothing", 500, 40, 99, 99, 0, 0},
	} {
		if got := KillXP(tc.xp, tc.m, tc.c, tc.max, tc.pct); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestMercKillShareAndRatio(t *testing.T) {
	if MercKillShare(256, false) != 86 || MercKillShare(256, true) != 256 {
		t.Error("merc share")
	}

	if ExpRatioScale(1000, 1024, 10) != 1000 || ExpRatioScale(1000, 512, 10) != 500 || ExpRatioScale(-3, 1024, 10) != -3 {
		t.Error("ratio")
	}
}

func TestExpRatioScale(t *testing.T) {
	for _, tc := range []struct {
		name            string
		xp, ratio, shft int
		want            int
	}{
		{"1024>>10 small xp is exact", 1000, 1024, 10, 1000},
		{"limit edge uses multiply path", 1048575, 1024, 10, 1048575},
		{"above limit drops low bits", 1048576 + 1023, 1024, 10, 1048576},
		{"big xp multiple of 1024 kept", 8388607, 1024, 10, 8387584},
		{"level 70 ratio 976", 100000, 976, 10, 95312},
		{"level 99 ratio 5", 100000, 5, 10, 488},
		{"zero stays", 0, 976, 10, 0},
		{"bad shift returns xp", 500, 5, 0, 500},
		{"shift 32 returns xp", 500, 5, 32, 500},
	} {
		if got := ExpRatioScale(tc.xp, tc.ratio, tc.shft); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestKillXPUsesInstalledRatio(t *testing.T) {
	defer SetExpRatio(nil)

	by := make([]int, 100)
	for i := range by {
		by[i] = 1024
	}

	by[85] = 256

	SetExpRatio(&ExpRatio{Shift: 10, ByLevel: by})

	if got := KillXP(4000, 85, 85, 99, 0); got != 1000 {
		t.Errorf("level 85 ratio 256: got %d want 1000", got)
	}

	if got := KillXP(4000, 50, 50, 99, 0); got != 4000 {
		t.Errorf("level 50 ratio 1024: got %d want 4000", got)
	}

	// the item bonus is added after the ratio
	if got := KillXP(4000, 85, 85, 99, 50); got != 1500 {
		t.Errorf("ratio then +50%%: got %d want 1500", got)
	}

	SetExpRatio(nil)

	if got := KillXP(4000, 85, 85, 99, 0); got != 4000 {
		t.Errorf("no table: got %d want 4000", got)
	}
}
