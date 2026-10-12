package d2monster

// SameClassFinder is MONAI_FindNearestSameClassMonsterCallback 0x5ae8d0 run through the unit walk
// (MONAI_ForEachUnitByModeCallback mode 1), wave 38 (VERIFIED): another monster-type unit with the same base class
// id (MONSTER_GetBaseClassId) that is neither dead nor dying, keeping the unit with the smallest
// SQUARED distance (UNIT_DistanceSquaredBetweenUnits), a strictly smaller value replacing the best. No
// alignment or hostility test. The PantherWoman compares aip3 squared with the result; the PantherJavelin
// compares aip3 itself (not squared) with it, a quirk of the exe.
type SameClassFinder interface {
	NearestSameClass(b *Brain) (t Target, sqDist int, ok bool)
}

// nearestSameClass asks the world for the callback result. Without a SameClassFinder it falls back on the
// older AllyFinder (edge distance squared).
func (c *Ctx) nearestSameClass() (Target, int, bool) {
	if f, ok := c.W.(SameClassFinder); ok {
		return f.NearestSameClass(c.B)
	}

	if f, ok := c.W.(AllyFinder); ok {
		t, d, found := f.NearestAlly(c.B)

		return t, d * d, found
	}

	return Target{}, 0, false
}
