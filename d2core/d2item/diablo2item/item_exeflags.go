package diablo2item

// Item flag bits of pItemData+0x18 that the equip tests read. ITEM_IsNotBrokenAndEquippable (0x62a650, VERIFIED w39)
// fails an item with either bit set; ITEM_BreakEquippedItem (0x55d660) sets 0x100 when a worn item runs out of
// durability and ITEM_RestoreBrokenItem (0x55d710) clears it (VERIFIED w27).
const (
	ExeFlagBroken        uint32 = 0x100
	ExeFlagNotEquippable uint32 = 0x4000
)

// ExeFlags is the item flag word as far as the engine models it: the bits a save carried, then the break and
// restore events applied to them by SetDurability. The other bits are not modelled.
func (i *Item) ExeFlags() uint32 {
	var f uint32
	if i.origin != nil {
		f = i.origin.Flags
	}

	return (f | i.flagSet) &^ i.flagClear
}

// HasBrokenFlag is the negation of ITEM_IsNotBrokenAndEquippable: flag 0x100 or 0x4000 is set.
func (i *Item) HasBrokenFlag() bool { return i.ExeFlags()&(ExeFlagBroken|ExeFlagNotEquippable) != 0 }

// noteDurability applies the break (durability gone) and restore (durability back above 0) rules to flag 0x100.
func (i *Item) noteDurability(n int) {
	if !i.attributes.durable {
		return
	}

	if n <= 0 {
		i.flagSet |= ExeFlagBroken
		i.flagClear &^= ExeFlagBroken

		return
	}

	i.flagSet &^= ExeFlagBroken
	i.flagClear |= ExeFlagBroken
}
