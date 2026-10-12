package d2client

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"

// defaultWobbleLife is the life used when the missile row has no range: the exe's hook caps the step counters at
// 0x4d frames (d2missile.PathType10Nodes clamps to it as well).
const defaultWobbleLife = 0x4d

// shotWobbleNodes are the path type 10 nodes of a client shot (CLTDO_019 0x4f1e70 seeds the missile with the
// destination x + index, see clientShot). rangeFrames is the missiles.txt Range column of the bolt (UNVERIFIED as
// the client life; the cap 0x4d applies either way).
func shotWobbleNodes(s clientShot, rangeFrames int) [][2]int {
	life := rangeFrames
	if life <= 0 {
		life = defaultWobbleLife
	}

	return d2missile.PathType10Nodes(s.from.X, s.from.Y, s.to.X, s.to.Y, s.pathSeed, life)
}
