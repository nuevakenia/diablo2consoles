package d2trade

import "testing"

// 0x62a9b0: an entry is "missing charges" when current (low byte) < max (next byte).
func TestHasMissingCharges(t *testing.T) {
	full := StatEntry{ID: StatChargedSkill, Value: 5 | 5<<8}
	part := StatEntry{ID: StatChargedSkill, Value: 3 | 5<<8}

	if HasMissingCharges(nil) || HasMissingCharges([]StatEntry{full}) {
		t.Fatal("full or no charges must not count")
	}

	if !HasMissingCharges([]StatEntry{full, part}) {
		t.Fatal("one partly used entry must count")
	}
}
