package d2player

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

// IsShiftBeltClick is the condition of the shift branch of INV_HandleGridClick (0x48c3c0): a left click
// with the shift flag (flag 4) on a beltable item of the inventory page, with an empty cursor and the
// control key up, sends the item to the belt instead of picking it up.
func IsShiftBeltClick(mod d2enum.KeyMod, cursorEmpty, beltable bool) bool {
	return mod == d2enum.KeyModShift && cursorEmpty && beltable
}

// shiftClickToBelt moves the clicked inventory item to the belt (shift + left click). The exe plays sound
// 0xeb for it (not reproduced). The belt slot search is PlaceBeltable (belt_exe.go; its first-empty-cell
// fallback needs the autobelt column, VERIFIED). It reports whether the click was consumed.
func (g *GameControls) shiftClickToBelt(mod d2enum.KeyMod, mx, my int) bool {
	if !g.inventory.IsOpen() || !g.inventory.grid.Contains(mx, my) {
		return false
	}

	item := g.inventory.grid.ItemAtScreen(mx, my)
	if item == nil || !IsShiftBeltClick(mod, g.inventory.CursorItem() == nil, g.belt.Beltable(item)) {
		return false
	}

	if _, ok := g.belt.PlaceBeltable(item); !ok {
		return false // no room: the click falls through and picks the potion up as usual
	}

	g.inventory.grid.Remove(item)
	g.saveHero()
	g.Infof("shift-click: %s moved to the belt", item.GetItemCode())

	return true
}
