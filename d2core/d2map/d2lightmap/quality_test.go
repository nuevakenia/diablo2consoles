package d2lightmap

import "testing"

func TestQuality(t *testing.T) {
	tests := []struct {
		name             string
		toggle, raw, fps int
		prev             int
		elapsed          uint32
		want             int
	}{
		{"fast, toggle 0", 0, 0, 60, 0, 5000, 2},
		{"slow", 0, 0, 5, 2, 5000, 0},
		{"mid", 0, 0, 11, 2, 5000, 1},
		{"13..15 keeps previous", 0, 0, 14, 2, 5000, 2},
		{"13..15 no previous", 0, 0, 14, 0, 5000, 1},
		{"toggle set, raw 0", 1, 0, 60, 2, 5000, 0},
		{"toggle set, raw set caps at 1", 1, 3, 60, 2, 5000, 1},
		{"hysteresis holds", 0, 0, 60, 1, 1000, 1},
		{"hysteresis released", 0, 0, 60, 1, 2001, 2},
	}

	for _, tc := range tests {
		q := &QualityState{Level: tc.prev, TickMS: 10000}
		if got := q.Quality(tc.toggle, tc.raw, tc.fps, 10000+tc.elapsed); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestRadialCap(t *testing.T) {
	if r, _ := RadialCap(0, false, 18); r != 16 {
		t.Error("remote player cap")
	}

	if r, _ := RadialCap(0, true, 18); r != 18 {
		t.Error("own player uncapped")
	}

	if r, _ := RadialCap(1, false, 3); r != 8 {
		t.Error("monster 8")
	}

	if _, ok := RadialCap(3, false, 3); ok {
		t.Error("kind 3 skipped")
	}
}
