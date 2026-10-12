package d2mapentity

import "testing"

func TestWobblePathPoints(t *testing.T) {
	if WobblePathPoints([][2]int{{1, 1}}) != nil {
		t.Fatal("single node")
	}

	p := WobblePathPoints([][2]int{{1, 1}, {3, 1}, {5, 2}})
	if len(p) != 2 || p[0].X() != 3 || p[1].Y() != 2 {
		t.Fatalf("%v", p)
	}
}
