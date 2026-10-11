package d2monreg

// Footprint is the set of subtiles the exe tests for terrain blocking when it places a monster of the given
// monstats2 SizeX: COLLISION_GetFootprintMaskByShapeAlt 0x64ec70, called by MONSTER_SpawnMonsterUnitCore 0x5b0600
// with the class' bSizeX (VERIFIED w20, from the decompile of 0x64ec70, 0x64e360 and 0x64e730):
//
//	SizeX 0 and 1  the spot itself (0x64e6e0)
//	SizeX 2        the spot and its four direct neighbours (0x64e360, named "3x3" but it reads five cells)
//	SizeX 3        the 3x3 square around the spot (0x64e730, inclusive rectangle x-1..x+1, y-1..y+1)
//	SizeX 4 and up the test returns 0xffff: the spot is always refused
//
// The earlier model tested a square of half-width SizeX (3x3 for size 1, 5x5 for 2, 7x7 for 3), which refused far too
// many spots on narrow ground such as the River of Flame.
func Footprint(size int) (cells [][2]int, blockAll bool) {
	switch {
	case size <= 1:
		return [][2]int{{0, 0}}, false
	case size == 2:
		return [][2]int{{0, 0}, {0, 1}, {0, -1}, {-1, 0}, {1, 0}}, false
	case size == 3:
		return [][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {0, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}, false
	}

	return nil, true
}
