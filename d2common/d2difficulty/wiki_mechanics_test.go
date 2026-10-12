package d2difficulty

import "testing"

// Wiki cross-check (wave 30). Wiki (recalled): resist penalty -40 nightmare,
// -100 hell (-20/-50 classic); death penalty 0/5/10 percent.
func TestWikiDifficultyPenalties(t *testing.T) {
	for l, want := range map[Level]int{0: 0, 1: -40, 2: -100} {
		if got := ResistPenalty(false, l); got != want {
			t.Errorf("LoD penalty %v: %d want %d", l, got, want)
		}
	}

	for l, want := range map[Level]int{0: 0, 1: -20, 2: -50} {
		if got := ResistPenalty(true, l); got != want {
			t.Errorf("classic penalty %v: %d want %d", l, got, want)
		}
	}

	for l, want := range map[Level]int{0: 0, 1: 5, 2: 10} {
		if got := DeathExpPenalty(l); got != want {
			t.Errorf("death penalty %v: %d want %d", l, got, want)
		}
	}
}
