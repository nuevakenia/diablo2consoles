package d2trade

// Vendor transaction rules of Game.exe 1.14b (TRADE_ServerSellItem 0x577320,
// TRADE_ServerRepairItem 0x575e80, TRADE_DeductGold 0x574bc0). See
// d2-re-notes/w23-economy.md.

// RepairClassIDs are the npc classes whose repair packet the server accepts
// (Charsi 154, Fara 178, Hratli 253, Halbu 257, Larzuk 511). VERIFIED.
var RepairClassIDs = [...]int{154, 178, 253, 257, 511}

// CanRepairAt reports whether a vendor class repairs.
func CanRepairAt(classID int) bool {
	for _, id := range RepairClassIDs {
		if id == classID {
			return true
		}
	}

	return false
}

// SellInfo is what TRADE_CheckItemSellable reads.
type SellInfo struct {
	NoSellFlag bool // item flag 0x1000
	Quest      bool // base record "quest" column non-zero
	QuestType  bool // item type 0x27 (ItemTypes "ques")
}

// Sellable is TRADE_CheckItemSellable (0x62a2c0 with ITEM_IsBaseItemSellable
// 0x629f00): the vendor refuses flagged items, quest items and quest types.
func Sellable(s SellInfo) bool {
	return !s.NoSellFlag && !s.Quest && !s.QuestType
}

// ResaleInfo are the conditions under which a sold item joins the vendor's
// stock (the clone placed into the vendor inventory by 0x577320).
type ResaleInfo struct {
	Cracked      bool // low quality item with the "Cracked" LowQualityItems name
	Broken       bool // item flag 0x100 (durability ran out)
	PlayerPart   bool // item type 7 (player ear)
	Personalized bool // flag 0x1000000
	Ethereal     bool // flag 0x400000
	HasContents  bool // something is socketed into it (non-empty item inventory)
	SingleCopy   bool // unique row with bit 4 of the row flags (carry1)
	GambleMode   bool // the player-vendor record's gamble flag
	VendorSells  bool // TRADE_CheckVendorSellsItem: a permanent stock code
}

// Resellable is true when the sold item is cloned into the vendor stock.
// Everything else is destroyed and only paid.
func Resellable(r ResaleInfo) bool {
	return !(r.Cracked || r.Broken || r.PlayerPart || r.Personalized || r.Ethereal ||
		r.HasContents || r.SingleCopy || r.GambleMode || r.VendorSells)
}

// ResalePaid is the amount paid for a resold item: the original sell price,
// or the sell price of the refilled clone when that is lower.
func ResalePaid(original, refilled int) int {
	if refilled < original {
		return refilled
	}

	return original
}

// DeductGold is TRADE_DeductGold: the inventory gold pays first, the rest
// comes from the stash. It fails (and changes nothing) when both together are
// short.
func DeductGold(inv, stash, cost int) (newInv, newStash int, ok bool) {
	if inv+stash < cost {
		return inv, stash, false
	}

	if cost <= inv {
		return inv - cost, stash, true
	}

	return 0, stash + inv - cost, true
}

// PartialRepair is the fall-back of the single item repair when the full price
// is not affordable (0x575e80, the item must not be broken): with gold > 0 and
// a price per durability point (price/(max-cur), kept in 1/1024) below the
// gold the player has, ALL inventory gold is spent and the item gets
// gold*1024/rate points, at most up to its maximum. ok is false when not even
// one point is affordable (nothing changes then).
func PartialRepair(price, cur, max, gold int) (newDur, spent int, ok bool) {
	if cur >= max || gold <= 0 {
		return cur, 0, false
	}

	rate := int(uint32(price<<10) / uint32(max-cur))
	if rate <= 0 || rate >= gold*0x400 {
		return cur, 0, false
	}

	points := gold * 0x400 / rate

	newDur = cur + points
	if newDur > max {
		newDur = max
	}

	return newDur, gold, true
}
