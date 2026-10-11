package d2player

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2steal"
)

// GiveBeltItem creates a potion and puts it on the belt (a console aid for the
// scenarios); it returns the belt cell.
func (g *GameControls) GiveBeltItem(code string) (int, error) {
	item, err := g.inventory.item.NewItem(code)
	if err != nil {
		return 0, err
	}

	item.Identify()

	cell, ok := g.belt.AutoBelt(item)
	if !ok {
		return 0, fmt.Errorf("%s does not fit on the belt", code)
	}

	return cell, nil
}

// Steal runs the belt theft of Game.exe 0x5a0be0 (d2steal.Steal) against the
// belt: when the roll passes and the cursor is empty, the first occupied cell
// is emptied, the column compacted toward the front row, and the item
// returned so the caller can drop it on the ground.
func (b *BeltPanel) Steal(step func() uint32, cursorEmpty bool) (InventoryItem, bool) {
	var belt d2steal.Belt

	for i, it := range b.items {
		if it != nil {
			belt[i] = &d2steal.Item{Code: it.GetItemCode(), Mode: d2steal.ModeBelt}
		}
	}

	byPtr := map[*d2steal.Item]InventoryItem{}

	for i, it := range belt {
		if it != nil {
			byPtr[it] = b.items[i]
		}
	}

	res := d2steal.Steal(&belt, step, true, cursorEmpty)
	if res.Stolen == nil {
		return nil, false
	}

	stolen := byPtr[res.Stolen]

	for i := range b.items {
		b.items[i] = nil
		if belt[i] != nil {
			b.items[i] = byPtr[belt[i]]
		}
	}

	b.changed()

	return stolen, true
}

// StealBeltItem is the belt theft against the local hero's belt; the cursor
// must be empty.
func (g *GameControls) StealBeltItem(step func() uint32) (InventoryItem, bool) {
	return g.belt.Steal(step, g.inventory.CursorItem() == nil)
}

// BeltCodes lists the item codes in the 16 belt cells ("" for empty).
func (g *GameControls) BeltCodes() [16]string {
	var out [16]string

	for i := range out {
		if it := g.belt.Get(i); it != nil {
			out[i] = it.GetItemCode()
		}
	}

	return out
}
