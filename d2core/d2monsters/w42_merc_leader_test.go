package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// leaderDirector is a 60x30 open map with a wall at x=30 (a gap at y=0..gap-1 when gap > 0), a merc at (mx, 15) and
// its owner at (ox, 15).
func leaderDirector(mx, ox, gap int) (*Director, *unit, d2monster.OwnerInfo) {
	base := d2path.NewCellGrid(0, 0, 60, 30)
	for y := gap; y < 30; y++ {
		base.Set(30, y, d2path.FlagWalk)
	}

	owner := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Level: 40}}
	owner.Position.Set(float64(ox), 15)

	d := &Director{Logger: d2util.NewLogger(), fp: newFootprints(base), units: map[uint32]*unit{},
		mercs: map[*d2mapentity.Player]*unit{}}

	b := d2monster.NewBrain(5, 271, d2monster.Normal, &d2monster.Profile{}, 1)
	b.X, b.Y, b.Size = mx, 15, 1
	u := &unit{m: &d2mapentity.Monster{}, b: b, merc: &mercUnit{owner: owner}}
	u.m.Position.Set(float64(mx), 15)
	d.units[b.ID] = u
	d.mercs[owner] = u

	return d, u, d2monster.OwnerInfo{Target: d2monster.Target{X: ox, Y: 15, Size: 1, IsPlayer: true}}
}

func TestW42RegionsFollowWalkableComponents(t *testing.T) {
	d, u, o := leaderDirector(10, 50, 0) // closed wall: two regions

	if d.SameRegion(u.b, o) {
		t.Error("a closed wall must split the regions")
	}

	if d.regionAt(30, 15) != 0 {
		t.Error("a wall cell has no region")
	}

	d, u, o = leaderDirector(10, 50, 5) // gap at y 0..4
	if !d.SameRegion(u.b, o) {
		t.Error("a gap joins the regions")
	}

	d, u, o = leaderDirector(10, 20, 0)
	if !d.SameRegion(u.b, o) {
		t.Error("same side, same region")
	}
}

func TestW42FootprintTown(t *testing.T) {
	d, u, _ := leaderDirector(10, 20, 0)

	if d.FootprintBlocked(u.b) {
		t.Error("free cell reported blocked")
	}

	d.fp.Move(900, 10, 15, d2path.FlagMissile)

	if !d.FootprintBlocked(u.b) {
		t.Error("flag 0x40 under the merc must block")
	}

	u.b.Size = 3 // the footprint is 3x3 around the merc

	d.fp.Move(900, 11, 16, d2path.FlagMissile)

	if !d.FootprintBlocked(u.b) {
		t.Error("flag 0x40 inside the 3x3 footprint must block")
	}

	if d.InTownRoom(u.b) {
		t.Error("not a town room")
	}

	u.b.LevelID = 1
	d.opt.IsTownLevel = func(l int) bool { return l == 1 }

	if !d.InTownRoom(u.b) {
		t.Error("IsTownLevel must make a town room")
	}
}

func TestW42ClearStateAndTrail(t *testing.T) {
	d, u, _ := leaderDirector(10, 20, 0)
	d.SetUnitState(u.b, 0xc, true)

	if !d.HasState(u.b, 0xc) {
		t.Fatal("state not set")
	}

	d.ClearState(u.b, 0xc)

	if d.HasState(u.b, 0xc) {
		t.Error("state not cleared")
	}

	for i := 0; i < 30; i++ {
		u.merc.pushTrail(i, 1)
		u.merc.pushTrail(i, 1) // duplicates are dropped
	}

	if len(u.merc.trail) != mercTrailLen || u.merc.trail[mercTrailLen-1].X != 29 || u.merc.trail[0].X != 10 {
		t.Errorf("trail ring: len=%d first=%v last=%v", len(u.merc.trail), u.merc.trail[0], u.merc.trail[len(u.merc.trail)-1])
	}
}

// Cut off from the leader by a closed wall: kind 0 finds no step that gets closer inside the merc's region... it
// does get closer (to the wall), so the first step is taken; once at the wall nothing improves and the host
// teleports the merc next to the owner.
func TestW42CutOffMercTeleports(t *testing.T) {
	d, u, o := leaderDirector(28, 50, 0) // 2 cells from the wall, owner behind it

	// a merc right at the wall: every step toward the owner leaves the region or is no closer
	u.b.X = 29
	u.m.Position.Set(29, 15)

	if !d.LeaderFollow(u.b, o, 0, true, 0x3c) {
		t.Fatal("cut-off follow must fall back to the teleport")
	}

	if u.b.X < 31 {
		t.Errorf("merc stayed on the far side: x=%d", u.b.X)
	}
}
