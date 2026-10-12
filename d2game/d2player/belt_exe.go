package d2player

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
)

// PlaceBeltable puts a beltable item on the belt the way INV_FindBeltSlotForItem

// it below; only when no column does, the item goes to the first empty cell of the
// front row, and only if its misc.txt autobelt column is set (the byte that the
// earlier notes called flag +0x131). A beltable item without autobelt (stamina,
// antidote, thawing) therefore needs a matching column. Used by the shift-click
// (INV_HandleGridClick 0x48c3c0 shift branch); it reports the cell.
func (b *BeltPanel) PlaceBeltable(item InventoryItem) (cell int, ok bool) {
	it, isItem := item.(*diablo2item.Item)
	if !isItem || !b.Beltable(item) {
		return 0, false
	}

	cell, ok = d2inventory.FindBeltSlotOpt(b.kinds(), b.Boxes(), b.kindOf(item), it.CommonRecord().AutoBelt)
	if !ok {
		return 0, false
	}

	if err := b.Set(cell, item); err != nil {
		return 0, false
	}

	b.changed()

	return cell, true
}
