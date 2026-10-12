package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
)

// w42: host side of the exe-ported Hireable AI (d2monster.MercLeaderOps): the state clear, the footprint test
// (COLLISION_TestFootprintMask mask 0x40), the town-room test, the region id (DRLG_GetRegionIdAtPointInRoom) and
// MONAI_TeleportToLeader kinds 0 and 1. The exe reads rooms and DRLG logic regions; this engine has neither, so:
//   - a region is a connected component of the static walkable subtile grid (4-neighbour flood fill, doors count as
//     open; the exe's logic regions are also flood-filled, DRLG_FloodFillLogicRegion; the cell rule is UNVERIFIED);
//   - the room is the merc's level: town when the owner stands in town (or IsTownLevel says so);
//   - the leader's "entry ring" of 20 breadcrumbs is the owner's last 20 distinct subtiles (UNVERIFIED how the exe
//     fills it).

var _ d2monster.MercLeaderOps = (*Director)(nil)

const (
	mercTrailLen   = 20 // the exe's ring has 20 entries (+0xa8, index +0xa0)
	mercStepLen    = 3  // length of one kind 0 step (UNVERIFIED)
	regionFillCap  = 1 << 20
	mercFootprintM = d2path.FlagMissile // mask 0x40
)

// FootprintBlocked implements MercLeaderOps: any cell of the merc's size x size footprint carries flag 0x40.
func (d *Director) FootprintBlocked(b *d2monster.Brain) bool {
	if d.fp == nil {
		return false
	}

	size := b.Size
	if size < 1 {
		size = 1
	}

	x0, y0 := b.X-size/2, b.Y-size/2

	for y := y0; y < y0+size; y++ {
		for x := x0; x < x0+size; x++ {
			if d.fp.Flags(x, y)&mercFootprintM != 0 {
				return true
			}
		}
	}

	return false
}

// InTownRoom implements MercLeaderOps (ROOM_IsTownRoom of the merc's room).
func (d *Director) InTownRoom(b *d2monster.Brain) bool {
	if d.opt.IgnoreTown { // test scenarios fight in town (the exe's mercs would not)
		return false
	}

	if u := d.unitOf(b); u != nil && u.merc != nil && u.merc.owner != nil && u.merc.owner.IsInTown() {
		return true
	}

	return b.LevelID > 0 && d.opt.IsTownLevel != nil && d.opt.IsTownLevel(b.LevelID)
}

// staticGrid is the map without unit footprints.
func (d *Director) staticGrid() d2path.Grid {
	if d.fp != nil {
		return d.fp.grid.Base
	}

	return d.grid
}

type regionMap struct {
	base  d2path.Grid
	label map[d2path.Point]int
	next  int
}

// regionAt labels the walkable component of (x, y) lazily; 0 is "not walkable".
func (d *Director) regionAt(x, y int) int {
	base := d.staticGrid()

	if d.regions == nil || d.regions.base != base {
		d.regions = &regionMap{base: base, label: map[d2path.Point]int{}}
	}

	r := d.regions
	start := d2path.Point{X: x, Y: y}

	if l, ok := r.label[start]; ok {
		return l
	}

	if base.Flags(x, y)&d2path.MaskMonsterOpensDoors != 0 {
		r.label[start] = 0

		return 0
	}

	r.next++
	id := r.next
	queue := []d2path.Point{start}
	r.label[start] = id

	for len(queue) > 0 && len(r.label) < regionFillCap {
		p := queue[0]
		queue = queue[1:]

		for _, n := range [4]d2path.Point{{X: p.X + 1, Y: p.Y}, {X: p.X - 1, Y: p.Y}, {X: p.X, Y: p.Y + 1}, {X: p.X, Y: p.Y - 1}} {
			if _, seen := r.label[n]; seen {
				continue
			}

			if base.Flags(n.X, n.Y)&d2path.MaskMonsterOpensDoors != 0 {
				r.label[n] = 0

				continue
			}

			r.label[n] = id

			queue = append(queue, n)
		}
	}

	return id
}

// SameRegion implements MercLeaderOps: the leader and the merc are in the same walkable component.
func (d *Director) SameRegion(b *d2monster.Brain, o d2monster.OwnerInfo) bool {
	return d.regionAt(b.X, b.Y) == d.regionAt(o.X, o.Y)
}

// leaderDest is where the leader is heading: its walk target while moving, else its position.
func leaderDest(o d2monster.OwnerInfo, u *unit) (int, int) {
	if u != nil && u.merc != nil && u.merc.owner != nil {
		p := u.merc.owner
		if tx, ty := int(p.Target.X()), int(p.Target.Y()); (tx != o.X || ty != o.Y) && tx != 0 && ty != 0 {
			return tx, ty
		}
	}

	return o.X, o.Y
}

