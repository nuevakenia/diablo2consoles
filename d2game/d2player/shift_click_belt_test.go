package d2player

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func TestIsShiftBeltClick(t *testing.T) {
	cases := []struct {
		mod                   d2enum.KeyMod
		cursorEmpty, beltable bool
		want                  bool
	}{
		{d2enum.KeyModShift, true, true, true},
		{d2enum.KeyModShift, false, true, false},
		{d2enum.KeyModShift, true, false, false},
		{0, true, true, false},
		{d2enum.KeyModControl, true, true, false},
	}

	for _, c := range cases {
		if got := IsShiftBeltClick(c.mod, c.cursorEmpty, c.beltable); got != c.want {
			t.Errorf("%+v: got %v", c, got)
		}
	}
}
