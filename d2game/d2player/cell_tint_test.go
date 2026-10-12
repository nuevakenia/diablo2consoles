package d2player

import (
	"image/color"
	"testing"
)

func TestTintTable(t *testing.T) {
	want := [][3]uint8{{0x80, 0, 0}, {0, 0x80, 0}, {0, 0, 0x80}, {0x80, 0x80, 0}, {0x80, 0x40, 0x40}}
	for m := TintBad; m < tintModes; m++ {
		if tintRGB[m] != want[m] {
			t.Errorf("mode %d = %v, want %v (UI_InitInventoryHighlightColors 0x47f8b0)", m, tintRGB[m], want[m])
		}
	}

	if got := TintColor(TintBad); got != (color.RGBA{R: 0x40, A: 0x80}) {
		t.Errorf("TintColor(bad) = %v", got)
	}

	if got := TintColor(99); got != (color.RGBA{}) {
		t.Errorf("TintColor(99) = %v", got)
	}
}

func TestGridCellMode(t *testing.T) {
	ok := CellState{RequirementsMet: true, Identified: true}

	cases := []struct {
		name string
		mod  func(*CellState)
		want TintMode
	}{
		{"identified and usable", func(*CellState) {}, TintUsable},
		{"requirements fail", func(s *CellState) { s.RequirementsMet = false }, TintBad},
		{"disabled flag", func(s *CellState) { s.Disabled = true }, TintBad},
		{"busy", func(s *CellState) { s.Busy = true }, TintBad},
		{"broken", func(s *CellState) { s.Broken = true }, TintBad},
		{"unidentified", func(s *CellState) { s.Identified = false }, TintUnidentified},
		{"unidentified and failing: bad wins", func(s *CellState) { s.Identified, s.RequirementsMet = false, false }, TintBad},
		{"highlightable quest item", func(s *CellState) { s.QuestHighlight = true }, TintBad},
	}
	for _, c := range cases {
		s := ok
		c.mod(&s)

		if got := GridCellMode(s); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

func TestHoverAndSlotModes(t *testing.T) {
	if HoverMode(false, false) != TintHover || HoverMode(true, false) != TintBad || HoverMode(true, true) != TintHover {
		t.Error("HoverMode")
	}

	if m, drawn := EquipSlotMode(CellState{RequirementsMet: true, Identified: true}); drawn {
		t.Errorf("a usable worn item has no rect, got %d", m)
	}

	if m, drawn := EquipSlotMode(CellState{RequirementsMet: true}); !drawn || m != TintUnidentified {
		t.Errorf("unidentified worn: %d %v", m, drawn)
	}

	if m, drawn := EquipSlotMode(CellState{Identified: true}); !drawn || m != TintBad {
		t.Errorf("failing worn: %d %v", m, drawn)
	}

	// the disabled flag is not tested for worn items
	if _, drawn := EquipSlotMode(CellState{RequirementsMet: true, Identified: true, Disabled: true}); drawn {
		t.Error("flag 4 is not tested by INV_DrawEquipSlots")
	}

	if SlotHighlightMode(true, true, false) != TintHover || SlotHighlightMode(true, false, false) != TintBad ||
		SlotHighlightMode(false, true, false) != TintBad || SlotHighlightMode(false, false, true) != TintHover {
		t.Error("SlotHighlightMode")
	}
}

func TestLowDurabilityWarning(t *testing.T) {
	cases := []struct {
		cur, max int
		want     bool
	}{{20, 100, true}, {21, 100, false}, {0, 10, true}, {2, 10, true}, {3, 10, false}, {5, 0, false}, {3, 19, true}, {4, 19, false}}
	for _, c := range cases {
		if got := LowDurabilityWarning(c.cur, c.max); got != c.want {
			t.Errorf("(%d,%d) = %v", c.cur, c.max, got)
		}
	}
}
