package d2cube

import "testing"

// ITEM_GetMaxSocketsForLevel (0x62bd70) is min(base gemsockets, ItemTypes cap
// for the item level) and a zero cap means no sockets at all: rings have a
// gemsockets of 1 in the table but their type allows none. VERIFIED.
func TestMaxSocketsZeroTypeCap(t *testing.T) {
	types := tsvOf("ItemType\tCode\tEquiv1\tEquiv2\tMaxSock1\tMaxSock25\tMaxSock40",
		"Ring\tring\t\t\t0\t0\t0")
	misc := tsvOf("name\ttype\ttype2\tcode\tgemsockets\tlevel\tspawnable\tstackable\tmaxstack",
		"Ring\tring\t\trin\t1\t1\t1\t0\t0")
	cat := NewCatalog(types, nil, nil, misc)

	if got := cat.MaxSockets("rin", 50); got != 0 {
		t.Errorf("ring max sockets %d, want 0", got)
	}
}
