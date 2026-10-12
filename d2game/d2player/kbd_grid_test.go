package d2player

import "testing"

func TestGridCursorMove(t *testing.T) {
	tests := []struct {
		name         string
		from         GridCursor
		dx, dy, w, h int
		want         GridCursor
	}{
		{"right", GridCursor{0, 0}, 1, 0, 10, 4, GridCursor{1, 0}},
		{"down", GridCursor{2, 1}, 0, 1, 10, 4, GridCursor{2, 2}},
		{"left edge stays", GridCursor{0, 2}, -1, 0, 10, 4, GridCursor{0, 2}},
		{"top edge stays", GridCursor{3, 0}, 0, -1, 10, 4, GridCursor{3, 0}},
		{"right edge stays", GridCursor{9, 1}, 1, 0, 10, 4, GridCursor{9, 1}},
		{"bottom edge stays", GridCursor{4, 3}, 0, 1, 10, 4, GridCursor{4, 3}},
		{"shrunk grid pulls the cursor in", GridCursor{9, 3}, 0, 0, 6, 3, GridCursor{5, 2}},
		{"no cells", GridCursor{3, 3}, 1, 1, 0, 0, GridCursor{}},
	}

	for _, tt := range tests {
		if got := tt.from.Move(tt.dx, tt.dy, tt.w, tt.h); got != tt.want {
			t.Errorf("%s: Move = %v want %v", tt.name, got, tt.want)
		}
	}
}

// Every size of a held item anchors on the cell the cursor shows: the click pixel that CellClickPoint returns makes
// CursorAnchor give back exactly that cell, or ok is false only where the footprint would leave the grid.
func TestCellClickPointAnchorsOnTheCell(t *testing.T) {
	const ox, oy, cell, gw, gh = 100, 200, 29, 10, 4

	for h := 1; h <= 4; h++ {
		for w := 1; w <= 2; w++ {
			for cy := 0; cy < gh; cy++ {
				for cx := 0; cx < gw; cx++ {
					mx, my, ok := CellClickPoint(ox, oy, cell, gw, gh, cx, cy, w, h)
					fits := cx+w <= gw && cy+h <= gh

					if ok != fits {
						t.Errorf("%dx%d at (%d,%d): ok=%v want %v", w, h, cx, cy, ok, fits)
						continue
					}

					if !ok {
						continue
					}

					if ax, ay, aok := CursorAnchor(mx, my, ox, oy, cell, w, h, gw, gh); !aok || ax != cx || ay != cy {
						t.Errorf("%dx%d at (%d,%d): pixel (%d,%d) anchors (%d,%d,%v)", w, h, cx, cy, mx, my, ax, ay, aok)
					}
				}
			}
		}
	}
}

// The whole keyboard sequence on a grid: move to an item, Enter picks it up, move, Enter drops it there; Enter on an
// occupied cell swaps; a footprint that leaves the grid is refused.
func TestKeyboardPickupAndDrop(t *testing.T) {
	sword := &fakeGridItem{w: 1, h: 3, x: 2, y: 0}
	ring := &fakeGridItem{w: 1, h: 1, x: 5, y: 1}
	g := newTestGrid(10, 4, sword, ring)
	cur := GridCursor{}

	// enter on the grid at a cell, with the held item's footprint
	enter := func(held InventoryItem) (InventoryItem, ClickAction) {
		w, h := 0, 0
		if held != nil {
			w, h = held.InventoryGridSize()
		}

		mx, my, ok := CellClickPoint(g.originX, g.originY, g.slotSize, g.width, g.height, cur.X, cur.Y, w, h)
		if !ok {
			mx, my, _ = CellClickPoint(g.originX, g.originY, g.slotSize, g.width, g.height, cur.X, cur.Y, 0, 0)
		}

		got, act, _, _ := g.ClickWith(held, mx, my, false)

		return got, act
	}

	// empty cell, empty cursor
	if _, act := enter(nil); act != ClickNoTarget {
		t.Fatalf("empty cell: %v", act)
	}

	// right 2, down 1 lands on the middle of the sword: Enter picks it up
	cur = cur.Move(2, 1, g.width, g.height)

	held, act := enter(nil)
	if act != ClickPickup || held != InventoryItem(sword) {
		t.Fatalf("pickup: %v %v", act, held)
	}

	// move to (7,0) and drop: the sword's top-left lands on the cursor cell
	cur = GridCursor{7, 0}

	held, act = enter(held)
	if act != ClickPlace || held != nil {
		t.Fatalf("drop: %v %v", act, held)
	}

	if x, y := sword.InventoryGridSlot(); x != 7 || y != 0 {
		t.Fatalf("sword at (%d,%d), want (7,0)", x, y)
	}

	// pick the ring up and drop it on the sword's cell: it swaps (the sword is the only item under it)
	cur = GridCursor{5, 1}

	held, act = enter(nil)
	if act != ClickPickup || held != InventoryItem(ring) {
		t.Fatalf("ring pickup: %v %v", act, held)
	}

	cur = GridCursor{7, 1}

	held, act = enter(held)
	if act != ClickSwap || held != InventoryItem(sword) {
		t.Fatalf("swap: %v %v", act, held)
	}

	// the sword (1x3) cannot go with its top-left on the last row
	cur = GridCursor{0, 3}

	held, act = enter(held)
	if act != ClickRefused || held != InventoryItem(sword) {
		t.Fatalf("overflow: %v %v", act, held)
	}
}
