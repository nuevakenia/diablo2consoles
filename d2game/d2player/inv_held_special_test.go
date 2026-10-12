package d2player

import "testing"

// itemsUnder lists the distinct items of a footprint in the scan order of INV_GetSingleOverlappedItem (0x63cc50):
// rows from the top, cells from the left, so the first cube met wins when two or more items are covered.
func TestItemsUnderScanOrder(t *testing.T) {
	a := newStack("a", 1, 5) // 1x1 at (1,0)
	a.x, a.y = 1, 0
	b := newStack("b", 1, 5) // 1x1 at (0,1)
	b.x, b.y = 0, 1
	big := &stackItem{fakeGridItem: fakeGridItem{w: 2, h: 2}, code: "big"} // covers (2,0)..(3,1)
	big.x, big.y = 2, 0

	g := newTestGrid(10, 4)
	g.items = []InventoryItem{b, big, a}

	got := g.itemsUnder(0, 0, 4, 2)
	if len(got) != 3 || got[0] != a || got[1] != big || got[2] != b {
		t.Fatalf("scan order: %v", got)
	}

	if n := len(g.itemsUnder(0, 2, 4, 2)); n != 0 {
		t.Fatalf("empty rows: %d", n)
	}
}

func TestStackProbeOfPlainItem(t *testing.T) {
	p := stackProbe(newStack("arrow", 3, 20))
	if p.Code != "arrow" || !p.Stackable || p.Quality != 2 {
		t.Fatalf("%+v", p)
	}
}
