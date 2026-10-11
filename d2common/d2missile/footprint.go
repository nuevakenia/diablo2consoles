package d2missile

// CellFootprint lists the collision cells a missile of the given Size column stamps with a cell bit
// (COLLISION_AddUnitFootprintBits 0x64fdd0 as called by SrvDoFunc 3 / 5, VERIFIED): size 1 is the cell itself;
// size 2 the cell plus its four edge neighbours (COLLISION_OrMaskIntoCell 0x64ef40: x-1, x, x+1 on the row and
// y-1, y+1 on the column); size 3 the 3x3 block around it (COLLISION_SetMaskRectAcrossRooms with x-1..x+1,
// y-1..y+1). Any other size stamps nothing. Blizzard shards (blizzard1) carry Size 2, so each stamps a plus.
func CellFootprint(x, y, size int) [][2]int {
	switch size {
	case 1:
		return [][2]int{{x, y}}
	case 2:
		return [][2]int{{x - 1, y}, {x, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}}
	case 3:
		out := make([][2]int, 0, 9)

		for cy := y - 1; cy <= y+1; cy++ {
			for cx := x - 1; cx <= x+1; cx++ {
				out = append(out, [2]int{cx, cy})
			}
		}

		return out
	}

	return nil
}
