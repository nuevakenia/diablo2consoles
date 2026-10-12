package d2player

// itemSoundHook plays a Sounds.txt handle for an item the hero moves between
// the cursor and a grid. The game screen sets it (it owns the sound engine).
var itemSoundHook func(handle, kind string)

// SetItemSoundHook sets the function that plays item handling sounds.
func SetItemSoundHook(fn func(handle, kind string)) { itemSoundHook = fn }

// itemPickupSound is the sound of an item entering the cursor: ITEM_InitClientItem
// (0x4be1f0) plays Sounds.txt row 0xeb (item_pickup) for every item that gets
// item mode 4, wherever it came from (VERIFIED, w29). The original does not use
// the item's own dropsound for a pickup.
func itemPickupSound() {
	if itemSoundHook != nil {
		itemSoundHook("item_pickup", "item-pickup")
	}
}

// itemSound plays the sound of an item being put down: the item's own drop
// sound (ITEM_GetDropSoundId 0x4be630: unique/set override of the base row,
// played by INV_HandleGridClick 0x48c3c0 and the equip slot handlers).
func itemSound(it InventoryItem, kind string) {
	if itemSoundHook == nil || it == nil {
		return
	}

	if s, ok := it.(interface{ DropSoundHandle() string }); ok {
		if h := s.DropSoundHandle(); h != "" {
			itemSoundHook(h, kind)
		}
	}
}
