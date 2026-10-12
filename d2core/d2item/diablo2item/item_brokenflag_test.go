package diablo2item

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

func TestBrokenFlag(t *testing.T) {
	it := &Item{attributes: &itemAttributes{durable: true, currentDurability: 5, durability: minMaxEnhanceable{max: 12}}}

	if it.HasBrokenFlag() {
		t.Fatal("fresh item flagged")
	}

	it.SetDurability(0) // ITEM_BreakEquippedItem
	if !it.HasBrokenFlag() || it.ExeFlags()&ExeFlagBroken == 0 {
		t.Fatal("0x100 not set at 0 durability")
	}

	it.SetDurability(3) // ITEM_RestoreBrokenItem
	if it.HasBrokenFlag() {
		t.Fatal("flag survived the repair")
	}

	// a saved flag counts, 0x4000 as well, and a repair clears 0x100 only
	it.SetOrigin(&d2s.Item{Flags: ExeFlagBroken | ExeFlagNotEquippable})
	if !it.HasBrokenFlag() {
		t.Fatal("saved flags ignored")
	}

	it.SetDurability(4)
	if it.ExeFlags()&ExeFlagBroken != 0 || !it.HasBrokenFlag() {
		t.Fatalf("flags after repair %#x", it.ExeFlags())
	}

	// a loose item with no durability and no flag is not broken by this test
	loose := &Item{attributes: &itemAttributes{durable: true, currentDurability: 0, durability: minMaxEnhanceable{max: 12}}}
	if loose.HasBrokenFlag() {
		t.Fatal("durability alone must not set the flag")
	}
}
