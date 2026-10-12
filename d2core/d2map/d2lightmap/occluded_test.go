package d2lightmap

import "testing"

func TestTraceGrid(t *testing.T) {
	// source in the middle of subtile (0,0): X = 4 (the +4 of SetPosition), wall column at +4
	g := &occGrids{}
	for row := 0; row < occGridSize; row++ {
		g.occ[row*occGridSize+gridCentre+4] = occlusionFull
	}

	g.trace(4, 4, 8*8)

	at := func(kx, ky int) int { return g.res[(gridCentre+ky)*occGridSize+gridCentre+kx] }

	tests := []struct {
		name   string
		kx, ky int
		want   int
	}{
		{"in front", 3, 0, 0},
		{"wall cell itself", 4, 0, 0},
		{"behind, straight", 6, 0, occlusionFull},
		{"behind, straight left side", -5, 0, 0},
		{"behind, diagonal", 6, 6, occlusionFull},
	}

	for _, tc := range tests {
		if got := at(tc.kx, tc.ky); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestTraceCellModes(t *testing.T) {
	// a neighbour at (row 32, col 33) blocked; target (row 32, col 34), source to the left (dy == 0): mode 1
	g := &occGrids{}
	g.occ[32*occGridSize+33] = occlusionFull
	g.traceCell(32, 34, 0, 0, 8*2, 0)

	if g.res[32*occGridSize+34] != occlusionFull {
		t.Errorf("source toward -x: neighbour col 33 is blocked, got %d", g.res[32*occGridSize+34])
	}

	g = &occGrids{}
	g.occ[32*occGridSize+35] = occlusionFull // neighbour toward the source (source at x=+)
	g.traceCell(32, 34, 100, 0, 8*2, 0)

	if g.res[32*occGridSize+34] != occlusionFull {
		t.Errorf("mode 1 toward +x: got %d", g.res[32*occGridSize+34])
	}

	// mode 2 blend: major axis x, ratio 128: halfway between blocked (16) and open (0) = 8
	g = &occGrids{}
	g.occ[32*occGridSize+35] = occlusionFull
	g.traceCell(32, 34, 100, 50, 0, 0)

	if got := g.res[32*occGridSize+34]; got != 8 {
		t.Errorf("mode 2 blend: got %d want 8", got)
	}

	// the target at the source position is not computed
	g = &occGrids{}
	g.traceCell(32, 32, 4, 4, 4, 4)

	if g.res[32*occGridSize+32] != 0 {
		t.Error("zero distance must leave the cell alone")
	}
}

func TestPaintOccludedGolden(t *testing.T) {
	radial, occl := New(), New()
	base := Cell{Intensity: 10, R: 255, G: 255, B: 255}

	hx, hy := 100, 100
	wall := func(x, y int) bool { return x == hx+3 && y >= hy-10 && y <= hy+10 }

	src := func() *Source {
		s := NewSource(8, 255, 255, 255, 255)
		s.SetPosition(float64(hx), float64(hy))

		return s
	}

	radial.Rebuild(hx, hy, base, []*Source{src()})

	os := src()
	os.Occluded = true
	occl.Blocked = wall
	occl.Rebuild(hx, hy, base, []*Source{os})

	cell := func(m *Map, dx, dy int) uint8 { return m.At(hx+dx-m.OriginX, hy+dy-m.OriginY).Intensity }

	// in front of and on the wall: identical to the radial light
	for _, dx := range []int{-3, 0, 2, 3} {
		if cell(radial, dx, 0) != cell(occl, dx, 0) {
			t.Errorf("dx=%d: radial %d occluded %d", dx, cell(radial, dx, 0), cell(occl, dx, 0))
		}
	}

	// behind the wall: the radial light still reaches, the occluded one leaves the base
	if cell(radial, 5, 0) <= 10 {
		t.Fatal("test setup: radial light must reach 5 subtiles")
	}

	if cell(occl, 5, 0) != 10 || cell(occl, 6, 2) != 10 {
		t.Errorf("light leaked through the wall: %d %d", cell(occl, 5, 0), cell(occl, 6, 2))
	}

	// without a Blocked func an occluded source paints radial
	free := New()
	fs := src()
	fs.Occluded = true
	free.Rebuild(hx, hy, base, []*Source{fs})

	if cell(free, 5, 0) != cell(radial, 5, 0) {
		t.Error("nil Blocked must fall back to the radial painter")
	}
}
