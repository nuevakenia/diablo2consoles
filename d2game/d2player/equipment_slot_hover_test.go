package d2player

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// The mouse over an empty equipment slot must not panic (the slot has no item, so there is no tooltip).
func TestHoverEmptyEquipmentSlotDoesNotPanic(t *testing.T) {
	g := &Inventory{grid: &ItemGrid{equipmentSlots: map[d2enum.EquippedSlot]EquipmentSlot{
		d2enum.EquippedSlotHead: {x: 10, y: 100, width: 50, height: 50},
	}}}
	g.lastMouseX, g.lastMouseY = 30, 80

	if !g.checkEquippedSlotsHover() {
		t.Fatal("the mouse is over the slot")
	}
}
