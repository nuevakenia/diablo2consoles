package d2player

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func TestStatSpendCount(t *testing.T) {
	cases := []struct {
		mod       d2enum.KeyMod
		remaining int
		want      int
	}{
		{0, 0, 0}, {0, 7, 1}, {d2enum.KeyModShift, 7, 5}, {d2enum.KeyModShift, 3, 3},
		{d2enum.KeyModControl, 12, 12}, {d2enum.KeyModControl | d2enum.KeyModShift, 12, 12}, {d2enum.KeyModShift, -1, 0},
	}
	for _, c := range cases {
		if got := StatSpendCount(c.mod, c.remaining); got != c.want {
			t.Errorf("mod %v remaining %d: got %d want %d", c.mod, c.remaining, got, c.want)
		}
	}
}

func TestStatValueColour(t *testing.T) {
	if StatValueColour(10, 10) != StatColourNormal || StatValueColour(11, 10) != StatColourIncreased ||
		StatValueColour(9, 10) != StatColourDecreased {
		t.Fatal("colour rule")
	}
}
