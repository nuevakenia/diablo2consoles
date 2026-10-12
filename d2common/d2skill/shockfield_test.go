package d2skill

import "testing"

func TestShockFieldPoints(t *testing.T) {
	if pts := ShockFieldPoints(50, 60, 0, 3, 0, 0); pts != nil {
		t.Errorf("count 0: %v", pts)
	}

	// below two missiles or a radius under two: just the target position
	for _, c := range [][2]int{{1, 5}, {6, 1}, {1, 1}} {
		if pts := ShockFieldPoints(50, 60, c[0], c[1], 0, 0); len(pts) != 1 || pts[0] != [2]int{50, 60} {
			t.Errorf("count %d radius %d: %v", c[0], c[1], pts)
		}
	}

	// the points lie in the square, repeat for the same target x, and a point next to the caster is dropped
	pts := ShockFieldPoints(50, 60, 12, 4, 0, 0)
	if len(pts) != 12 {
		t.Fatalf("12 throws, got %d", len(pts))
	}

	for _, p := range pts {
		if p[0] < 46 || p[0] >= 54 || p[1] < 56 || p[1] >= 64 {
			t.Errorf("point %v outside the square", p)
		}
	}

	again := ShockFieldPoints(50, 60, 12, 4, 0, 0)
	for i := range pts {
		if pts[i] != again[i] {
			t.Fatal("not repeatable for the same seed")
		}
	}

	if near := ShockFieldPoints(50, 60, 200, 4, 50, 60); len(near) >= 200 {
		t.Errorf("a caster in the middle keeps every point: %d", len(near))
	}

	// RAND_RollSeedModulo with the DRLG_InitSeed pair: first step from (lo=7, hi=0x29a)
	s := newShockSeed(7)
	v := uint64(7)*0x6ac690c5 + 0x29a
	if got, want := s.modulo(8), int(uint32(v)%8); got != want {
		t.Errorf("modulo %d, want %d", got, want)
	}
}
