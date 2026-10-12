package d2skill

// tableDistTab is the 8x8 distance table at 0x6ec730 used by UNIT_GetTableDistance (read wave 28), indexed [|dy|][|dx|] in subtiles. -1 entries mean "touching".
var tableDistTab = [8][8]int{
	{-1, -1, -1, 0, 2, 4, 6, 8},
	{-1, -1, 0, 1, 2, 4, 6, 8},
	{-1, 0, 0, 2, 3, 5, 7, 8},
	{0, 1, 2, 2, 4, 5, 7, 8},
	{2, 2, 3, 4, 5, 6, 7, 9},
	{4, 4, 5, 5, 6, 7, 8, 9},
	{6, 6, 7, 7, 7, 8, 10, 10},
	{8, 8, 8, 8, 9, 9, 10, 11},
}

// tableDistance is UNIT_GetTableDistance (VERIFIED) for two units of size below 2 (every unit this port gives a
// melee burst): inside the 8x8 window the table value plus one (0 for a -1 entry), beyond it the larger delta
// twice plus the smaller. Units of size 2 or more (bosses) use a size-adjusted variant that is not ported.
func tableDistance(dx, dy int) int {
	if dx < 0 {
		dx = -dx
	}

	if dy < 0 {
		dy = -dy
	}

	if dx < 8 && dy < 8 {
		v := tableDistTab[dy][dx]
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

// withinMeleeRange is COMBAT_IsTargetWithinMeleeRange 0x622e40 without the line trace (VERIFIED otherwise):
// table distance below 1, or at most meleeRange + 1 + extra.
func withinMeleeRange(dx, dy, meleeRange, extra int) bool {
	d := tableDistance(dx, dy)

	return d < 1 || d <= meleeRange+1+extra
}
