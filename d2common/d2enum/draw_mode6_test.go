package d2enum

import "testing"

func TestMode6Weight(t *testing.T) {
	if w := Mode6Weight(0, 0, 0); w != 0 {
		t.Errorf("black: %v", w)
	}

	if w := Mode6Weight(1, 1, 1); w != 1 {
		t.Errorf("white: %v", w)
	}

	prev := -1.0

	for i := 0; i <= 10; i++ {
		v := float64(i) / 10
		w := Mode6Weight(v, v, v)

		if w < prev {
			t.Errorf("not monotone at %v", v)
		}

		prev = w
	}

	// measured buckets (brightness -> weight): 0.44 -> 0.54
	if w := Mode6Weight(0.44, 0.44, 0.44); w < 0.45 || w > 0.65 {
		t.Errorf("mid: %v", w)
	}

	if Mode6Weight(0, 1, 0) <= Mode6Weight(1, 0, 0) {
		t.Error("green should outweigh red")
	}
}
