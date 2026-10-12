package d2gamescreen

import "testing"

// The hero swings only inside the exact gate of COMBAT_RollAttackOutcome 0x57cc10 (bare hands: table distance <= 2).
func TestHeroInSwingReach(t *testing.T) {
	v := &Game{}

	cases := []struct {
		dx, dy int
		want   bool
	}{
		{0, 0, true}, {2, 0, true}, {3, 0, true}, {4, 0, false}, {2, 2, true}, {3, 3, false}, {7, 0, false}, {-3, 0, true},
	}

	for _, c := range cases {
		if got := v.heroInSwingReach(10, 10, 10+c.dx, 10+c.dy); got != c.want {
			t.Errorf("offset (%d,%d): got %v want %v", c.dx, c.dy, got, c.want)
		}
	}
}
