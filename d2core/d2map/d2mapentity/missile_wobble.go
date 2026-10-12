package d2mapentity

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"

// WobblePathPoints turns the nodes of d2missile.PathType10Nodes (subtile points, the start first) into the
// positions the entity walks through; the start node is dropped because the entity already stands on it.
func WobblePathPoints(nodes [][2]int) []d2vector.Position {
	if len(nodes) < 2 {
		return nil
	}

	out := make([]d2vector.Position, 0, len(nodes)-1)
	for _, n := range nodes[1:] {
		out = append(out, d2vector.NewPosition(float64(n[0]), float64(n[1])))
	}

	return out
}

// SetWobblePath makes the legacy client missile walk the path type 10 nodes (Charged Strike bolts, CLTDO_019
// 0x4f1e70 sets path type 10 on the client missile as well). done is called at the last node.
func (m *Missile) SetWobblePath(nodes [][2]int, done func()) {
	pts := WobblePathPoints(nodes)
	if len(pts) == 0 {
		if done != nil {
			done()
		}

		return
	}

	m.SetPath(pts, done)
}
