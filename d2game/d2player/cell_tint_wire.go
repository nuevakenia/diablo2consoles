package d2player

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
)

// wireTints gives the item grids their cell tints and the inventory its slot highlight.
func (g *GameControls) wireTints() {
	g.inventory.slotFits = func(it InventoryItem, loc d2equip.Loc) bool { return g.evaluateEquip(it, loc).OK }
	g.inventory.grid.tint = g.gridTint
	g.inventory.grid.slotTint = g.slotTintOf

	for _, c := range []*ContainerPanel{g.stash, g.cube} {
		if c != nil && c.grid != nil {
			c.grid.tint = g.gridTint
		}
	}

	if g.Trade != nil && g.Trade.grid != nil {
		g.Trade.grid.tint = g.gridTint
	}
}

func (g *GameControls) gridTint(it InventoryItem) TintMode { return GridCellMode(g.cellStateOf(it)) }

func (g *GameControls) slotTintOf(it InventoryItem) (TintMode, bool) {
	return EquipSlotMode(g.cellStateOf(it))
}

// cellStateOf collects the facts INV_DrawGridItems asks about an item. Flag 4 and the busy
// state have no counterpart here (always false); "broken" is a durability of 0.
func (g *GameControls) cellStateOf(it InventoryItem) CellState {
	s := CellState{RequirementsMet: true, Identified: true}

	item, ok := it.(*diablo2item.Item)
	if !ok {
		return s
	}

	s.RequirementsMet = g.requirementNote(item) == ""
	s.Identified = item.IsIdentified()

	if cur, maxDur := item.Durability(); maxDur > 0 && cur <= 0 {
		s.Broken = true
	}

	return s
}
