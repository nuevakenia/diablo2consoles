package d2player

import (
	"fmt"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
)

// WieldFromPack is the scripts' way of putting an item into the weapon hand (command "wield <code>"): it takes
// the first item with this base code from the inventory or the Horadric Cube onto the cursor and equips it in
// the right hand through the same rules a click uses (TryEquipCursor). The Compelling Orb needs Khalim's Will
// wielded (VERIFIED Game.exe 0x5b9550), so the Act 3 scenarios use it.
func (g *GameControls) WieldFromPack(code string) error {
	if g.inventory.CursorItem() != nil {
		return fmt.Errorf("the cursor is busy")
	}

	for _, grid := range []*ItemGrid{g.inventory.grid, g.cube.grid} {
		for _, it := range grid.Items() {
			d, ok := it.(*diablo2item.Item)
			if !ok || !strings.EqualFold(strings.TrimSpace(d.CommonCode), code) {
				continue
			}

			grid.Remove(it)
			g.inventory.SetCursorItem(it)

			if res := g.TryEquipCursor(d2equip.LocRightHand); !res.OK {
				g.inventory.SetCursorItem(nil)
				g.inventory.grid.AutoPlace(it, true)

				return fmt.Errorf("cannot wield %s: %s %s", code, res.Reason, res.Detail)
			}

			if left := g.inventory.CursorItem(); left != nil {
				g.inventory.SetCursorItem(nil)
				g.inventory.grid.AutoPlace(left, true)
			}

			return nil
		}
	}

	return fmt.Errorf("no %s in the inventory or the cube", code)
}
