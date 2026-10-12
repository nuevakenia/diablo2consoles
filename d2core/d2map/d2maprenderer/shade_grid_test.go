package d2maprenderer

import "testing"

// The cached vertex offsets must be exactly what the per-draw computation produced.
func TestShadeGridMatchesDirect(t *testing.T) {
	mr := &MapRenderer{}

	for _, kind := range []shadeKind{shadeFloor, shadeWall} {
		for _, sz := range [][4]int{{160, 80, 4, 2}, {160, 200, 4, 4}, {160, 31, 5, 3}} {
			w, h, cols, rows := sz[0], sz[1], sz[2], sz[3]

			for pass := 0; pass < 2; pass++ { // miss, then hit
				g := mr.shadeGrid(kind, w, h, cols, rows)
				if len(g) != (cols+1)*(rows+1) {
					t.Fatalf("len %d", len(g))
				}

				n := 0

				for j := 0; j <= rows; j++ {
					for i := 0; i <= cols; i++ {
						x, y := float64(i*w)/float64(cols), float64(j*h)/float64(rows)

						var u, v float64
						if kind == shadeFloor {
							u, v = floorSubtile(x, y)
						} else {
							u, v = wallBaseSubtile(x)
						}

						if g[n].u != u || g[n].v != v {
							t.Fatalf("kind %d %v vertex %d: got (%v,%v) want (%v,%v)", kind, sz, n, g[n].u, g[n].v, u, v)
						}

						n++
					}
				}
			}
		}
	}
}
