package d2skill

import "testing"

// Royal Strike's finisher release (w37): count 1 runs SRVDO_040 (one progressive missile at the target), count 3
// runs SRVDO_041 (par5 = 16 chaos ice missiles with a heading each).
func TestRoyalStrikeRelease(t *testing.T) {
	for _, tc := range []struct{ count, want int }{{1, 1}, {3, 16}} {
		cf := newClassFixture(map[string]int{"Royal Strike": 4, "Dragon Talon": 12})
		z := cf.addFoe("z", 2, 0)
		cf.u.roller = &seq{vals: fours(100)}
		cf.u.ar = 100000
		cf.u.stats["progressive_other"] = tc.count

		_, r := cf.castOn("Dragon Talon", z)
		if len(r.Missiles) != tc.want {
			t.Fatalf("count %d: %d missiles, want %d", tc.count, len(r.Missiles), tc.want)
		}

		if tc.count == 3 {
			for i, m := range r.Missiles {
				x, y := int(int16(m.Data2C)), int(int16(m.Data2C>>16))
				if x < -20 || x > 19 || y < -20 || y > 19 || (x == 0 && y == 0) {
					t.Errorf("missile %d heading (%d,%d) out of range", i, x, y)
				}
			}
		}
	}
}

func TestChaosHeading(t *testing.T) {
	s := newShockSeed(0)

	for i := 0; i < 200; i++ {
		x, y := chaosHeading(&s)
		if x < -20 || x > 19 || y < -20 || y > 19 || (x == 0 && y == 0) {
			t.Fatalf("heading (%d,%d)", x, y)
		}
	}

	if packHeading(-3, 5) != 0x0005fffd {
		t.Errorf("pack %x", packHeading(-3, 5))
	}
}
