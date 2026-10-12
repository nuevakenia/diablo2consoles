package d2player

import (
	"fmt"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// Golden of the skill tree layout (800x600): close buttons per class and tab, icons, points anchor.
func TestSkillTreeCloseGolden(t *testing.T) {
	want := [7][3]int{
		{571, 500, 415}, {415, 415, 571}, {415, 571, 415}, {415, 500, 415},
		{571, 415, 571}, {571, 571, 571}, {500, 571, 415},
	}

	for c := range want {
		for tab := range want[c] {
			if got := Mode800.SkillTreeCloseX(c, tab); got != want[c][tab] {
				t.Errorf("class %d tab %d: got %d want %d", c, tab, got, want[c][tab])
			}
		}
	}

	// the Go constants (with the panel shift) stay equal to the exe numbers
	if Mode800.SkillTreeCloseX(0, 0) != skillCloseButtonXRight || Mode800.SkillTreeCloseX(0, 1) != skillCloseButtonXMiddle ||
		Mode800.SkillTreeCloseX(0, 2) != skillCloseButtonXLeft {
		t.Fatal("close constants drifted from the exe table")
	}

	if b := Mode800.SkillTreeCloseBottom(); b != 477 || b-32 != skillCloseButtonY {
		t.Fatalf("close bottom %d", b)
	}

	if Mode640.SkillTreeCloseX(0, 0) != 640-0x95 {
		t.Fatalf("640 close x %d", Mode640.SkillTreeCloseX(0, 0))
	}
}

func TestSkillTreeIconsMatchGo(t *testing.T) {
	for row := 1; row <= 6; row++ {
		for col := 1; col <= 3; col++ {
			x, y, w, h := Mode800.SkillTreeIconRect(row, col)
			gx, gy := skillIconPosition(row, col)

			if x != gx || y+h != gy || w != 48 {
				t.Errorf("row %d col %d: exe %d,%d go %d,%d", row, col, x, y+h, gx, gy)
			}
		}
	}
}

func TestSkillTreeTitlesGolden(t *testing.T) {
	x1, x2 := Mode800.SkillTreeTitleBox()
	if x1 != 630 || x2 != 720 {
		t.Fatalf("title box %d..%d", x1, x2)
	}

	px, py, zx := Mode800.SkillTreePoints()
	if px != 668 || py != 140 || zx != 695 {
		t.Fatalf("points %d %d %d", px, py, zx)
	}

	// Amazon: 3 common lines, then three groups of 3 lines inside the three tab spans 168..276, 276..384, 384..492
	var ys []int
	for _, l := range Mode800.SkillTreeTitles(0) {
		ys = append(ys, l.Y)
	}

	if got, want := fmt.Sprint(ys), "[85 97 109 210 222 234 318 330 342 419 431 443]"; got != want {
		t.Fatalf("amazon title ys %s want %s", got, want)
	}

	// every class keeps its class lines inside the three tab spans (the common lines are above)
	for c := 0; c < 7; c++ {
		for _, l := range Mode800.SkillTreeTitles(c)[3:] {
			if l.Y < 168 || l.Y > 492 {
				t.Errorf("class %d line 0x%x outside the tab spans: y %d", c, l.StrID, l.Y)
			}
		}
	}

	// Paladin string ids in drawing order
	var ids []int
	for _, l := range Mode800.SkillTreeTitles(3)[3:] {
		ids = append(ids, l.StrID)
	}

	if got := fmt.Sprintf("%x", ids); got != "[108e 108f 1090 108f 1091 1086]" {
		t.Fatalf("paladin ids %s", got)
	}

	if SkillTreeClassIndex(d2enum.HeroDruid) != 5 || SkillTreeClassIndex(d2enum.HeroNone) != -1 {
		t.Fatal("class index")
	}

	if len(Mode800.SkillTreeLayoutRects(0)) != 7 {
		t.Fatal("rect count")
	}
}
