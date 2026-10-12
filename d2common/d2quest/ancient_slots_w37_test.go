package d2quest

import "testing"

// VERIFIED w37 (0x589f10): statue object 474 -> slot 1, 475 -> slot 0, 476 -> slot 2.
func TestAncientSlotForStatue(t *testing.T) {
	want := map[int]int{StatueObject3: 1, StatueObject1: 0, StatueObject2: 2}
	for row, slot := range want {
		if got, ok := AncientSlotForStatueObject(row); !ok || got != slot {
			t.Errorf("row %d: slot %d ok=%v, want %d", row, got, ok, slot)
		}
	}

	if _, ok := AncientSlotForStatueObject(1); ok {
		t.Error("an unrelated row has a slot")
	}

	seen := map[int]bool{}

	for _, c := range []int{NPCAncientStatue1, NPCAncientStatue2, NPCAncientStatue3} {
		slot, ok := AncientSlotForStatueMonster(c)
		if !ok || seen[slot] {
			t.Errorf("statue class %d: slot %d ok=%v", c, slot, ok)
		}

		seen[slot] = true
	}
}
