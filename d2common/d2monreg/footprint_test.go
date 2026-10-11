package d2monreg

import "testing"

func TestFootprint(t *testing.T) {
	for _, c := range []struct {
		size  int
		cells int
		all   bool
	}{{0, 1, false}, {1, 1, false}, {2, 5, false}, {3, 9, false}, {4, 0, true}, {7, 0, true}} {
		cells, all := Footprint(c.size)
		if len(cells) != c.cells || all != c.all {
			t.Errorf("size %d: %d cells, blockAll %v", c.size, len(cells), all)
		}
	}
}
