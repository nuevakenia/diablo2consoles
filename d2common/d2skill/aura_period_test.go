package d2skill

import "testing"

func TestNextPeriodicTick(t *testing.T) {
	for _, c := range []struct{ frame, period, want int }{
		{1, 50, 51}, {2, 50, 51}, {51, 50, 101}, {50, 50, 51}, {100, 25, 101}, {101, 25, 126}, {7, 3, 11}, {7, 5, 11},
	} {
		if got := NextPeriodicTick(c.frame, c.period); got != c.want {
			t.Errorf("NextPeriodicTick(%d,%d)=%d want %d", c.frame, c.period, got, c.want)
		}
	}
}

func TestPeriodFloor(t *testing.T) {
	for in, want := range map[int]int{-3: 5, 0: 5, 5: 5, 6: 6, 50: 50} {
		if PeriodFloor(in) != want {
			t.Errorf("PeriodFloor(%d)", in)
		}
	}
}
