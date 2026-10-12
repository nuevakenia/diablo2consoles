package d2player

import (
	"math/rand"
	"os"
	"sync"
)

// potionRoll is the draw behind the vitality / energy doubling of a drunk
// healing or mana potion (d2inventory.PotionDoubles). The exe draws from the
// drinker's unit seed; the hero has no such seed here, so a process-wide
// generator stands in (UNVERIFIED equivalence of the sequence, the odds are the
// same). OD2_POTION_ROLL=off turns the doubling off, for numbers that must not vary.
var (
	potionRollMu  sync.Mutex
	potionRollRng = rand.New(rand.NewSource(0x5eed)) //nolint:gosec // game rolls, not security
)

func potionRoll(n int) int {
	if os.Getenv("OD2_POTION_ROLL") == "off" {
		return 0
	}

	potionRollMu.Lock()
	defer potionRollMu.Unlock()

	return potionRollRng.Intn(n)
}
