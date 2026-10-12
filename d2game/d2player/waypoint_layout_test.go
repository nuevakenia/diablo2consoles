package d2player

import "testing"

func TestWaypointRowGolden(t *testing.T) {
	tests := []struct {
		i          int
		hitY, h    int
		textX      int
		textBottom int
	}{
		{0, 121, 29, 160, 144},
		{3, 229, 29, 160, 249},
		{4, 266, 29, 160, 284},
		{8, 410, 29, 160, 424},
	}

	for _, c := range tests {
		rs, ok := Mode800.WaypointRowRects(c.i)
		if !ok {
			t.Fatalf("row %d", c.i)
		}

		if rs[0].X != 98 || rs[0].W != 279 || rs[0].Y != c.hitY || rs[0].H != c.h {
			t.Errorf("row %d hit %v", c.i, rs[0])
		}

		if rs[1].X != c.textX || rs[1].Y != c.textBottom {
			t.Errorf("row %d text %v", c.i, rs[1])
		}
	}

	if _, ok := Mode800.WaypointRowRects(9); ok {
		t.Error("row 9 exists")
	}

	b, h := Mode800.WaypointClose()
	if b.String() != "waypoint close 353 445 32 32" || h.String() != "waypoint close_hover 353 447 36 34" {
		t.Errorf("close %v %v", b, h)
	}
}
