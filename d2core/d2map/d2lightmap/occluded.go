package d2lightmap

// Occluded light painter: LIGHT_ApplyOccludedLight 0x4709d0 (kind 0 sources at light quality 2) and the
// ray-traced kind 2 painters 0x470d80 / 0x4710f0 (objects, always).
//
// What the notes (renderer.md b4) record: the painter builds a visibility grid by ray marching from the
// source over the occupancy flags of the light map (cells flagged where the collision mask hits 0x22,
// i.e. block-LOS or block-light sub-tiles; occlusion value 16 for an occupied cell, interpolated per ray
// octant, 8 sample rays per ring), then each cell with occlusion below 16 gets
//
//	value = (8 - occl/2) * ((r - d) * (intensity<<16 / r) >> 16) / 8
//
// with d the octagon distance. UNVERIFIED (the per-octant interpolation is not recorded): here every cell
// gets its own straight integer ray from the source cell; a blocked cell between the source and the cell
// puts the cell at occlusion 16 (no light), otherwise 0; a blocked cell itself is lit (its face is what a
// wall sample reads). The kind 2 painters being the same algorithm as painter B is also UNVERIFIED.

// Blocker reports whether an absolute subtile blocks light (collision mask 0x22).
type Blocker func(subX, subY int) bool

// occlusionFull is the occlusion value of a blocked path.
const occlusionFull = 16

// PaintOccluded paints the source like paintRadial but not through blocked sub-tiles.
func (m *Map) PaintOccluded(s *Source, blocked Blocker) {
	r := s.Radius
	if r <= 0 || s.Intensity == 0 {
		return
	}

	step := (int(s.Intensity) << 16) / r
	sx, sy := s.X>>fixedShift, s.Y>>fixedShift
	minX, maxX := (s.X-r)>>fixedShift-m.OriginX, (s.X+r)>>fixedShift-m.OriginX
	minY, maxY := (s.Y-r)>>fixedShift-m.OriginY, (s.Y+r)>>fixedShift-m.OriginY

	for cy := max(minY, 0); cy <= min(maxY, Size-1); cy++ {
		for cx := max(minX, 0); cx <= min(maxX, Size-1); cx++ {
			dx := (cx+m.OriginX)<<fixedShift + 4 - s.X
			dy := (cy+m.OriginY)<<fixedShift + 4 - s.Y
			d := ApproxDistance(dx, dy)

			base := ((r - d) * step) >> 16
			if base <= 0 {
				continue
			}

			occl := rayOcclusion(sx, sy, cx+m.OriginX, cy+m.OriginY, blocked)
			if occl >= occlusionFull {
				continue
			}

			if v := (8 - occl/2) * base / 8; v > 0 {
				m.addToCell(cx, cy, v, s.R, s.G, s.B)
			}
		}
	}
}

// rayOcclusion walks the integer line from (x0,y0) to (x1,y1), both inclusive of the ends but only the
// cells strictly between them can occlude.
func rayOcclusion(x0, y0, x1, y1 int, blocked Blocker) int {
	dx, dy := x1-x0, y1-y0
	n := absInt(dx)

	if a := absInt(dy); a > n {
		n = a
	}

	for i := 1; i < n; i++ {
		// round-to-nearest interpolation along the major axis
		x := x0 + (2*dx*i+sign(dx*n)*n)/(2*n)
		y := y0 + (2*dy*i+sign(dy*n)*n)/(2*n)

		if blocked(x, y) {
			return occlusionFull
		}
	}

	return 0
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}

	return 0
}
