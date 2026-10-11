package d2gamescreen

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2game/d2player"
)

type stubItem struct{ code string }

func (s stubItem) InventoryGridSize() (int, int) { return 1, 1 }
func (s stubItem) GetItemCode() string           { return s.code }
func (s stubItem) InventoryGridSlot() (int, int) { return 0, 0 }
func (s stubItem) SetInventoryGridSlot(int, int) {}
func (s stubItem) GetItemDescription() []string  { return nil }

type stubThief struct {
	item   d2player.InventoryItem
	rolled uint32
}

func (s *stubThief) StealBeltItem(step func() uint32) (d2player.InventoryItem, bool) {
	s.rolled = step()
	if s.rolled%100 < 30 || s.item == nil {
		return nil, false
	}

	return s.item, true
}

func TestStealFromBeltDropsTheItem(t *testing.T) {
	var dropped []string

	drop := func(c string) { dropped = append(dropped, c) }

	th := &stubThief{item: stubItem{"hp1 "}}
	if stealFromBelt(th, func() uint32 { return 10 }, drop) || len(dropped) != 0 {
		t.Fatal("a low roll must not steal")
	}

	if !stealFromBelt(th, func() uint32 { return 99 }, drop) || len(dropped) != 1 || dropped[0] != "hp1" {
		t.Fatalf("dropped %v", dropped)
	}

	t.Setenv("OD2_BELTSTEAL_ROLL", "77")

	if beltStealStep()() != 77 {
		t.Fatal("test roll env ignored")
	}
}
