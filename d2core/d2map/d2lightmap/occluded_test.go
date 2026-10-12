package d2lightmap

import "testing"

func TestRayOcclusion(t *testing.T) {
	wall := func(x, y int) bool { return x == 5 }

	tests := []struct {
		name           string
		x0, y0, x1, y1 int
		want           int
	}{
		{"clear", 0, 0, 4, 0, 0},
		{"wall cell itself is lit", 0, 0, 5, 0, 0},
		{"behind the wall", 0, 0, 7, 0, occlusionFull},
		{"diagonal behind", 0, 0, 8, 4, occlusionFull},
		{"other side, going left", 9, 0, 3, 0, occlusionFull},
		{"neighbour", 4, 0, 5, 0, 0},
		{"same cell", 3, 3, 3, 3, 0},
	}

	for _, tc := range tests {
		if got := rayOcclusion(tc.x0, tc.y0, tc.x1, tc.y1, wall); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
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
