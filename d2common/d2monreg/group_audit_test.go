package d2monreg

import (
	"math"
	"testing"
)

// Group audit (wave 26): on a fully open map the density hits of a level follow the table (rooms * 169 rolls of 3x3
// subtiles * (MonDen+1) / 100000: the roll passes when step % 100000 <= density, FUN_0054cad0), every plain group has
// at most MaxGrp members (the base classes 0x13 and 0x5b of FUN_0054ca80 are exempt) and the leaders of the plain groups
// equal the group diagnostic. 15 levels of all five acts, 3 difficulties, 6 game seeds.
func TestGroupAuditSample(t *testing.T) {
	tb := realUModTables(t)
	levels := []int{2, 3, 6, 8, 20, 27, 40, 47, 59, 74, 80, 91, 104, 107, 112}

	checked := 0

	for diff := 0; diff < 3; diff++ {
		for _, lvl := range levels {
			hits, want := 0, 0.0

			for seed := uint32(1); seed <= 6; seed++ {
				g := NewGame(tb, 77*seed+uint32(diff), diff, true)
				rg := g.Regions[lvl]

				if rg == nil || rg.Density == 0 || len(rg.Types) == 0 {
					continue
				}

				pop := populateLevel(g, lvl, 30, seed*31)
				hits += g.Diag.Hits
				want += 30 * 169 * float64(rg.Density+1) / 100000

				followers := map[*Unit]int{}

				for _, u := range pop.Units {
					if !u.Unique && !u.Champion && u.Leader != nil && u.Minion {
						followers[u.Leader]++
					}
				}

				groups := 0

				for _, u := range pop.Units {
					if u.Unique || u.Champion || u.Minion {
						continue
					}

					groups++

					m := tb.Mons[u.Class]
					size := 1 + followers[u]

					if size > m.MaxGrp && m.BaseID != 0x13 && m.BaseID != 0x5b {
						t.Errorf("level %d diff %d: %s group of %d, MaxGrp %d", lvl, diff, m.Key, size, m.MaxGrp)
					}
				}

				if groups != g.Diag.Groups {
					t.Errorf("level %d diff %d: %d leaders, diag groups %d", lvl, diff, groups, g.Diag.Groups)
				}
			}

			if want < 20 {
				continue
			}

			checked++

			if d := math.Abs(float64(hits)-want) / math.Sqrt(want); d > 5 {
				t.Errorf("level %d diff %d: %d hits, expected %.1f (%.1f sigma)", lvl, diff, hits, want, d)
			}
		}
	}

	if checked < 20 {
		t.Errorf("only %d level/difficulty pairs had enough density to check", checked)
	}
}
