package d2missile

import "testing"

func TestCellFootprint(t *testing.T) {
	if got := CellFootprint(10, 20, 1); len(got) != 1 || got[0] != [2]int{10, 20} {
		t.Errorf("size 1: %v", got)
	}

	plus := CellFootprint(10, 20, 2)
	want := map[[2]int]bool{{9, 20}: true, {10, 20}: true, {11, 20}: true, {10, 19}: true, {10, 21}: true}

	if len(plus) != 5 {
		t.Fatalf("size 2 cells: %v", plus)
	}

	for _, c := range plus {
		if !want[c] {
			t.Errorf("unexpected cell %v", c)
		}
	}

	if got := CellFootprint(10, 20, 3); len(got) != 9 || got[0] != [2]int{9, 19} || got[8] != [2]int{11, 21} {
		t.Errorf("size 3: %v", got)
	}

	if CellFootprint(1, 1, 0) != nil || CellFootprint(1, 1, 4) != nil {
		t.Error("other sizes stamp nothing")
	}
}
