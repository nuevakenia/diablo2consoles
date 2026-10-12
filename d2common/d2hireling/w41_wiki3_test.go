package d2hireling

import "testing"

// Wiki round 3 (w41): MERC_CalcReviveCost 0x666160 is min(50000, (lvl*lvl/2)*15)
// (IMUL, signed halve, x15, CMP 0xc350; confirmed in the disassembly).
func TestW41ReviveCost(t *testing.T) {
	for _, c := range []struct{ lvl, want int }{
		{1, 0}, {2, 30}, {10, 750}, {30, 6750}, {58, 25230}, {60, 27000}, {81, 49200}, {82, 50000}, {99, 50000},
	} {
		if got := ReviveCost(c.lvl); got != c.want {
			t.Errorf("ReviveCost(%d) = %d, want %d", c.lvl, got, c.want)
		}
	}
}
