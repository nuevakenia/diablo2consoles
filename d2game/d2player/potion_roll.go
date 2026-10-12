package d2player

import (
	"os"
	"sync"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// potionRoll is the draw behind the vitality / energy doubling of a drunk
// healing or mana potion (d2inventory.PotionDoubles). The exe draws from the
// drinker's unit seed; the hero has no such seed here, so a process-wide
// seed stands in: it is the exe's own generator (d2rand.Seed: the same multiplier/carry step and the mask or modulo
// roll; VERIFIED against RAND_RollSeedModulo 0x457b60 / RAND_RollSeedBounded 0x457b10, called by the potion use at
// 0x5bbf70 as roll(stat) then roll(100), doubled when the second is below half of the first), so the two draws of PotionDoubles advance it exactly like
// the exe advances a unit seed. Only the start value of a unit seed (set at unit creation from the game seed) is not
// reproducible here, so the sequence of results is a fresh one per process. OD2_POTION_ROLL=off turns the doubling off,
// for numbers that must not vary.
var (
	potionRollMu  sync.Mutex
	potionRollRng = d2rand.New(0x5eed)
)

func potionRoll(n int) int {
	if os.Getenv("OD2_POTION_ROLL") == "off" {
		return 0
	}

	potionRollMu.Lock()
	defer potionRollMu.Unlock()

	return int(potionRollRng.Roll(int32(n)))
}
