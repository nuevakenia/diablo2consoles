package d2gamescreen

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2object"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// The Arcane Sanctuary is a set of platforms joined by teleport pads. The scripted walk (walkto:, kill:) plans on the ground
// only: for some map seeds the Summoner stands on a platform the pathfinder cannot reach ("cannot get closer than 75 tiles",
// wave 43). "padsto <tileX> <tileY>" is the script's stand-in for a player who steps on the pads one after the other: it
// labels the walkable regions of the level, links each pad's region to the region its partner pad lands in, and moves the
// hero along the shortest chain of pads to the region of the target (the hero is moved from pad to pad at once; a player
// would walk between them).

type padLink struct {
	from, to int // regions
	pad      *d2mapentity.Object
	lx, ly   int // landing subtile
}

// commandPadsTo is "padsto <tileX> <tileY>".
func (v *Game) commandPadsTo(args []string) error {
	if len(args) != 2 || v.gameClient == nil || v.localPlayer == nil {
		return errors.New("usage: padsto <tileX> <tileY> (in a game)")
	}

	tx, err1 := strconv.ParseFloat(args[0], 64)
	ty, err2 := strconv.ParseFloat(args[1], 64)

	if err1 != nil || err2 != nil {
		return errors.New("usage: padsto <tileX> <tileY>")
	}

	m := v.gameClient.MapEngine
	size := m.Size()
	w, h := size.Width*subtilesInTile, size.Height*subtilesInTile
	label := make([]int32, w*h) // 0 unlabelled, -1 blocked, n>0 region
	next := int32(0)

	fill := func(sx, sy int) {
		next++
		queue := [][2]int{{sx, sy}}
		label[sy*w+sx] = next

		for len(queue) > 0 {
			c := queue[len(queue)-1]
			queue = queue[:len(queue)-1]

			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				x, y := c[0]+d[0], c[1]+d[1]
				if x < 0 || y < 0 || x >= w || y >= h || label[y*w+x] != 0 {
					continue
				}

				if m.WalkBlocked(x, y) {
					label[y*w+x] = -1

					continue
				}

				label[y*w+x] = next
				queue = append(queue, [2]int{x, y})
			}
		}
	}

	regionAt := func(sx, sy int) int {
		if sx < 0 || sy < 0 || sx >= w || sy >= h {
			return 0
		}

		if label[sy*w+sx] == 0 {
			if m.WalkBlocked(sx, sy) {
				label[sy*w+sx] = -1
			} else {
				fill(sx, sy)
			}
		}

		return int(label[sy*w+sx])
	}

	nearest := func(sx, sy, radius int) (int, int, bool) { return m.NearestWalkable(sx, sy, radius) }

	hx, hy := v.heroTilePos()
	hsx, hsy := int(hx*subtilesInTile), int(hy*subtilesInTile)

	if x, y, ok := nearest(hsx, hsy, 6); ok {
		hsx, hsy = x, y
	}

	tsx, tsy := int(tx*subtilesInTile), int(ty*subtilesInTile)

	if x, y, ok := nearest(tsx, tsy, 15); ok {
		tsx, tsy = x, y
	}

	start, goal := regionAt(hsx, hsy), regionAt(tsx, tsy)
	if start <= 0 || goal <= 0 {
		return fmt.Errorf("padsto: hero region %d, target region %d", start, goal)
	}

	if start == goal {
		v.Infof("PADSTO target (%.0f,%.0f) is on the hero's platform already", tx, ty)

		return nil
	}

	var pads []*d2mapentity.Object

	for _, e := range m.Entities() {
		if o, ok := e.(*d2mapentity.Object); ok {
			rec := o.Record()
			if d2object.InfoFor(d2object.Def{OperateFn: rec.OperateFn, SubClass: rec.SubClass}).Fn == d2object.FnTeleportPad {
				pads = append(pads, o)
			}
		}
	}

	var links []padLink

	for _, pad := range pads {
		px, py := pad.GetPositionF()
		self := d2object.Pos{X: px, Y: py}

		var all []d2object.Pos

		for _, o := range pads {
			if o.Record().Index == pad.Record().Index {
				x, y := o.GetPositionF()
				all = append(all, d2object.Pos{X: x, Y: y})
			}
		}

		to, ok := d2object.PadPartner(self, all)
		if !ok {
			continue
		}

		fx, fy, ok1 := nearest(int(px*subtilesInTile), int(py*subtilesInTile), 8)
		lx, ly, ok2 := nearest(int(to.X*subtilesInTile), int(to.Y*subtilesInTile), padLandingRadius)

		if !ok1 || !ok2 {
			continue
		}

		links = append(links, padLink{from: regionAt(fx, fy), to: regionAt(lx, ly), pad: pad, lx: lx, ly: ly})
	}

	// breadth first over the regions
	prev := map[int]padLink{start: {}}
	queue := []int{start}

	for len(queue) > 0 && prev[goal].pad == nil {
		r := queue[0]
		queue = queue[1:]

		for _, l := range links {
			if l.from != r || l.to <= 0 {
				continue
			}

			if _, seen := prev[l.to]; !seen {
				prev[l.to] = l

				queue = append(queue, l.to)
			}
		}
	}

	if prev[goal].pad == nil {
		return fmt.Errorf("padsto: no chain of pads from region %d to region %d (%d pads, %d links)", start, goal, len(pads), len(links))
	}

	var chain []padLink

	for r := goal; r != start; r = prev[r].from {
		chain = append([]padLink{prev[r]}, chain...)
	}

	for _, l := range chain {
		px, py := l.pad.GetPositionF()
		v.localPlayer.SetPositionSubtile(l.lx, l.ly)
		v.Infof("PADSTO pad (%.1f,%.1f) -> hero lands at subtile (%d,%d), region %d -> %d", px, py, l.lx, l.ly, l.from, l.to)
	}

	v.Infof("PADSTO reached the platform of (%.0f,%.0f) over %d pad(s)", tx, ty, len(chain))

	return nil
}
