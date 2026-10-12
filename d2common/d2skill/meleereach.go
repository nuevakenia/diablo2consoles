package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"

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

// UnitSizer is implemented by targets that know their UNIT_GetSizeX (monstats2 SizeX for monsters).
type UnitSizer interface{ UnitSize() int }

// HeroUnitSize is UNIT_GetSizeX of a player (2, read at 0x6207a0 w37).
const HeroUnitSize = 2

// UnitSizeOf returns the size of a target (1 when it does not say).
func UnitSizeOf(t interface{}) int {
	if s, ok := t.(UnitSizer); ok && s.UnitSize() > 0 {
		return s.UnitSize()
	}

	return 1
}

// TableDistanceSized is UNIT_GetTableDistance 0x642980 (VERIFIED w37) with the unit sizes sa, sb (UNIT_GetSizeX):
// offsets below 8 on both axes and sizes below 4 read the table (touching reads 0 and ends there), minus 1 when a size
// is 3 (not below 0), plus 1 only when one of the sizes is below 2; otherwise each axis loses (sa/2 + sb/2), is floored
// at 0 and the result is 2*max+min.
func TableDistanceSized(dx, dy, sa, sb int) int {
	dx, dy = absInt(dx), absInt(dy)

	if dx < 8 && dy < 8 && sa < 4 && sb < 4 {
		v := tableDistanceRows[dy][dx]
		if v < 0 {
			return 0
		}

		if sa == 3 || sb == 3 {
			v--
		}

		if v < 0 {
			v = 0
		}

		if sa < 2 || sb < 2 {
			return v + 1
		}

		return v
	}

	h := sa/2 + sb/2
	dx, dy = absInt(dx-h+absInt(dx-h))/2, absInt(dy-h+absInt(dy-h))/2

	if dx <= dy {
		return dx + dy*2
	}

	return dy + dx*2
}

// AttackInReachSized is AttackInReach with the unit sizes of the exe's table distance (hero attacker: HeroUnitSize).
func AttackInReachSized(monsterAttacker bool, meleeRange, dx, dy, sa, sb int) bool {
	extra := 1
	if monsterAttacker {
		extra = 3
	}

	d := TableDistanceSized(dx, dy, sa, sb)

	return d < 1 || d <= meleeRange+1+extra
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
// 0x804 = closed door + wall: the exe refuses the blow when that trace reports a blocker; the pure functions here
// have no map, so the trace lives in MeleeLineClear, VERIFIED w40): in reach when the table distance is below 1,
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
// is MeleeLineClear (meleetrace.go, w40), applied by cast.outOfReach where the pipeline has a grid; these pure helpers skip it. The special classes 0x102/0x105 get range + 8 in 0x622e40
// (VERIFIED w37, not ported: the engine has no such ids). AttackInReachSized adds the unit sizes.
func AttackInReach(monsterAttacker bool, meleeRange, dx, dy int) bool {
	extra := 1
	if monsterAttacker {
		extra = 3
	}

	return WithinMeleeRange(dx, dy, meleeRange, extra)
}

// MeleeReachFull is the whole COMBAT_IsTargetWithinMeleeRange 0x622e40 (VERIFIED w38 from the decompile):
//  1. a monster target of base class 0x102 / 0x105 is in reach when the sized table distance is below meleeRange+8
//     (no line trace);
//  2. otherwise in reach when the sized table distance is below 1 (no trace either);
//  3. otherwise, when the distance is at most meleeRange+1+extra, in reach exactly when the line trace between the
//     two units for collision mask 0x804 reports no blocker (COLLISION_CanTraceLineBetweenUnits, nonzero = blocked);
//     farther away it is out of reach.
//
// blocked may be nil (no map): the trace then counts as clear, as before w38. The caller closes over the two
// positions and returns true when the 0x804 trace is blocked.
func MeleeReachFull(dx, dy, sa, sb, meleeRange, extra, targetClass int, blocked func() bool) bool {
	d := TableDistanceSized(dx, dy, sa, sb)

	if (targetClass == classWaterWatcherLimb || targetClass == classWaterWatcherHead) && d < meleeRange+8 {
		return true
	}

	if d < 1 {
		return true
	}

	if d > meleeRange+1+extra {
		return false
	}

	return blocked == nil || !blocked()
}

// TraceBlocked804 reports whether the line between two subtile positions is blocked for collision mask 0x804
// (the trace of 0x622e40). A nil grid never blocks.
func TraceBlocked804(g d2path.Grid, ax, ay, bx, by int) bool {
	if g == nil {
		return false
	}

	clear, _ := d2path.TraceLine(g, 0x804, d2path.Point{X: ax, Y: ay}, d2path.Point{X: bx, Y: by})

	return !clear
}
