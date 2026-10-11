package d2player

import "testing"

func TestMiniPanelPlacement(t *testing.T) {
	cases := []struct {
		left, right bool
		want        MiniPanelSlot
	}{
		{false, false, MiniPanelDefault},
		{true, false, MiniPanelShiftRight},
		{false, true, MiniPanelShiftLeft},
		{true, true, MiniPanelHidden},
	}

	for _, c := range cases {
		if got := MiniPanelPlacement(c.left, c.right); got != c.want {
			t.Errorf("left=%v right=%v: got %v want %v", c.left, c.right, got, c.want)
		}
	}
}

// golden values read from UI_DrawMiniPanel (0x47b510): strip x relative to W/2.
func TestMiniPanelStripX(t *testing.T) {
	cases := []struct {
		slot   MiniPanelSlot
		single bool
		want   int
	}{
		{MiniPanelDefault, true, -77},
		{MiniPanelDefault, false, -87},
		{MiniPanelShiftRight, true, 54},
		{MiniPanelShiftRight, false, 33},
		{MiniPanelShiftLeft, true, -205},
		{MiniPanelShiftLeft, false, -205},
	}

	for _, c := range cases {
		if got := MiniPanelStripX(c.slot, c.single); got != c.want {
			t.Errorf("slot=%v single=%v: got %d want %d", c.slot, c.single, got, c.want)
		}
	}
}

// the shift distances of the widget code must land on the same strip x as the exe's table.
func TestMiniPanelShiftsMatchStripX(t *testing.T) {
	for _, single := range []bool{true, false} {
		m := &miniPanel{isSinglePlayer: single}
		def := MiniPanelStripX(MiniPanelDefault, single)

		if got := def + m.shiftRight(); got != MiniPanelStripX(MiniPanelShiftRight, single) {
			t.Errorf("single=%v right: %d", single, got)
		}

		if got := def - m.shiftLeft(); got != MiniPanelStripX(MiniPanelShiftLeft, single) {
			t.Errorf("single=%v left: %d", single, got)
		}
	}
}
