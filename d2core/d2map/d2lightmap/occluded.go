package d2lightmap

// Occluded light painter: LIGHT_ApplyOccludedLight 0x4709d0 (kind 0 sources at light quality 2) and the
// kind 2 pair LIGHT_TraceOcclusionForLightSource 0x470d80 + LIGHT_ApplyLightSourceToLightMap 0x4710f0
// (objects, always). Read from the exe (w37):
//
//   - a 64x64 occupancy grid is centred on the source subtile (grid cell 32,32); a blocked cell holds 16.
//     Kind 0 fills it from the 48x48 light map flag array (outside that window = blocked); kind 2 reads the
//     collision mask 0x22 around the unit directly (no window).
//   - for rings t = 0..R-2 (R = radius>>3, Chebyshev distance t+2 from the source cell) and j = 0..t+2,
//     8 rays end on the ring: (-j,-d) (+j,-d) (-j,+d) (+j,+d) (+d,-j) (+d,+j) (-d,-j) (-d,+j), d = t+2.
//     Each target value is interpolated from the neighbours one step toward the source (step direction from
//     the sign of source - target, in 1/8 subtile units; blend weight = minor/major*256): a blocked
//     neighbour counts 16, otherwise the neighbour's already computed value (cells at distance <= 1 are 0).
//   - painting: cells with occlusion < 16 get (8 - occl/2) * ((r - d) * (intensity<<16 / r) >> 16) >> 3.
//
// The kind 2 trace order is the same as kind 0, so one implementation serves both; kind 2 additionally
// caches the result until the radius changes (a pure function here, no cache needed).
// UNVERIFIED: the exe measures the distance from the source to the cell corner; the radial painter in this
// package (paintRadial) uses the cell centre, and so does this one, to keep the two consistent.

// Blocker reports whether an absolute subtile blocks light (collision mask 0x22).
type Blocker func(subX, subY int) bool

// occlusionFull is the occlusion value of a blocked path.
const occlusionFull = 16

// PaintOccluded paints the source like paintRadial but not through blocked sub-tiles.

// occGridSize is the side of the occupancy and result grids; the source sits at gridCentre.
const (
	occGridSize = 64
	gridCentre  = 32
)

// occGrids is the pair of 64x64 work buffers of the exe (DAT_007a4ad8 occupancy, DAT_007a0ad8 result).
type occGrids struct {
	occ [occGridSize * occGridSize]int
	res [occGridSize * occGridSize]int
}

// val is the value a ray takes from a neighbour: 16 when it is blocked, else its computed result.
func (g *occGrids) val(row, col int) int {
	if o := g.occ[row*occGridSize+col]; o != 0 {
		return o
	}

	return g.res[row*occGridSize+col]
}

// traceCell is LIGHT_SetupOcclusionRayStepImpl + LIGHT_StepOcclusionRayImpl for the target cell (row,col)
// whose centre is (px,py) in 1/8 subtile units; (sx,sy) is the source position.
func (g *occGrids) traceCell(row, col, sx, sy, px, py int) {
	dx, dy := sx-px, sy-py
	if dx == 0 && dy == 0 {
		return
	}

	dirX, dirY := 1, 1
	if dx < 0 {
		dirX, dx = -1, -dx
	}

	if dy < 0 {
		dirY, dy = -1, -dy
	}

	var v int

	switch {
	case dx == 0:
		v = g.val(row+dirY, col)
	case dy == 0:
		v = g.val(row, col+dirX)
	case dy <= dx:
		ratio := (dy << 8) / dx
		v = ((256-ratio)*g.val(row, col+dirX) + ratio*g.val(row+dirY, col+dirX)) >> 8
	default:
		ratio := (dx << 8) / dy
		v = ((256-ratio)*g.val(row+dirY, col) + ratio*g.val(row+dirY, col+dirX)) >> 8
	}

	g.res[row*occGridSize+col] = v
}

// trace runs the ring loop for a source of radius r at (sx,sy).
func (g *occGrids) trace(sx, sy, r int) {
	R := r >> 3
	if R <= 1 {
		return
	}

	cx8, cy8 := (sx>>3)<<3, (sy>>3)<<3

	at := func(kx, ky int) { g.traceCell(gridCentre+ky, gridCentre+kx, sx, sy, cx8+4+kx*8, cy8+4+ky*8) }

	for t := 0; t <= R-2; t++ {
		d := t + 2

		for j := 0; j < t+3; j++ {
			at(-j, -d)
			at(j, -d)
			at(-j, d)
			at(j, d)
			at(d, -j)
			at(d, j)
			at(-d, -j)
			at(-d, j)
		}
	}
}

// PaintOccluded paints the source like paintRadial but not through blocked sub-tiles (kind 0 painter;
// Source.Object selects the kind 2 occupancy without the 48x48 window).
func (m *Map) PaintOccluded(s *Source, blocked Blocker) {
	r := s.Radius
	if r <= 0 || r >= 0x100 || s.Intensity == 0 {
		return
	}

	R := r >> 3
	// origin of the grid region in 1/8 subtile units, then in subtiles; the exe fills n = (2r>>3)+1 cells a side
	ox8 := (s.X &^ 7) - r
	oy8 := (s.Y &^ 7) - r
	osx, osy := ox8>>3, oy8>>3
	n := (r*2)>>3 + 1

	g := &occGrids{}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			sx, sy := osx+j, osy+i

			var isBlocked bool

			if !s.Object && (sx < m.OriginX || sx >= m.OriginX+Size || sy < m.OriginY || sy >= m.OriginY+Size) {
				isBlocked = true
			} else {
				isBlocked = blocked(sx, sy)
			}

			if isBlocked {
				g.occ[(gridCentre-R+i)*occGridSize+gridCentre-R+j] = occlusionFull
			}
		}
	}

	g.trace(s.X, s.Y, r)

	step := (int(s.Intensity) << 16) / r

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			occl := g.res[(gridCentre-R+i)*occGridSize+gridCentre-R+j]
			if occl >= occlusionFull {
				continue
			}

			cx, cy := osx+j-m.OriginX, osy+i-m.OriginY
			if cx < 0 || cy < 0 || cx >= Size || cy >= Size {
				continue
			}

			d := ApproxDistance((osx+j)<<fixedShift+4-s.X, (osy+i)<<fixedShift+4-s.Y)

			if v := (8 - occl>>1) * (((r - d) * step) >> 16) >> 3; v > 0 {
				m.addToCell(cx, cy, v, s.R, s.G, s.B)
			}
		}
	}
}
