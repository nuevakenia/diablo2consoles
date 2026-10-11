package d2gamescreen

import "testing"

// A defensive fight against a monster hovering at the edge of the threat radius ended after 0.1 s with no kill,
// over and over, and froze the walk of the barbarian in 9h (w21). Such a fight earns the walk a pause.
func TestQuickFight(t *testing.T) {
	tests := []struct {
		kills   int
		elapsed float64
		want    bool
	}{
		{0, 0.1, true}, {0, 0.99, true}, {0, 1.0, false}, {0, 12, false}, {1, 0.1, false}, {3, 6.2, false},
	}

	for _, tc := range tests {
		if got := quickFight(tc.kills, tc.elapsed); got != tc.want {
			t.Errorf("quickFight(%d, %.2f) = %v, want %v", tc.kills, tc.elapsed, got, tc.want)
		}
	}
}

func TestNextFruitless(t *testing.T) {
	c := 0
	for i := 1; i <= 3; i++ {
		if c = nextFruitless(c, 0); c != i {
			t.Fatalf("fight %d without a kill: count %d", i, c)
		}
	}

	if c = nextFruitless(c, 2); c != 0 {
		t.Errorf("a kill must reset the count, got %d", c)
	}
}
