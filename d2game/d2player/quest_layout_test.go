package d2player

import (
	"strings"
	"testing"
)

func TestQuestLayoutGolden(t *testing.T) {
	want := []string{
		"quest tab0 85 62 63 31",
		"quest tab4 333 62 63 31",
		"quest slot0 106 86 80 95",
		"quest slot2 300 86 80 95",
		"quest slot3 106 183 80 95",
		"quest slot5 300 183 80 95",
		"quest close 358 450 32 32",
		"quest close_hover 358 452 36 34",
		"quest descr_button 306 450 32 32",
		"quest descr_hover 310 451 30 33",
		"quest name_bottom 80 308 0 0",
		"quest descr_bottom0 80 330 270 20",
	}

	got := map[string]bool{}
	for _, r := range Mode800.QuestLayoutRects() {
		got[r.String()] = true
	}

	for _, w := range want {
		if !got[w] {
			t.Errorf("missing %q", w)
		}
	}
}

func TestQuestDescriptionTextPixelWrap(t *testing.T) {
	measure := func(s string) int { return 10 * len(s) } // 27 glyphs fit in 270 px
	got := QuestDescriptionText("aaaaaaaaa bbbbbbbbb ccccccccc ddddddddd", measure)
	want := "aaaaaaaaa bbbbbbbbb\nccccccccc ddddddddd"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	if got := QuestDescriptionText(strings.Repeat("x", 27), measure); strings.Contains(got, "\n") {
		t.Fatalf("27 glyphs = 270 px must fit: %q", got)
	}
}
