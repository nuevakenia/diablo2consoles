package d2skill

// tableDistance is UNIT_GetTableDistance (VERIFIED, read in Game.exe) for two units of size 1: the table at
// 0x6ec730 (8x8 entries indexed dx + dy*8 in subtiles, -1 means touching) plus 1. Beyond 7 subtiles on an
// axis the exe uses 2*max+min with no +1 (unit half sizes, not modelled here: size 1 assumed, UNVERIFIED for big
// monsters).
var tableDistanceRows = [8][8]int{
	{-1, -1, -1, 0, 2, 4, 6, 8},
	{-1, -1, 0, 1, 2, 4, 6, 8},
	{-1, 0, 0, 2, 3, 5, 7, 8},
	{0, 1, 2, 2, 4, 5, 7, 8},
	{2, 2, 3, 4, 5, 6, 7, 9},
	{4, 4, 5, 5, 6, 7, 8, 9},
	{6, 6, 7, 7, 7, 8, 10, 10},
	{8, 8, 8, 8, 9, 9, 10, 11},
}

// TableDistance returns the engine's melee distance metric for a subtile offset.
func TableDistance(dx, dy int) int {
	dx, dy = absInt(dx), absInt(dy)

	if dx < 8 && dy < 8 {
		v := tableDistanceRows[dy][dx]
		if v < 0 {
			return 0
		}

		return v + 1
	}

	if dx <= dy {
		return dx + dy*2
	}

	return dy + dx*2
}

// WithinMeleeRange is COMBAT_IsTargetWithinMeleeRange 0x622e40 without the two special monsters (0x102, 0x105:
// range + 8) and without the line trace (collision mask 0x804, not available here: UNVERIFIED): in reach when
// the table distance is below 1, or at most meleeRange + 1 + extra. The scan radius of Fend (meleeRange + 4,
// a Chebyshev-like subtile radius) is a different test and stays as it is.
func WithinMeleeRange(dx, dy, meleeRange, extra int) bool {
	d := TableDistance(dx, dy)

	return d < 1 || d <= meleeRange+1+extra
}
