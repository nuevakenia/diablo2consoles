package d2skill

import "testing"

func TestFanScale(t *testing.T) {
	for _, c := range []struct{ vx, vy, wx, wy int }{
		{0, 0, 0, 0},
		{1, 0, 4, 0}, // 1 -> x4 = 16, no second step
		{1, 1, 4, 4}, // 2 -> x4 -> 32
		{2, 0, 4, 0}, // 4 is not below 4, but below 16: x2
		{3, 0, 6, 0}, // 9 -> x2 = 36
		{2, 2, 4, 4}, // 8 -> x2 = 32
		{4, 0, 4, 0}, // 16 untouched
		{10, -3, 10, -3},
	} {
		if x, y := FanScale(c.vx, c.vy); x != c.wx || y != c.wy {
			t.Errorf("scale (%d,%d) = (%d,%d), want (%d,%d)", c.vx, c.vy, x, y, c.wx, c.wy)
		}
	}
}

func TestFanPerpendicular(t *testing.T) {
	for _, c := range []struct{ vx, vy, wx, wy int }{
		{10, 0, 0, -1}, // aiming along +x: the row runs along y
		{-10, 0, 0, 1},
		{0, 10, 1, 0},
		{10, 10, 1, -1}, // a diagonal aim: a diagonal row
		{30, 3, 0, -1},  // 3 and -30 halve to 1 and -1 only after the length drops to 3 or less
		{0, 0, 0, 0},
	} {
		if x, y := FanPerpendicular(c.vx, c.vy); x != c.wx || y != c.wy {
			t.Errorf("perp (%d,%d) = (%d,%d), want (%d,%d)", c.vx, c.vy, x, y, c.wx, c.wy)
		}
	}
}

func TestFanDestinations(t *testing.T) {
	sx, sy := FanSpacing(30, 0)
	if sx != 0 || sy != -1 {
		t.Fatalf("spacing %d,%d", sx, sy)
	}

	// Shock Wave: five points centred on the aim point
	got := FanDestinations(30, 0, 5, sx, sy)
	for i, want := range []int{2, 1, 0, -1, -2} {
		if got[i] != [2]int{30, want} {
			t.Errorf("point %d = %v", i, got[i])
		}
	}

	// six: the exe halves 6 * s towards zero, the row is not symmetric
	got = FanDestinations(30, 0, 6, sx, sy)
	if got[0][1] != 3 || got[5][1] != -2 {
		t.Errorf("six points %v", got)
	}

	// a diagonal aim spaces the row along the other diagonal
	dx, dy := FanSpacing(10, 10)
	got = FanDestinations(10, 10, 3, dx, dy)
	if got[0] != [2]int{9, 11} || got[2] != [2]int{11, 9} {
		t.Errorf("diagonal %v", got)
	}

	if FanDestinations(5, 5, 0, 1, 0) != nil {
		t.Error("zero missiles")
	}
}
