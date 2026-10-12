package d2player

import "testing"

func TestReplenishSeconds(t *testing.T) {
	// frames = max(125, 2500/value+1) at 25 frames per second (0x560a70)
	for v, want := range map[int64]float64{30: 5, 100: 5, 20: 126.0 / 25, 21: 5, 10: 251.0 / 25, 1: 2501.0 / 25, 0: 0, -5: 0} {
		if got := replenishSeconds(v); got != want {
			t.Errorf("replenishSeconds(%d) = %v, want %v", v, got, want)
		}
	}
}
