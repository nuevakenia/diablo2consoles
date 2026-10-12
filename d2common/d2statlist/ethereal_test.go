package d2statlist

import "testing"

// ITEM_ApplyEtherealFlagAndDamageBonus (0x660a40): (base*3)/2, integer division.
func TestEtherealRoundingMatchesExe(t *testing.T) {
	for in, want := range map[int]int{0: 0, 1: 1, 5: 7, 11: 16, 100: 150} {
		if got := EtherealDefense(in); got != want {
			t.Errorf("defense %d: got %d want %d", in, got, want)
		}

		if got := EtherealDamage(in); got != want {
			t.Errorf("damage %d: got %d want %d", in, got, want)
		}
	}
}
