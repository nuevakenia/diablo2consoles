package d2skill

// tableDistance is UNIT_GetTableDistance (VERIFIED, read in Game.exe) for two units of size 1: the table at
// 0x6ec730 (8x8 entries indexed dx + dy*8 in subtiles, -1 means touching) plus 1. Beyond 7 subtiles on an
// axis the exe uses 2*max+min with no +1. Unit sizes are not modelled by TableDistance (size 1 assumed); the full
// size handling, VERIFIED, is TableDistanceSized.
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

// TableDistanceSized is the whole of UNIT_GetTableDistance 0x642980 (VERIFIED) with the unit sizes
// (UNIT_GetSizeX of both units; 1 for a player and a normal monster, 2 or 3 for large ones). Inside 8 subtiles on
// both axes and with both sizes below 4 it reads the table: -1 is 0, a size 3 unit takes 1 off, and the +1 is only
// added when one of the units has a size below 2. In every other case the half sizes (size / 2 each) come off the
// axis distances, both are floored at 0 and the result is the smaller axis plus twice the larger one. The size
// parameters are not wired: the callers of this port pass no unit sizes (TableDistance stands for sizes 1).
func TableDistanceSized(dx, dy, sizeA, sizeB int) int {
	dx, dy = absInt(dx), absInt(dy)

	if dx < 8 && dy < 8 && sizeA < 4 && sizeB < 4 {
		v := tableDistanceRows[dy][dx]
		if v < 0 {
			return 0
		}

		if sizeA == 3 || sizeB == 3 {
			v--
		}

		if v < 0 {
			v = 0
		}

		if sizeA < 2 || sizeB < 2 {
			return v + 1
		}

		return v
	}

	half := sizeA/2 + sizeB/2
	dx, dy = maxInt(dx-half, 0), maxInt(dy-half, 0)

	if dx <= dy {
		return dx + dy*2
	}

	return dy + dx*2
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

// WithinMeleeRange is COMBAT_IsTargetWithinMeleeRange 0x622e40 without the two special monsters (see
// WithinMeleeRangeOfClass) and without the line trace (COLLISION_CanTraceLineBetweenUnits with the collision mask
// 0x804: the exe refuses the blow when that trace reports a blocker; the pure functions here have no map, so the
// trace is not modelled, UNVERIFIED which cells 0x804 stands for): in reach when the table distance is below 1,
// or at most meleeRange + 1 + extra. The scan radius of Fend (meleeRange + 4,
// a Chebyshev-like subtile radius) is a different test and stays as it is.
func WithinMeleeRange(dx, dy, meleeRange, extra int) bool {
	d := TableDistance(dx, dy)

	return d < 1 || d <= meleeRange+1+extra
}

// Special target classes of 0x622e40 (VERIFIED, monstats row numbers): WaterWatcherLimb (0x102) and
// WaterWatcherHead (0x105).
const (
	classWaterWatcherLimb = 0x102
	classWaterWatcherHead = 0x105
)

// WithinMeleeRangeOfClass is WithinMeleeRange for a monster target of the given base class: the two water watcher
// parts are in reach as soon as the table distance is below meleeRange + 8 (checked before the normal test, with
// no line trace). Pass -1 for a target that is not a monster.
func WithinMeleeRangeOfClass(dx, dy, meleeRange, extra, targetClass int) bool {
	if (targetClass == classWaterWatcherLimb || targetClass == classWaterWatcherHead) &&
		TableDistance(dx, dy) < meleeRange+8 {
		return true
	}

	return WithinMeleeRange(dx, dy, meleeRange, extra)
}

// AttackInReach is the range gate of COMBAT_RollAttackOutcome 0x57cc10 (VERIFIED w29) for callers that pass no extra
// range argument (every melee skill SRVST/SRVDO, the stack argument is 0): COMBAT_IsTargetWithinMeleeRange with
// nExtra = 1 for a player attacker and 3 for a monster attacker (the attacker's unit type is 1), so a blow lands when the
// table distance is below 1 or at most meleeRange + 2 (players) / meleeRange + 4 (monsters). The line trace of 0x622e40
// is not modelled (UNVERIFIED).
func AttackInReach(monsterAttacker bool, meleeRange, dx, dy int) bool {
	extra := 1
	if monsterAttacker {
		extra = 3
	}

	return WithinMeleeRange(dx, dy, meleeRange, extra)
}
