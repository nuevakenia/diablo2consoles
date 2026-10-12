package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"

// The line trace of the melee range test. COMBAT_IsTargetWithinMeleeRange (0x622e40, VERIFIED w40) refuses a target
// that passes the table distance test (distance of at least 1) when COLLISION_CanTraceLineBetweenUnits (0x622c80)
// with the collision mask 0x804 reports a blocker. That function hands both unit positions (subtiles) and sizes
// (UNIT_GetSizeX) to COLLISION_TraceLineCheckHelper (0x622af0), which pulls the two end points towards each other and
// walks the cells with COLLISION_TraceLineForMask (0x64f5f0).

// MeleeTraceMask is the collision mask of the trace: closed door (0x800) and wall (0x4).
const MeleeTraceMask = d2path.FlagDoor | d2path.FlagWall

// meleeTraceMaxSize is the cap the helper puts on both unit sizes.
const meleeTraceMaxSize = 2

// MeleeLineClear is COLLISION_CanTraceLineBetweenUnits for the melee test: it reports whether no cell of the mask
// lies on the line between the two units. Units a hair apart (the sum of the axis distances below the sum of the
// capped sizes) count as clear. The room walk of the exe is replaced by the global grid; a cell outside it reads as
// blocked (d2path.OutOfGrid), the exe's "no room found" result.
func MeleeLineClear(g d2path.Grid, mask uint16, x1, y1, size1, x2, y2, size2 int) bool {
	if size1 > meleeTraceMaxSize {
		size1 = meleeTraceMaxSize
	}

	if size2 > meleeTraceMaxSize {
		size2 = meleeTraceMaxSize
	}

	ax, ay := absInt(x2-x1), absInt(y2-y1)
	if ax+ay < size1+size2 {
		return true
	}

	if size1 != 0 || size2 != 0 {
		adjust := func(p1, p2 int) (int, int) {
			if p2 > p1 {
				return p1 + size1, p2 - size2
			}

			return p1 - size1, p2 + size2
		}

		if ay <= ax {
			x1, x2 = adjust(x1, x2)
		}

		if ay >= ax {
			y1, y2 = adjust(y1, y2)
		}
	}

	return !traceBlocked(g, mask, x1, y1, x2, y2)
}

// traceBlocked is COLLISION_TraceLineForMask: both end cells and every cell of the integer line between them are
// tested against the mask. The major axis steps once per cell; the minor axis steps when the accumulated error
// reaches the major distance.
func traceBlocked(g d2path.Grid, mask uint16, x1, y1, x2, y2 int) bool {
	dx, dy := absInt(x2-x1), absInt(y2-y1)
	sx, sy := 1, 1

	if x2 < x1 {
		sx = -1
	}

	if y2 < y1 {
		sy = -1
	}

	x, y, acc := x1, y1, 0

	if dx < dy { // steep: walk y
		for {
			if g.Flags(x, y)&mask != 0 {
				return true
			}

			acc += dx
			y += sy

			if acc >= dy {
				acc -= dy
				x += sx
			}

			if y == y2+sy {
				return false
			}
		}
	}

	if dx == 0 { // dx == dy == 0: the single cell
		return g.Flags(x, y)&mask != 0
	}

	for {
		if g.Flags(x, y)&mask != 0 {
			return true
		}

		acc += dy
		x += sx

		if acc >= dx {
			acc -= dx
			y += sy
		}

		if x == x2+sx {
			return false
		}
	}
}
