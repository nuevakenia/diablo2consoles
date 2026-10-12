package d2inventory

// HeldAction is what a left click on a grid does while an item is held on the cursor (the held branch of
// INV_HandleGridClick, 0x48c3c0, VERIFIED).
type HeldAction int

// The actions.
const (
	// HeldNothing: the click is swallowed (belt/trade page rules, a tome that is full, a cube into a cube page).
	HeldNothing HeldAction = iota
	// HeldPlace: the footprint holds no item (or two or more and no cube): packet 0x18 places the item. With two
	// or more items the server refuses because the cells are taken; the client still plays the drop sound.
	HeldPlace
	// HeldStack: packet 0x21, the held item merges into the single item under it (CanStack).
	HeldStack
	// HeldSocket: packet 0x28, the held item is socketed into the single item under it.
	HeldSocket
	// HeldScrollToTome: packet 0x29, a scroll goes into the matching tome under it.
	HeldScrollToTome
	// HeldIntoCube: packet 0x2a, the held item goes into the Horadric Cube under it.
	HeldIntoCube
	// HeldSwap: packet 0x1f, the single item under the footprint goes to the cursor and the held one takes its place.
	HeldSwap
	// HeldImpossible: the hero says "impossible" (voice 0x13): the cube has no free slot for the item.
	HeldImpossible
)

// HeldUnder describes one item under the footprint, in cell scan order (rows top to bottom, cells left to right).
type HeldUnder struct {
	IsCube bool
	// Stack is CanStack(held, this).
	Stack bool
	// Socket is INV_CanSocketCursorItemIntoTarget (0x480350) for this item.
	Socket bool
	// Tome is true when the held item is a scroll and this item the tome of the same kind.
	Tome bool
	// TomeFull is true when the tome already holds its maximum quantity (the server refuses, 0x55cd70).
	TomeFull bool
}

// HeldInput is the state HeldDrop decides on.
type HeldInput struct {
	// Page is the inventory.txt record page of the grid: 0 inventory, 2 trade, 3 cube, 4 stash.
	Page int
	// HeldIsCube is true when the held item is the Horadric Cube itself.
	HeldIsCube bool
	// Under lists the distinct items under the anchored footprint.
	Under []HeldUnder
	// CubeFits is true when the cube page has a free slot for the held item (INV_FindFreeSlotForItem on page 3).
	CubeFits bool
}

// HeldDrop ports the held-item branch of INV_HandleGridClick (VERIFIED, 0x48c3c0 with 0x63cae0 and 0x63cc50):
//
//   - two or more items under the footprint: INV_GetSingleOverlappedItem with its cube flag returns the first
//     Horadric Cube met in the scan (the flag makes it stop on a cube), and that cube takes the item; with no cube
//     the click falls into the place branch, which the server refuses;
//   - one item: stackable -> stack (not on the trade page, page 2); else socketing; else scroll onto tome; then a
//     Horadric Cube takes the item through packet 0x2a (hero voice "impossible" when it has no free slot); otherwise
//     the swap (not on page 2);
//   - no item: place, except that the cube page (3) ignores a cube held over it.
func HeldDrop(in HeldInput) HeldAction {
	cube := func() HeldAction {
		if !in.CubeFits {
			return HeldImpossible
		}

		return HeldIntoCube
	}

	switch len(in.Under) {
	case 0:
		if in.Page == 3 && in.HeldIsCube {
			return HeldNothing
		}

		return HeldPlace
	case 1:
		u := in.Under[0]

		switch {
		case u.Stack:
			if in.Page == 2 {
				return HeldNothing
			}

			return HeldStack
		case u.Socket:
			return HeldSocket
		case u.Tome:
			if u.TomeFull {
				return HeldNothing
			}

			return HeldScrollToTome
		case u.IsCube:
			return cube()
		}

		if in.Page == 2 {
			return HeldNothing
		}

		return HeldSwap
	}

	for _, u := range in.Under {
		if u.IsCube {
			return cube()
		}
	}

	if in.Page == 3 && in.HeldIsCube {
		return HeldNothing
	}

	return HeldPlace // refused by the server
}
