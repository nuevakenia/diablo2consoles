package d2vendor

import "testing"

func TestStockPagesAreSeparateGrids(t *testing.T) {
	s := NewStock()
	a := &Item{Code: "a", W: 2, H: 3, Page: 0}
	b := &Item{Code: "b", W: 2, H: 3, Page: 1}
	n := &Item{Code: "n", W: 1, H: 1, Page: -1}

	if !s.Place(a) || !s.Place(b) || s.Place(n) {
		t.Fatal("placement")
	}

	if a.X != 0 || a.Y != 0 || b.X != 0 || b.Y != 0 {
		t.Fatalf("each page starts at the origin: %+v %+v", a, b)
	}

	if s.ItemAtPage(1, 1, 2) != b || s.ItemAtPage(0, 1, 2) != a || s.ItemAtPage(2, 0, 0) != nil {
		t.Fatal("ItemAtPage")
	}

	s.Remove(a)

	c := &Item{Code: "c", W: 2, H: 3, Page: 0}
	if !s.Place(c) || c.X != 0 || c.Y != 0 {
		t.Fatal("page 0 cell not freed")
	}
}
