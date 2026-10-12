package d2player

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// SetBeltForTest wears the belt with this base code ("none" or "" takes it off) and empties the potion belt, so
// a scenario can start every case from a clean belt of a known size (scripts only). It returns the boxes.
func (g *GameControls) SetBeltForTest(code string) (int, error) {
	if code == "" || code == "none" {
		g.inventory.grid.ChangeEquippedSlot(d2enum.EquippedSlotBelt, nil)
	} else {
		item, err := g.inventory.item.NewItem(code)
		if err != nil {
			return 0, err
		}

		item.Identify()
		g.inventory.grid.Load(item)
		g.inventory.grid.ChangeEquippedSlot(d2enum.EquippedSlotBelt, item)
	}

	for i := range g.belt.items {
		g.belt.items[i] = nil
	}

	g.belt.SetExpanded(false)

	return g.belt.Boxes(), nil
}

// BeltHoverForTest moves the pointer over a point of the 800x600 column as a mouse move does, so the belt tooltip
// shows in a screenshot (scripts only). It returns the cell under the point (-1 for none).
func (g *GameControls) BeltHoverForTest(ux, uy int) int {
	g.belt.OnMouseMove(ux, uy)

	return g.belt.hoverCell
}

// BeltBoxesNow is the usable cell count of the belt for a scenario log line.
func (g *GameControls) BeltBoxesNow() string {
	return fmt.Sprintf("boxes=%d rows=%d expanded=%v", g.belt.Boxes(), g.belt.Rows(), g.belt.Expanded())
}
