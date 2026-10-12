package d2monster

import "testing"

// TestWakeUniqueAndBoss pins the gate of MONAI_CheckSummonerWakeUp 0x5aed10
// (w27): besides the Summoner, a unit with the unique type flag (mask 8) or a
// boss class takes the one-shot 20 frame wake-up; an ordinary unit does not.
func TestWakeUniqueAndBoss(t *testing.T) {
	for _, c := range []struct {
		name   string
		unique bool
		boss   bool
		shouts int
	}{{"plain", false, false, 0}, {"unique", true, false, 1}, {"boss", false, true, 1}} {
		w := newFake(8, false)
		b := brainAt(profile("Skeleton", 50, 10, 50, 50))
		b.Unique = c.unique
		b.Profile.Boss = c.boss

		Tick(w, b)

		if w.shouts != c.shouts {
			t.Fatalf("%s: shouts %d want %d", c.name, w.shouts, c.shouts)
		}

		if c.shouts == 1 && (b.Wake != 20 || !b.WakeShouted) {
			t.Fatalf("%s: wake %d shouted %v", c.name, b.Wake, b.WakeShouted)
		}
	}
}
