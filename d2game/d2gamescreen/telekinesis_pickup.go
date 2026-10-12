package d2gamescreen

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"

// telekinesisPickup is Telekinesis taking a ground item from afar (SRVDO_Telekinesis 0x5c79f0, item branch, w37):
// the item goes into the inventory without the hero walking to it. It uses the normal pickup path (gold into the
// purse, an item to the cursor and, with quick pickup, into the inventory). UNVERIFIED: the exe's own pickup
// (ITEMMODE_ServerPickupGroundItemToInventory) never uses the cursor.
func (v *Game) telekinesisPickup(_ *d2mapentity.Player, itemID string) {
	if v.gameClient == nil || v.gameClient.MapEngine == nil || v.gameControls == nil {
		return
	}

	it, ok := v.gameClient.MapEngine.Entities()[itemID].(*d2mapentity.Item)
	if !ok || it == nil {
		return
	}

	v.pickUp(it)
}
