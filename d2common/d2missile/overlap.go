package d2missile

// FootprintOverlap is the unit test of the missile move (ROOM_ForEachNearRoomContainingPoint 0x6430a0, V wave 22,
// called by MISSILE_SrvDoStandardMove 0x5abd00 with UNIT_GetSizeX of the MISSILE as its size argument): a unit
// of size unitSize (monstats2 SizeX, capped at 3; below 1 it is never hit) standing at (ux, uy) is hit by a missile
// of Size missileSize (1..3) whose path node is (mx, my) when the two footprints overlap. The shape is picked by
// the pair of sizes: 1/1 the same cell; 1/2 and 2/1 the cell plus its four edge neighbours (|dx|+|dy| <= 1);
// 1/3 and 3/1 the 3x3 block; 2/2 a diamond (|dx|+|dy| <= 2); 2/3 and 3/2 the 3x3 block grown by one cell on each
// edge (a 5x5 without its corners); 3/3 the 5x5 block.
//
// The Sim still finds targets through World.Targets (one cell per node, with the engine's own radius), so this
// function is not wired: replacing the lookup would change the reach of every size 1 missile, and the sizes
// 2 and 3 (lightning, blizzard shards, the erruption cracks, volcano fire, molten boulder) would need the
// engine to hand over unit positions. UNVERIFIED: which path nodes the move scans (all nodes of the frame, V in
// the standard move) against the engine's per-frame step.
func FootprintOverlap(missileSize, unitSize, mx, my, ux, uy int) bool {
	if missileSize < 1 || missileSize > 3 || unitSize < 1 {
		return false
	}

	if unitSize > 3 {
		unitSize = 3
	}

	dx, dy := absInt(mx-ux), absInt(my-uy)

	switch (unitSize - 1) + 3*(missileSize-1) {
	case 0:
		return dx == 0 && dy == 0
	case 1, 3:
		return dx+dy <= 1
	case 2, 6:
		return dx <= 1 && dy <= 1
	case 4:
		return dx+dy <= 2
	case 5, 7:
		if dx < 3 && dy < 2 {
			return true
		}

		return dy < 3 && dx <= 1
	case 8:
		return dx <= 2 && dy <= 2
	}

	return false
}
