package d2monster

// Distance is the metric used by the AI (VERIFIED, 0x5db280): with
// dx=|x1-x2| and dy=|y1-y2| it is max(dx,dy)+min(dx,dy)/2 (integer division).
func Distance(dx, dy int) int {
	if dx < 0 {
		dx = -dx
	}

	if dy < 0 {
		dy = -dy
	}

	if dx < dy {
		dx, dy = dy, dx
	}

	return dx + dy/2
}

// EdgeDistance is Distance after subtracting a unit size from each axis,
// clamped at zero (VERIFIED for the "first subtracts the unit's size" part of
// MONAI_GetEdgeDistanceBetweenUnits 0x5db0d0; VERIFIED w37: it subtracts UNIT_GetSizeX of the FIRST unit only, from
// both axes, clamps at zero and returns (min + 2*max)/2, i.e. this Distance; the second unit's size is not used, so a
// single size is right).
func EdgeDistance(dx, dy, size int) int {
	if dx < 0 {
		dx = -dx
	}

	if dy < 0 {
		dy = -dy
	}

	dx -= size
	dy -= size

	if dx < 0 {
		dx = 0
	}

	if dy < 0 {
		dy = 0
	}

	return Distance(dx, dy)
}

// UnitDistance is UNIT_GetDistanceToUnit 0x642b10 (w40, VERIFIED): both axes lose sizeA/2 + sizeB/2 (integer
// halves, clamped at zero) before the usual max + min/2 metric. Totem uses it; most AIs use EdgeDistance.
func UnitDistance(dx, dy, sizeA, sizeB int) int {
	if dx < 0 {
		dx = -dx
	}

	if dy < 0 {
		dy = -dy
	}

	g := sizeA/2 + sizeB/2
	dx -= g
	dy -= g

	if dx < 0 {
		dx = 0
	}

	if dy < 0 {
		dy = 0
	}

	return Distance(dx, dy)
}
