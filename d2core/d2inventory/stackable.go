package d2inventory

// StackProbe is what the stackable test of the original reads from an item.
type StackProbe struct {
	// Code is the base item (txt file number in the exe).
	Code string
	// Quality is the item quality 1..9 (low, normal, superior, magic, set, rare, unique, crafted, tempered).
	Quality int
	// FileIndex is the unique or set row index (0 for plain items).
	FileIndex int
	// Stackable is the base item's stackable flag (only the first item's is read).
	Stackable bool
	Ethereal  bool
	// Sockets is the number of sockets of the item.
	Sockets int
	// Damage holds the stats 0x15, 0x16, 0x17, 0x18, 0x9f and 0xa0 (min/max damage, secondary damage, throw damage).
	Damage [6]int
}

// NonMagicalQuality is ITEM_IsNonMagicalQuality (0x62a460, VERIFIED from the disassembly): the qualities 1 to 3
// (low, normal, superior) and anything above 9; 0 and 4 to 9 (magic, set, rare, unique, crafted, tempered) are not.
func NonMagicalQuality(q int) bool {
	if q == 0 {
		return false
	}

	return q <= 3 || q > 9
}

// CanStack is INV_CheckItemsStackable (0x62c9a0, VERIFIED): the same base item, quality and file index, a
// stackable base, the same ethereal flag, both of a non magical quality, equal damage stats 0x15..0x18, 0x9f and 0xa0
// and no sockets on either. Grid drops (0x48c3c0), the stack packet 0x21, shop top-ups and the hand slots all use it.
func CanStack(a, b StackProbe) bool {
	switch {
	case a.Code != b.Code, a.Quality != b.Quality, a.FileIndex != b.FileIndex:
		return false
	case !a.Stackable, a.Ethereal != b.Ethereal:
		return false
	case !NonMagicalQuality(a.Quality) || !NonMagicalQuality(b.Quality):
		return false
	case a.Damage != b.Damage:
		return false
	}

	return a.Sockets == 0 && b.Sockets == 0
}
