package d2drop

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// Wiki round 3 (w41): affix counts and gold. Community figures are recalled
// (WIKI-ONLY); the engine side is exe-confirmed (rare table 6e45a8, jewel
// base at 5bf94a, ITEMGEN_ApplyGoldFindBonus 556a10).

type fixedRoll uint32

func (f fixedRoll) Roll(n int32) uint32 {
	if n < 1 {
		return 0
	}

	return uint32(f) % uint32(n)
}

func (f fixedRoll) Chance() bool { return f&1 == 1 }

func TestW41RareAffixCountDistribution(t *testing.T) {
	counts := map[int]int{}

	for r := 0; r < 8; r++ {
		counts[RareAffixCount(fixedRoll(r), false)]++
	}

	// 3:1/8, 4:2/8, 5:3/8, 6:2/8 (mean 4.75): never fewer than 3 or more than 6.
	want := map[int]int{3: 1, 4: 2, 5: 3, 6: 2}
	for k, v := range want {
		if counts[k] != v {
			t.Errorf("rare affix count %d: %d/8, want %d/8", k, counts[k], v)
		}
	}

	if RareAffixCount(fixedRoll(0), true) != 3 || RareAffixCount(fixedRoll(1), true) != 4 {
		t.Error("rare jewel must have 3 or 4 affixes")
	}
}

func TestW41GoldAmount(t *testing.T) {
	for _, ilvl := range []int{1, 5, 30, 85} {
		lo, hi := 1<<30, 0

		for s := uint32(1); s < 4000; s++ {
			a := GoldAmount(&d2rand.Seed{Lo: s, Hi: 0x29a}, ilvl, 0)
			if a < lo {
				lo = a
			}

			if a > hi {
				hi = a
			}
		}
		// amount = ilvl + rand(5*ilvl): within [ilvl, 6*ilvl-1].
		if lo < ilvl || hi > 6*ilvl-1 {
			t.Errorf("ilvl %d: amount range %d..%d outside %d..%d", ilvl, lo, hi, ilvl, 6*ilvl-1)
		}
	}

	if got := GoldAmount(fixedRoll(0), 0, 0); got != 1 {
		t.Errorf("zero ilvl gold = %d, want 1", got)
	}

	if got := GoldAmount(fixedRoll(7), 40, 123); got != 123 {
		t.Errorf("forced amount = %d, want 123", got)
	}
}

func TestW41GoldFindAndMul(t *testing.T) {
	cases := []struct{ gold, kill, owner, want int }{
		{100, 0, 0, 100}, {100, 100, 0, 200}, {100, 250, 0, 350}, {101, 50, 0, 151},
		{100, 50, 30, 180}, {1, 99, 0, 1}, {7, 15, 0, 8},
	}
	for _, c := range cases {
		if got := ApplyGoldFind(c.gold, c.kill, c.owner); got != c.want {
			t.Errorf("gold find %+v = %d", c, got)
		}
	}

	if ScaleGoldMul(100, 256) != 100 || ScaleGoldMul(100, 512) != 200 || ScaleGoldMul(100, 128) != 50 {
		t.Error("gld mul=N is amount*N>>8")
	}
}

func TestW41MagicFindSources(t *testing.T) {
	if MagicFindOf(1, 40, 60) != 100 || MagicFindOf(0, 40, 0) != 40 || MagicFindOf(4, 40, 60) != 0 {
		t.Error("magic find is the dropper's plus its owner's, players and monsters only")
	}
}
