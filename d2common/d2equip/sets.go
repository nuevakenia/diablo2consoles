package d2equip

// Weapon sets. A character has two weapon sets: the one in the hands lives in LocRightHand / LocLeftHand (4, 5), the
// other in LocSwapRight / LocSwapLeft (11, 12). VERIFIED (w37, the swap in ITEMMODE_ValidateEquippedItems 0x55f4c0 and the
// save writer SAVE_WriteItemListJM 0x52f220): a weapon swap physically exchanges the items of 4/5 with those of 11/12
// (ITEM_SetBodyLocation) and toggles a client byte (+0x45c); the header field at 0x10 of the .d2s ("active_arms", bit 0
// of SAVE_WriteD2sHeader 0x566c70) only records that byte, which tells the interface whether the hands hold "set II".
// Items are written with the body location they have, so the weapons in the hands are ALWAYS at 4/5 in a file.
// (Earlier versions mapped 4/5 to 11/12 when the field was 1; that was wrong.)

// SetCount is the number of weapon sets.
const SetCount = 2

// EffectiveLoc maps a physical location to the one the game treats the item as being at. It is the identity: the
// active set is always the one at 4/5 (see above); the second argument is the header field, kept for the callers.
func EffectiveLoc(loc Loc, _ int) Loc { return loc }

// NextSet is the active set after a swap.
func NextSet(activeArms int) int { return (activeArms + 1) % SetCount }

// HandsOf returns the right and left hand items from a body: always the items at 4/5 (the header field does not move them).
func HandsOf(body map[Loc]*Item, _ int) (right, left *Item) {
	return body[LocRightHand], body[LocLeftHand]
}