// pushTrail records the owner's subtile in the ring (distinct consecutive cells only).
func (mu *mercUnit) pushTrail(x, y int) {
	p := d2path.Point{X: x, Y: y}

	if n := len(mu.trail); n > 0 && mu.trail[n-1] == p {
		return
	}

	if len(mu.trail) == mercTrailLen {
		copy(mu.trail, mu.trail[1:])
		mu.trail = mu.trail[:mercTrailLen-1]
	}

	mu.trail = append(mu.trail, p)
}

// LeaderFollow implements MercLeaderOps for MONAI_TeleportToLeader kinds 0 and 1 (see the notes in
// d2-re-notes/w42-merc-hooks.md). It returns false when no move was started.
func (d *Director) LeaderFollow(b *d2monster.Brain, o d2monster.OwnerInfo, kind int, run bool, speed int) bool {
	u := d.unitOf(b)
	if u == nil || u.merc == nil || u.m == nil {
		return false
	}

	if speed != 0 {
		d.SetSpeed(b, speed)
	}

	var ok bool

	switch kind {
	case 1:
		ok = d.followTrail(u, o, run)
	default:
		ok = d.followStep(u, o, run)
	}

	d.emit("merc", "MERC leader follow kind=%d run=%v speed=%d ok=%v from=(%d,%d) owner=(%d,%d)", kind, run, speed, ok, b.X, b.Y, o.X, o.Y)

	if !ok && d.regionAt(b.X, b.Y) != d.regionAt(o.X, o.Y) {
		// cut off from the leader (door shut behind, other island): nothing walkable gets closer; the exe's
		// distance rule teleports far leaders only, so this rescue is an engine choice (UNVERIFIED)
		ok = d.Teleport(b)
	}

	return ok
}

// followStep is kind 0: one step of mercStepLen toward the leader's destination over the 8 directions, keeping
// the merc's region and taking the direction that ends closest to the destination.
func (d *Director) followStep(u *unit, o d2monster.OwnerInfo, run bool) bool {
	b := u.b
	dx, dy := leaderDest(o, u)
	home := d.regionAt(b.X, b.Y)
	best := d2path.Point{}
	bestD := sqDist(b.X-dx, b.Y-dy)
	found := false

	for _, dir := range [8][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}} {
		p := d2path.Point{X: b.X + dir[0]*mercStepLen, Y: b.Y + dir[1]*mercStepLen}

		if home == 0 || d.regionAt(p.X, p.Y) != home || d.fp.BlockedFor(b.ID, p.X, p.Y) {
			continue
		}

		if dd := sqDist(p.X-dx, p.Y-dy); dd < bestD {
			best, bestD, found = p, dd, true
		}
	}

	if !found {
		return false
	}

	return d.moveTo(u, d2monster.Point{X: best.X, Y: best.Y}, nil, 0, run)
}

// followTrail is kind 1: walk to the oldest breadcrumb of the leader that is in the merc's region and still
// ahead of the merc, else (no crumb works) PetMoveTowardTarget with max(4, dist>>2) toward the leader.
func (d *Director) followTrail(u *unit, o d2monster.OwnerInfo, run bool) bool {
	b := u.b
	home := d.regionAt(b.X, b.Y)

	for _, p := range u.merc.trail {
		if d2monster.EdgeDistance(b.X-p.X, b.Y-p.Y, b.Size) < 2 || d.regionAt(p.X, p.Y) != home {
			continue
		}

		if d.moveTo(u, d2monster.Point{X: p.X, Y: p.Y}, nil, 0, run) {
			return true
		}
	}

	dist := d2monster.EdgeDistance(b.X-o.X, b.Y-o.Y, b.Size)
	step := dist >> 2

	if step < 4 {
		step = 4
	}

	if dist < 1 {
		return false
	}

	// the point `step` subtiles from the merc on the line to the leader
	px := b.X + (o.X-b.X)*step/(dist+1)
	py := b.Y + (o.Y-b.Y)*step/(dist+1)

	free, ok := d2path.NearestFree(d.fp.grid, d2path.MaskMonster, d2path.Point{X: px, Y: py}, 3)
	if !ok {
		return false
	}

	return d.moveTo(u, d2monster.Point{X: free.X, Y: free.Y}, nil, 0, run)
}

func sqDist(dx, dy int) int { return dx*dx + dy*dy }
