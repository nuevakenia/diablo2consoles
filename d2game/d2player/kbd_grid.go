package d2player

import (
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// Keyboard-only item moving (w42). While the inventory, the stash, the cube or a vendor window is open the arrow keys
// move a cell cursor over the grid under it, Enter or Space does what a left click on that cell does (pick the item up
// with an empty cursor, drop / swap / merge the held item, buy from a vendor), Ctrl+Enter auto-places like Ctrl+click,
// and PageUp / PageDown (or Home / End) hop to the previous / next open grid. Arrow keys at the edge of a grid stay put.
// Enter is also the chat key, so it is taken only while the cell cursor is showing (after the first arrow key).
//
// It is the same path as a mouse click: Enter sends a left click at the pixel of the cell to the mouse handlers
// (OnMouseButtonDown), so the grids apply INV_HandleGridClick (0x48c3c0) and the trade window its buy rule unchanged.

// GridCursor is a cell position on a grid.
type GridCursor struct{ X, Y int }

// Move steps the cursor by (dx, dy) and keeps it inside a w x h grid (no wrap: the edge stays the edge). A grid with
// no cells leaves the cursor at 0,0.
func (c GridCursor) Move(dx, dy, w, h int) GridCursor {
	if w < 1 || h < 1 {
		return GridCursor{}
	}

	c.X = clampInt(c.X+dx, 0, w-1)
	c.Y = clampInt(c.Y+dy, 0, h-1)

	return c
}

func clampInt(v, lo, hi int) int {
	switch {
	case v < lo:
		return lo
	case v > hi:
		return hi
	}

	return v
}

// CellClickPoint finds a pixel whose left click on the grid acts on the cell (cx, cy). With an empty cursor
// (held == nil) that is the centre of the cell. With a held w x h item the grid anchors the item's footprint on the
// click (CursorAnchor), so the pixel is searched among the cell centres around the cell for the one whose anchor is
// exactly (cx, cy), the top-left cell of the footprint. ok is false when the item cannot be anchored there (the
// footprint would leave the grid).
func CellClickPoint(originX, originY, cell, gridW, gridH, cx, cy, w, h int) (mx, my int, ok bool) {
	centre := func(x, y int) (int, int) { return originX + x*cell + cell/2, originY + y*cell + cell/2 }

	if w < 1 || h < 1 {
		mx, my = centre(cx, cy)

		return mx, my, cx >= 0 && cy >= 0 && cx < gridW && cy < gridH
	}

	for y := cy; y <= cy+h; y++ {
		for x := cx; x <= cx+w; x++ {
			px, py := centre(x, y)

			if ax, ay, okAnchor := CursorAnchor(px, py, originX, originY, cell, w, h, gridW, gridH); okAnchor && ax == cx && ay == cy {
				return px, py, true
			}
		}
	}

	return 0, 0, false
}

// kbdGrid is one grid the cell cursor can sit on.
type kbdGrid struct {
	name string
	grid *ItemGrid
}

// kbdState is the keyboard cell cursor of GameControls.
type kbdState struct {
	active bool
	panel  string // name of the grid the cursor is on
	cells  map[string]GridCursor
}

// kbdGrids lists the open grids, in the order PageDown visits them.
func (g *GameControls) kbdGrids() []kbdGrid {
	var out []kbdGrid

	if g.Trade != nil && g.Trade.IsOpen() && g.Trade.grid != nil {
		out = append(out, kbdGrid{"trade", g.Trade.grid})
	}

	if g.stash != nil && g.stash.IsOpen() {
		out = append(out, kbdGrid{"stash", g.stash.grid})
	}

	if g.cube != nil && g.cube.IsOpen() {
		out = append(out, kbdGrid{"cube", g.cube.grid})
	}

	if g.inventory != nil && g.inventory.IsOpen() {
		out = append(out, kbdGrid{"inventory", g.inventory.grid})
	}

	return out
}

// KbdCell reports where the keyboard cell cursor is: the grid name and the cell; panel is "" when it is not showing.
func (g *GameControls) KbdCell() (panel string, x, y int) {
	if !g.kbd.active {
		return "", 0, 0
	}

	c := g.kbd.cells[g.kbd.panel]

	return g.kbd.panel, c.X, c.Y
}

// kbdCurrent returns the grid the cursor is on (the first open one when the remembered one closed).
func (g *GameControls) kbdCurrent(grids []kbdGrid) (kbdGrid, int) {
	for i, kg := range grids {
		if kg.name == g.kbd.panel {
			return kg, i
		}
	}

	g.kbd.panel = grids[0].name

	return grids[0], 0
}

// kbdPoint is the pixel of the cursor cell, adjusted for the held item.
func (g *GameControls) kbdPoint(kg kbdGrid) (mx, my int) {
	c := g.kbd.cells[kg.name]
	w, h := 0, 0

	if held := g.inventory.CursorItem(); held != nil {
		w, h = held.InventoryGridSize()
	}

	mx, my, ok := CellClickPoint(kg.grid.originX, kg.grid.originY, kg.grid.slotSize, kg.grid.width, kg.grid.height, c.X, c.Y, w, h)
	if !ok { // the footprint does not fit there: aim at the cell anyway (the click is refused like a mouse click)
		mx, my, _ = CellClickPoint(kg.grid.originX, kg.grid.originY, kg.grid.slotSize, kg.grid.width, kg.grid.height, c.X, c.Y, 0, 0)
	}

	return mx, my
}

// gridKeyNav handles the keys of the cell cursor; it reports whether the key was taken.
func (g *GameControls) gridKeyNav(event d2interface.KeyEvent) bool {
	grids := g.kbdGrids()
	if len(grids) == 0 || g.escapeMenu.IsOpen() || g.NPCMenu.IsOpen() || g.Identify.IsOpen() {
		g.kbd.active = false

		return false
	}

	dx, dy, hop := 0, 0, 0
	click := false

	switch event.Key() {
	case d2enum.KeyLeft:
		dx = -1
	case d2enum.KeyRight:
		dx = 1
	case d2enum.KeyUp:
		dy = -1
	case d2enum.KeyDown:
		dy = 1
	case d2enum.KeyPageUp, d2enum.KeyHome:
		hop = -1
	case d2enum.KeyPageDown, d2enum.KeyEnd:
		hop = 1
	case d2enum.KeyEnter, d2enum.KeyKPEnter, d2enum.KeySpace:
		if !g.kbd.active {
			return false // Enter stays the chat key until the cursor is showing
		}

		click = true
	default:
		return false
	}

	if g.kbd.cells == nil {
		g.kbd.cells = map[string]GridCursor{}
	}

	kg, idx := g.kbdCurrent(grids)

	if !g.kbd.active {
		g.kbd.active = true // the first arrow key only shows the cursor where it was
		g.logKbd("show", kg)
		g.kbdMoveMouse(kg)

		return true
	}

	switch {
	case hop != 0:
		kg = grids[(idx+hop+len(grids))%len(grids)]
		g.kbd.panel = kg.name
		g.logKbd("hop", kg)
	case click:
		g.kbdClick(kg, event.KeyMod())

		return true
	default:
		g.kbd.cells[kg.name] = g.kbd.cells[kg.name].Move(dx, dy, kg.grid.width, kg.grid.height)
		g.logKbd("move", kg)
	}

	g.kbdMoveMouse(kg)

	return true
}

func (g *GameControls) logKbd(what string, kg kbdGrid) {
	c := g.kbd.cells[kg.name]
	g.Infof("KBDGRID %s panel=%s cell=(%d,%d) cursor_item=%t", what, kg.name, c.X, c.Y, g.inventory.CursorItem() != nil)
}

// kbdFind (test helper, console command kbdfind) puts the cursor on the top-left cell of the first item of the grid it
// is on, and logs it (KBDGRID find ...).
func (g *GameControls) kbdFind() {
	grids := g.kbdGrids()
	if len(grids) == 0 || !g.kbd.active {
		g.Infof("KBDGRID find: no cell cursor showing")
		return
	}

	kg, _ := g.kbdCurrent(grids)

	items := kg.grid.Items()
	if len(items) == 0 {
		g.Infof("KBDGRID find panel=%s: no items", kg.name)
		return
	}

	x, y := items[0].InventoryGridSlot()
	g.kbd.cells[kg.name] = GridCursor{x, y}
	g.logKbd("find", kg)
	g.kbdMoveMouse(kg)
}

// kbdMoveMouse makes the pointer logic see the cursor cell: the held item is drawn there and the tooltip follows.
func (g *GameControls) kbdMoveMouse(kg kbdGrid) {
	mx, my := g.kbdPoint(kg)
	g.OnMouseMove(&synthEvent{x: mx, y: my})
}

// kbdClick is Enter on the cursor cell: a left click through the mouse handlers.
func (g *GameControls) kbdClick(kg kbdGrid, mod d2enum.KeyMod) {
	mx, my := g.kbdPoint(kg)
	c := g.kbd.cells[kg.name]
	before := g.inventory.CursorItem()

	g.OnMouseMove(&synthEvent{x: mx, y: my})
	g.OnMouseButtonDown(&synthEvent{button: d2enum.MouseButtonLeft, mod: mod, x: mx, y: my})
	g.OnMouseButtonUp(&synthEvent{button: d2enum.MouseButtonLeft, mod: mod, x: mx, y: my})

	after := g.inventory.CursorItem()
	code := func(it InventoryItem) string {
		if it == nil {
			return "-"
		}

		return it.GetItemCode()
	}

	act := ClickNone

	switch kg.name {
	case "inventory":
		act = g.inventory.lastClick
	case "stash":
		act = g.stash.lastClick
	case "cube":
		act = g.cube.lastClick
	}

	held := 0
	if after != nil {
		held = 1
	}

	g.Infof("KBDGRID enter panel=%s cell=(%d,%d) cursor_before=%s cursor_after=%s act=%s inv_items=%d stash_items=%d total=%d",
		kg.name, c.X, c.Y, code(before), code(after), act, len(g.inventory.grid.items), len(g.stash.grid.items),
		len(g.inventory.grid.items)+len(g.stash.grid.items)+held)
}

// renderKbdCursor draws the cell cursor: the cell, or the footprint of the held item anchored on it.
func (g *GameControls) renderKbdCursor(target d2interface.Surface) {
	if !g.kbd.active {
		return
	}

	grids := g.kbdGrids()
	if len(grids) == 0 {
		g.kbd.active = false // every panel closed: the next Enter is the chat key again

		return
	}

	kg, _ := g.kbdCurrent(grids)
	c := g.kbd.cells[kg.name]
	w, h := 1, 1

	if held := g.inventory.CursorItem(); held != nil {
		w, h = held.InventoryGridSize()
	}

	x, y := kg.grid.SlotToScreen(c.X, c.Y)

	target.PushTranslation(x, y)
	target.DrawRect(w*kg.grid.slotSize, h*kg.grid.slotSize, color.RGBA{R: 255, G: 220, B: 80, A: 70})
	target.Pop()
}
