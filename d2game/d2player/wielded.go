package d2player

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
)

// WieldedCode is the item code of the weapon in the hero's hand ("" for none), as the quest code asks it: the
// Compelling Orb wants Khalim's Will "qf2" wielded (VERIFIED Game.exe 0x5b9550, INV_GetSelectedWeapon).
func (g *GameControls) WieldedCode() string {
	if g == nil || g.inventory == nil {
		return ""
	}

	it, ok := g.inventory.WornAt(d2equip.LocRightHand).(*diablo2item.Item)
	if !ok || it == nil {
		return ""
	}

	return strings.TrimSpace(it.CommonCode)
}
