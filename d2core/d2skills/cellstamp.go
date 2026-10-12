package d2skills

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"

// cellStamps is the production d2missile.CellMarker (wave 22). SrvDoFunc 3 and 5 (Blizzard shards, poison
// clouds, fire walls) OR the missile bit 0x40 into the footprint of their Size column
// (d2missile.CellFootprint, 0x64fdd0 VERIFIED) on every frame, so the stamp is kept for two frames and
// dropped after: a missile that moved or died stops marking its old cells. UNVERIFIED: the exe's exact
// clearing point (it must remove the footprint when the missile moves or dies); two frames of grace is the
// simplest model that keeps Eruption sub-missiles off the cells of live shards.
type cellStamps struct {
	cur, prev map[[2]int]uint16
}

func newCellStamps() *cellStamps {
	return &cellStamps{cur: map[[2]int]uint16{}, prev: map[[2]int]uint16{}}
}

// roll starts a new frame: last frame's stamps become the grace generation, older ones are dropped.
func (c *cellStamps) roll() {
	if c == nil {
		return
	}

	c.prev, c.cur = c.cur, c.prev
	for k := range c.cur {
		delete(c.cur, k)
	}
}

func (c *cellStamps) mark(x, y, size int, flag uint16) {
	if c == nil {
		return
	}

	for _, cell := range d2missile.CellFootprint(x, y, size) {
		c.cur[cell] |= flag
	}
}

func (c *cellStamps) flags(x, y int) uint16 {
	if c == nil {
		return 0
	}

	k := [2]int{x, y}

	return c.cur[k] | c.prev[k]
}

// MarkCell implements d2missile.CellMarker.
func (w *world) MarkCell(x, y, size int, flag uint16) { w.e.stamps.mark(x, y, size, flag) }

var _ d2missile.CellMarker = (*world)(nil)
