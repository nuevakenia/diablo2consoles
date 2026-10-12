package d2automap

import "testing"

func TestTextStack(t *testing.T) {
	width := func(s string) int { return len(s) * 10 }

	tests := []struct {
		name  string
		w     int
		lines []string
		want  []TextLine
	}{
		{"1512 level + version", 1512, []string{"", "", "Rogue Encampment", VersionText},
			[]TextLine{{"Rogue Encampment", 1512 - 160 - 16, 24}, {VersionText, 1512 - 70 - 16, 40}}},
		{"1920 level only", 1920, []string{"Den of Evil"},
			[]TextLine{{"Den of Evil", 1920 - 110 - 16, 24}}},
		{"800 three lines", 800, []string{"G", "L", "v"},
			[]TextLine{{"G", 774, 24}, {"L", 774, 40}, {"v", 774, 56}}},
	}

	for _, tc := range tests {
		got := TextStack(tc.lines, tc.w, width)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}

		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s line %d: got %v want %v", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

func TestDifficultyStringID(t *testing.T) {
	for flag, want := range map[int]int{0: 0, 1: 0x1423, 2: 0x1422, 3: 0} {
		if got := DifficultyStringID(flag); got != want {
			t.Errorf("flag %d: got %#x want %#x", flag, got, want)
		}
	}
}

func TestLabelColor(t *testing.T) {
	tests := []struct {
		name string
		it   GroundItem
		want int
	}{
		{"normal", GroundItem{Quality: 2}, LabelWhite},
		{"magic", GroundItem{Quality: 4}, LabelBlue},
		{"set", GroundItem{Quality: 5}, LabelGreen},
		{"rare", GroundItem{Quality: 6}, LabelYellow},
		{"unique", GroundItem{Quality: 7}, LabelGold},
		{"crafted", GroundItem{Quality: 8}, LabelOrange},
		{"tempered", GroundItem{Quality: 9}, LabelDarkGrn},
		{"socketed normal", GroundItem{Quality: 3, Socketed: true}, LabelGrey},
		{"ethereal normal", GroundItem{Quality: 3, Ethereal: true}, LabelGrey},
		{"ethereal magic stays blue", GroundItem{Quality: 4, Ethereal: true}, LabelBlue},
		{"quest", GroundItem{Quality: 2, Quest: true, Code: "bks"}, LabelGold},
		{"leg is not a quest colour", GroundItem{Quality: 2, Quest: true, Code: "leg"}, LabelWhite},
		{"essence", GroundItem{Quality: 2, Code: "bet"}, LabelOrange},
		{"rune", GroundItem{Quality: 2, Code: "r01", IsRune: true}, LabelOrange},
	}

	for _, tc := range tests {
		if got := LabelColor(tc.it); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestLabelVisibleGolden(t *testing.T) {
	sizes := []struct{ w, h int }{{1512, 982}, {1920, 1080}}

	tests := []struct {
		x, y, mode int
		want       [2]bool // per size
	}{
		{756, 400, 0, [2]bool{true, true}},
		{-1, 400, 0, [2]bool{false, false}},
		{1600, 400, 0, [2]bool{false, true}},
		{756, 966, 0, [2]bool{true, true}},
		{756, 967, 0, [2]bool{false, true}},
		{756, -9, 0, [2]bool{false, false}},
		{756, -8, 0, [2]bool{true, true}},
		{724, 400, 1, [2]bool{true, true}},
		{725, 400, 1, [2]bool{false, true}},
		{756, 400, 2, [2]bool{true, false}},
	}

	for _, tc := range tests {
		for i, s := range sizes {
			if got := LabelVisible(tc.x, tc.y, s.w, s.h, tc.mode); got != tc.want[i] {
				t.Errorf("%dx%d (%d,%d) mode %d: got %v want %v", s.w, s.h, tc.x, tc.y, tc.mode, got, tc.want[i])
			}
		}
	}
}

func TestClampLabelGolden(t *testing.T) {
	// 1512x982: bottom limit 982-0x30 = 934, right edge 1512
	got := ClampLabel(LabelRect(2, 10, 60, 16), 0, 1512, 982)
	if got.L < 0 || got.T != 6 {
		t.Errorf("top-left clamp: %+v", got)
	}

	got = ClampLabel(LabelRect(1500, 500, 60, 16), 0, 1512, 982)
	if got.R != 1511 || got.R-got.L != 68 {
		t.Errorf("right clamp: %+v", got)
	}

	got = ClampLabel(LabelRect(700, 1000, 60, 16), 0, 1920, 1080)
	if got.B != 1000 || got.T != 988 {
		t.Errorf("1920x1080 bottom (limit 1032) must not move: %+v", got)
	}

	got = ClampLabel(LabelRect(700, 1050, 60, 16), 0, 1920, 1080)
	if got.B != 1032 {
		t.Errorf("1920x1080 bottom clamp: %+v", got)
	}
}
