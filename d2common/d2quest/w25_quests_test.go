package d2quest

import "testing"

// Malah hands out the defrost potion once per game (VERIFIED 0x587460: a counter in the node data), even when the
// first one was lost.
func TestMalahPotionOncePerGame(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelHarrogath)

	first := g.Hear(NPCMalah, 20127)
	if !effectCode(first, EffectGiveItem, ItemDefrostPotion) {
		t.Fatalf("first request gave no potion: %v", first)
	}

	second := g.Hear(NPCMalah, 20127)
	if effectCode(second, EffectGiveItem, ItemDefrostPotion) {
		t.Fatalf("second request gave another potion: %v", second)
	}
}
