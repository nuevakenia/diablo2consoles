package d2missile

import (
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// Path type 10, the wobbling path of Firestorm's extra emitters, Twister and
// Tornado (and Charged Strike's bolts, which share the hook). The per-missile
// hook 0x5c7340 (VERIFIED) seeds the missile with destination x + index, sets
// the path type to 10 and caps the step counters at 0x4d; the path then comes
// from PATH_ApplyDirectionStepVelocity 0x67d160 (VERIFIED by disassembly, wave
// 20). This replaces the batch-9 reading, which had decoded 0x5d2df0: that
// function is only the callback of the unused SKILL_UnusedSpawnMissileArcB and
// never runs for these skills.
//
// The rule: n = min(life, 0x4d) / 2 steps. The heading is fixed for the whole
// path, the 8-way direction to the destination (PATH_GetDirectionIndex8
// 0x67ba30). Every step rolls the missile's seed once and turns that heading
// by -1, 0 or +1 eighth through a 32-entry table indexed by the low five bits
// of the new seed; the point moves two subtiles along the turned direction.
// Node 0 is the start, node n the end. Nothing accumulates: the wobble is
// per step around a straight heading.

// pathType10MaxLife is the cap on the step counters (PATH_InitStepCounters).
const pathType10MaxLife = 0x4d

// dirIndex8 is the first column of the 25-entry table at 0x6f2ac8, indexed by
// 5*dx + dy + 12 with both deltas clamped to -2..2. The value is an index into
// pathType10Step.
var dirIndex8 = [25]int{
	5, 4, 4, 4, 3,
	6, 5, 4, 3, 2,
	6, 6, 6, 2, 2,
	6, 7, 0, 1, 2,
	7, 0, 0, 0, 1,
}

// pathType10Step is the table at 0x6f2d48 (x, y per direction, before the
// factor 2 of the stepper).
var pathType10Step = [8][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}

// PathDirectionIndex8 is PATH_GetDirectionIndex8 (0x67ba30, VERIFIED by
// disassembly) followed by the first column of the table 0x6f2ac8: the 8-way
// step direction (an index into 0:+x, 1:+x+y, 2:+y, 3:-x+y, 4:-x, 5:-x-y,
// 6:-y, 7:+x-y) from a position to a destination, in subtiles.
func PathDirectionIndex8(fromX, fromY, toX, toY int) int {
	dx, dy := toX-fromX, toY-fromY
	ax, ay := absInt(dx), absInt(dy)
	ex, ey := dx, dy

	switch {
	case ax < 2*ay && ay >= 2*ax: // mostly vertical: x collapses to -1, 0 or 1
		if dx < 0 {
			ex = -1
		} else {
			ex = dx & 1
		}
	case ax >= 2*ay: // mostly horizontal: y collapses
		if dy < 0 {
			ey = -1
		} else {
			ey = dy & 1
		}
	}

	return dirIndex8[5*clampInt(ex, -2, 2)+clampInt(ey, -2, 2)+12]
}

// pathType10Turn is the 32-entry turn table on the stack of 0x67d160: entry 0
// is 0, entries 2..31 repeat -1, 0, +1. Entry 1 is never written by the exe
// (stale stack); it is taken as 0 here, UNVERIFIED (1 draw in 32).
func pathType10Turn(k uint32) int {
	if k < 2 {
		return 0
	}

	return int((k-2)%3) - 1
}

// PathType10Nodes returns the nodes (subtile points, the start first) that
// path type 10 builds for a missile starting at (sx, sy) towards (dx, dy)
// with the generator seeded by seed (destination x + index) and the given
// lifetime in frames. Fewer than two frames of life give no nodes.
func PathType10Nodes(sx, sy, dx, dy int, seed uint32, life int) [][2]int {
	if life > pathType10MaxLife {
		life = pathType10MaxLife
	}

	n := life / 2
	if n < 1 {
		return nil
	}

	base := PathDirectionIndex8(sx, sy, dx, dy)
	rng := d2rand.New(seed)
	cur := [2]int{sx, sy}
	out := make([][2]int, 0, n+1)

	for i := 0; i < n; i++ {
		k := rng.Step() & 0x1f
		d := (base + pathType10Turn(k)) & 7

		out = append(out, cur)
		cur[0] += pathType10Step[d][0] * 2
		cur[1] += pathType10Step[d][1] * 2
	}

	return append(out, cur)
}

// newWalkPath turns integer nodes into a walkable polyline that starts at the
// missile's exact (float) position; the nodes are applied as offsets.
func newWalkPath(x, y float64, nodes [][2]int) *spiral {
	s := &spiral{pts: make([][2]float64, len(nodes))}
	s.cum = make([]float64, len(nodes))

	for i, n := range nodes {
		s.pts[i] = [2]float64{x + float64(n[0]-nodes[0][0]), y + float64(n[1]-nodes[0][1])}
		if i > 0 {
			s.cum[i] = s.cum[i-1] + math.Hypot(s.pts[i][0]-s.pts[i-1][0], s.pts[i][1]-s.pts[i-1][1])
		}
	}

	s.total = s.cum[len(s.cum)-1]

	return s
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

// OnWalkPath reports whether the missile follows a node path (path type 10 or
// the Blessed Hammer spiral) instead of a straight line.
func (m *Missile) OnWalkPath() bool { return m.spiral != nil }
