package d2player

import "testing"

// Golden numbers: Inventory.txt rows "Big Bank Page2", "Bank Page2", "Transmogrify Box2" (800x600 mode; panel left 80,
// pictures drawn at y = panelArtTop). The grid rectangles are numbers only. The grid recess of each picture (measured
// on the pictures: TradeStash.dc6 6x8 at (74,82)-(244,313); bank.dc6 6x4 at (74,273)-(244,386); supertransmogrifier.dc6
// 3x4 at (118,139)-(205,253)) is the record rectangle minus (80, 60).
var containerRecords = map[string]containerLayout{
	"Big Bank Page2":    {Cols: 6, Rows: 8, Left: 154, Top: 142, Cell: 29, PanelLeft: 80},
	"Bank Page2":        {Cols: 6, Rows: 4, Left: 154, Top: 333, Cell: 29, PanelLeft: 80},
	"Transmogrify Box2": {Cols: 3, Rows: 4, Left: 198, Top: 199, Cell: 29, PanelLeft: 80},
}

func lookupRecord(name string) (containerLayout, bool) {
	l, ok := containerRecords[name]

	return l, ok
}

func TestContainerLayoutExpansionStash(t *testing.T) {
	l := resolveContainerLayout(stashKind, lookupRecord, func(string) bool { return true })

	if l.Art != "/data/global/ui/PANEL/TradeStash.DC6" || l.Cols != 6 || l.Rows != 8 || l.Left != 154 || l.Top != 142 {
		t.Fatalf("expansion stash: %+v", l)
	}

	// grid rectangle on screen (column space), and on the picture
	if l.Left+l.Cols*l.Cell != 328 || l.Top+l.Rows*l.Cell != 374 {
		t.Errorf("grid end (%d,%d)", l.Left+l.Cols*l.Cell, l.Top+l.Rows*l.Cell)
	}

	if l.Left-l.PanelLeft != 74 || l.Top-panelArtTop != 82 {
		t.Errorf("grid on the picture at (%d,%d), want (74,82)", l.Left-l.PanelLeft, l.Top-panelArtTop)
	}
}

func TestContainerLayoutClassicStash(t *testing.T) {
	// no expansion picture: the classic record goes with bank.dc6, never the 6x8 grid on the 6x4 picture
	l := resolveContainerLayout(stashKind, lookupRecord, func(string) bool { return false })

	if l.Art != "/data/global/ui/PANEL/bank.DC6" || l.Cols != 6 || l.Rows != 4 || l.Top-panelArtTop != 273 {
		t.Fatalf("classic stash: %+v", l)
	}

	// no records at all: the built-in fallback is the classic pair as well
	l = resolveContainerLayout(stashKind, func(string) (containerLayout, bool) { return containerLayout{}, false },
		func(string) bool { return true })

	if l.Art != "/data/global/ui/PANEL/bank.DC6" || l.Cols != 6 || l.Rows != 4 || l.Left != 154 || l.Top != 333 {
		t.Fatalf("fallback stash: %+v", l)
	}
}

func TestContainerLayoutCube(t *testing.T) {
	l := resolveContainerLayout(cubeKind, lookupRecord, func(string) bool { return true })

	if l.Art != "/data/global/ui/PANEL/supertransmogrifier.dc6" || l.Cols != 3 || l.Rows != 4 ||
		l.Left-l.PanelLeft != 118 || l.Top-panelArtTop != 139 {
		t.Fatalf("cube: %+v", l)
	}

	// Transmute button sits under the grid: bottom = grid bottom + gap + height
	if bottom := l.Top + l.Rows*l.Cell; bottom != 315 {
		t.Errorf("cube grid bottom %d", bottom)
	}
}

// Items of 1x1 .. 2x4 cells stay inside the grid rectangle at every cell.
func TestContainerGridItemRects(t *testing.T) {
	l := resolveContainerLayout(stashKind, lookupRecord, func(string) bool { return true })
	g := newPlainItemGrid(nil, nil, 0, l.Cols, l.Rows, l.Left, l.Top, l.Cell)
	gl, gt, gr, gb := g.Bounds()

	for _, sz := range [][2]int{{1, 1}, {1, 2}, {2, 2}, {1, 3}, {2, 3}, {2, 4}} {
		for y := 0; y+sz[1] <= l.Rows; y++ {
			for x := 0; x+sz[0] <= l.Cols; x++ {
				sx, sy := g.SlotToScreen(x, y)
				right, bottom := sx+sz[0]*l.Cell, sy+sz[1]*l.Cell

				if sx < gl || sy < gt || right > gr || bottom > gb {
					t.Fatalf("item %dx%d at (%d,%d) leaves the grid %d,%d,%d,%d", sz[0], sz[1], x, y, gl, gt, gr, gb)
				}

				if cx, cy := g.ScreenToSlot(sx+1, sy+1); cx != x || cy != y {
					t.Fatalf("hit test of (%d,%d) gives (%d,%d)", x, y, cx, cy)
				}
			}
		}
	}
}
