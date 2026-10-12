package d2player

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
)

// The interface is laid out at 800x600 and stands in a column whose origin at 1512x982 (the default screen) is
// ((1512-800)/2, 982-600) = (356, 382) (d2display.Origin, AnchorBottom). This test turns the 800 mode rectangles of
// every panel (HUD, mini panel, character/inventory/skill tree/quest/party/waypoint/merc/gold) into screen pixels
// and compares them with testdata/ui_1512x982.golden (set UPDATE_GOLDEN=1 to rewrite it). The 800 numbers carry
// the exe verification of their own tests; this adds the column translation.
func ui1512Dump() string {
	ox, oy := d2display.Origin(d2display.Size{W: 1512, H: 982}, d2display.AnchorBottom)
	m := Mode800

	var rs []UIRect

	rs = append(rs, m.HUDRects()...)
	rs = append(rs, m.MiniPanelRects(true, 0)...)

	for _, x := range []int{m.LeftPanelX(), m.RightPanelX()} {
		for i, p := range m.PanelPictures(x) {
			p.Panel, p.Name = "panelpic", fmt.Sprintf("x%d.%d", x, i)
			rs = append(rs, p)
		}
	}

	rs = append(rs, m.QuestLayoutRects()...)

	for i := 0; i < 8; i++ {
		rs = append(rs, m.PartyRowRects(i)...)
	}

	rs = append(rs, m.PartyCloseRect(), m.PartyHeaderHover())

	for i := 0; i < wpMaxRows; i++ {
		r, _ := m.WaypointRowRects(i)
		rs = append(rs, r...)
	}

	b, h := m.WaypointClose()
	rs = append(rs, b, h)

	for class := 0; class < 7; class++ {
		for _, r := range m.SkillTreeLayoutRects(class) {
			r.Name = fmt.Sprintf("c%d.%s", class, r.Name)
			rs = append(rs, r)
		}
	}

	rs = append(rs, m.MercPanelRects()...)

	var sb strings.Builder

	for _, r := range rs {
		fmt.Fprintf(&sb, "%s %s %d %d %d %d\n", r.Panel, r.Name, r.X+ox, r.Y+oy, r.W, r.H)
	}

	for _, k := range []GoldKind{GoldStash, GoldCarried, GoldTrade} {
		g := m.Gold(k, false)
		fmt.Fprintf(&sb, "gold %d text_y %d button %d,%d\n", k, g.TextY+oy, g.ButtonX+ox, g.ButtonY+oy)
	}

	return sb.String()
}

func TestUIGolden1512x982(t *testing.T) {
	got := ui1512Dump()
	path := "testdata/ui_1512x982.golden"

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(want) != got {
		t.Errorf("1512x982 UI layout differs from %s (UPDATE_GOLDEN=1 to rewrite)", path)
	}
}

// TestUIGolden1512ColumnOrigin pins the translation itself with hand-computed numbers.
func TestUIGolden1512ColumnOrigin(t *testing.T) {
	ox, oy := d2display.Origin(d2display.Size{W: 1512, H: 982}, d2display.AnchorBottom)
	if ox != 356 || oy != 382 {
		t.Fatalf("origin %d,%d", ox, oy)
	}

	// left panels start at column x 80, y 60 -> screen 436,442; right panels at 400,60 -> 756,442
	l := Mode800.PanelPictures(Mode800.LeftPanelX())[0]
	r := Mode800.PanelPictures(Mode800.RightPanelX())[0]

	if l.X+ox != 436 || l.Y+oy != 442 || r.X+ox != 756 || r.Y+oy != 442 {
		t.Errorf("panel tops %d,%d %d,%d", l.X+ox, l.Y+oy, r.X+ox, r.Y+oy)
	}
}
