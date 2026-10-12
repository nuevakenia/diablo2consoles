package d2skill

import "testing"

func TestIronMaidenDamage(t *testing.T) {
	for _, tc := range []struct{ dealt, pct, want int }{
		{10, 225, 22}, {100, 200, 200}, {1, 200, 2}, {0, 200, 0}, {10, 0, 0}, {-5, 200, 0}, {7, 25, 1},
	} {
		if got := IronMaidenDamage(tc.dealt, tc.pct); got != tc.want {
			t.Errorf("IronMaidenDamage(%d,%d) = %d, want %d", tc.dealt, tc.pct, got, tc.want)
		}
	}
}

func TestLifeTapHeal(t *testing.T) {
	for _, tc := range []struct {
		dealt, pct, life, max, want int
		ok                          bool
	}{
		{20, 50, 50, 100, 60, true},
		{400, 50, 50, 100, 100, true}, // clamped to the maximum
		{20, 50, 0, 100, 0, false},    // dead attacker
		{20, 0, 50, 100, 50, false},   // curse without a percent
		{20, 50, 50, 0, 50, false},    // no maximum life
		{0, 50, 50, 100, 50, false},   // no damage
	} {
		got, ok := LifeTapHeal(tc.dealt, tc.pct, tc.life, tc.max)
		if got != tc.want || ok != tc.ok {
			t.Errorf("LifeTapHeal(%d,%d,%d,%d) = %d,%v want %d,%v", tc.dealt, tc.pct, tc.life, tc.max, got, ok, tc.want, tc.ok)
		}
	}
}
