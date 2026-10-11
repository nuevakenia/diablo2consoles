package d2player

import "testing"

func TestShareHealPassesThroughTheHook(t *testing.T) {
	g := &GameControls{}

	if got := g.shareHeal(100); got != 100 {
		t.Fatalf("without a hook: %d", got)
	}

	var seen []int

	g.SetHealShare(func(heal int) int {
		seen = append(seen, heal)

		return heal - heal/4 // the golem took a quarter
	})

	if got := g.shareHeal(100); got != 75 {
		t.Fatalf("with the golem share: %d", got)
	}

	if got := g.shareHeal(0); got != 0 || len(seen) != 1 {
		t.Fatalf("a zero heal must not reach the hook: got %d calls %v", got, seen)
	}
}
