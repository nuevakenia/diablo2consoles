package d2player

import "testing"

type stubBeltItem struct{ code string }

func (s *stubBeltItem) InventoryGridSize() (int, int) { return 1, 1 }
func (s *stubBeltItem) GetItemCode() string           { return s.code }
func (s *stubBeltItem) InventoryGridSlot() (int, int) { return 0, 0 }
func (s *stubBeltItem) SetInventoryGridSlot(int, int) {}
func (s *stubBeltItem) GetItemDescription() []string  { return nil }

func TestBeltPanelSteal(t *testing.T) {
	roll := func(v uint32) func() uint32 { return func() uint32 { return v } }

	newBelt := func() *BeltPanel {
		b := &BeltPanel{}
		b.items[1] = &stubBeltItem{"hp1"} // column 1, front
		b.items[5] = &stubBeltItem{"hp2"} // column 1, second row
		b.items[2] = &stubBeltItem{"mp1"}

		return b
	}

	// roll below 30: nothing happens
	b := newBelt()
	if it, ok := b.Steal(roll(29), true); ok || it != nil || b.items[1] == nil {
		t.Fatal("a roll of 29 must not steal")
	}

	// busy cursor: nothing is stolen
	if _, ok := b.Steal(roll(30), false); ok || b.items[1] == nil {
		t.Fatal("a busy cursor must stop the theft")
	}

	// first occupied cell (cell 1) goes; column 1 compacts toward the front row
	it, ok := b.Steal(roll(30), true)
	if !ok || it.GetItemCode() != "hp1" {
		t.Fatalf("stolen %v %v", it, ok)
	}

	if b.items[1] == nil || b.items[1].GetItemCode() != "hp2" || b.items[5] != nil || b.items[2] == nil {
		t.Fatal("column 1 not compacted toward row 0")
	}
}
