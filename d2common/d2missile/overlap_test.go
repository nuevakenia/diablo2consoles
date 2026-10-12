package d2missile

import "testing"

func TestFootprintOverlap(t *testing.T) {
	// shape strings: rows dy -2..2, columns dx -2..2 around the unit, 'x' = hit
	shapes := map[[2]int][5]string{
		{1, 1}: {".....", ".....", "..x..", ".....", "....."},
		{1, 2}: {".....", "..x..", ".xxx.", "..x..", "....."},
		{2, 1}: {".....", "..x..", ".xxx.", "..x..", "....."},
		{1, 3}: {".....", ".xxx.", ".xxx.", ".xxx.", "....."},
		{3, 1}: {".....", ".xxx.", ".xxx.", ".xxx.", "....."},
		{2, 2}: {"..x..", ".xxx.", "xxxxx", ".xxx.", "..x.."},
		{2, 3}: {".xxx.", "xxxxx", "xxxxx", "xxxxx", ".xxx."},
		{3, 2}: {".xxx.", "xxxxx", "xxxxx", "xxxxx", ".xxx."},
		{3, 3}: {"xxxxx", "xxxxx", "xxxxx", "xxxxx", "xxxxx"},
	}

	for k, rows := range shapes {
		for r, row := range rows {
			for c, ch := range row {
				got := FootprintOverlap(k[0], k[1], c-2, r-2, 0, 0)
				if want := ch == 'x'; got != want {
					t.Errorf("missile %d unit %d offset (%d,%d): %v, want %v", k[0], k[1], c-2, r-2, got, want)
				}
			}
		}
	}

	if FootprintOverlap(1, 0, 0, 0, 0, 0) || FootprintOverlap(0, 1, 0, 0, 0, 0) || FootprintOverlap(4, 1, 0, 0, 0, 0) {
		t.Error("size 0 / 4 should never overlap")
	}

	if !FootprintOverlap(3, 9, 2, 2, 0, 0) { // unit size caps at 3
		t.Error("unit size is capped at 3")
	}
}
