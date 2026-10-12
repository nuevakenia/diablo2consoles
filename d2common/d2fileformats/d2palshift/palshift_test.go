package d2palshift

import "testing"

func TestLoadAndApply(t *testing.T) {
	if _, err := Load(make([]byte, 100)); err != ErrSize {
		t.Fatalf("size check: %v", err)
	}

	data := make([]byte, Rows*RowSize)
	for r := 0; r < Rows; r++ {
		for i := 0; i < RowSize; i++ {
			data[r*RowSize+i] = byte(i) // identity
		}
	}

	data[3*RowSize+10] = 200 // row 3 sends index 10 to 200

	tab, err := Load(data)
	if err != nil {
		t.Fatal(err)
	}

	var pal [RowSize][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(i), uint8(i)}
	}

	row, ok := tab.Row(3)
	if !ok {
		t.Fatal("row 3")
	}

	out := Apply(&pal, row)
	if out[10] != pal[200] || out[11] != pal[11] {
		t.Errorf("remap: %v %v", out[10], out[11])
	}

	for _, n := range []int{-1, 8, 100} {
		if _, ok := tab.Row(n); ok {
			t.Errorf("row %d should not exist", n)
		}
	}
}
