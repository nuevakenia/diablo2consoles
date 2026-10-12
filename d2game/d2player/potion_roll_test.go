package d2player

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// The potion roll is the exe's seed step: two draws per doubling check (stat, then 100), as at 0x5bbf70.
func TestPotionRollUsesExeSeed(t *testing.T) {
	potionRollMu.Lock()
	potionRollRng = d2rand.New(0x5eed)
	potionRollMu.Unlock()

	ref := d2rand.New(0x5eed)

	for _, n := range []int{25, 100, 8, 1, 60} {
		if got, want := potionRoll(n), int(ref.Roll(int32(n))); got != want {
			t.Fatalf("n=%d got %d want %d", n, got, want)
		}
	}
}
