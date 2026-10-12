package d2path

import "testing"

// Wave 31: stat 0x43 of a worn armor piece is -speed (ITEMGEN_InitItemBaseStats 0x555b10, VERIFIED), so heavy body
// armor and large shields slow walking and running; the faster run/walk term is diminished with K 150.
func TestArmorSpeedSlowsMovement(t *testing.T) {
	b := PlayerBase(ClassBarbarian)

	for _, tc := range []struct {
		name     string
		running  bool
		armor    int // sum of the speed columns worn
		frw      int
		wantPct  int
		wantYard float64 // yards per second: 9.375 subtiles per second is 6 yards
	}{
		{"walk", false, 0, 0, 100, 6},
		{"run", true, 0, 0, 150, 9},
		{"walk, plate (10)", false, 10, 0, 90, 5.4},
		{"run, plate and tower shield (20)", true, 20, 0, 130, 7.8},
		{"run, scale mail (10) and large shield (5)", true, 15, 0, 135, 8.1},
		// diminished faster run/walk: 150*30/(150+30) = 25
		{"run 30 frw", true, 0, 30, 175, 10.5},
		{"walk 30 frw, plate", false, 10, 30, 115, 6.9},
	} {
		pct := PlayerSpeedPercent(b, tc.running, tc.frw, -tc.armor)
		if pct != tc.wantPct {
			t.Errorf("%s: %d percent, want %d", tc.name, pct, tc.wantPct)
		}

		yd := PlayerSubtilesPerSecond(b, tc.running, tc.frw, -tc.armor) * 6 / 9.375
		if d := yd - tc.wantYard; d > 0.05 || d < -0.05 {
			t.Errorf("%s: %.2f yards per second, want %.2f", tc.name, yd, tc.wantYard)
		}
	}

	// the floor of 25 percent
	if got := PlayerSpeedPercent(b, false, 0, -90); got != 25 {
		t.Errorf("floor: %d", got)
	}
}
