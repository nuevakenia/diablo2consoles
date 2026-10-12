package d2drop

import (
	"math"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// expectedDrops is the analytic mean number of drops of one roll of a class
// with positive picks (negative picks are the deterministic walk, tested
// separately): every pick drops nothing with NoDrop/(total+NoDrop) and
// otherwise one entry, weighted by Prob; an entry that names a class
// recursively drops that class' mean.
func expectedDrops(tcs *TreasureTable, name string, players, depth int) float64 {
	tc, ok := tcs.TreasureClass(name)
	if !ok || depth > 20 {
		return 1
	}

	if tc.Picks < 0 {
		return math.NaN()
	}

	total := tc.TotalProb()
	if total == 0 {
		return 0
	}

	picks := tc.Picks
	if picks == 0 {
		picks = 1
	}

	nd := EffectiveNoDrop(tc.NoDrop, total, players)

	var inner float64

	for _, e := range tc.Entries {
		v := 1.0
		if _, isTC := tcs.TreasureClass(e.Code); isTC {
			v = expectedDrops(tcs, e.Code, players, depth+1)
		}

		inner += float64(e.Prob) * v
	}

	return float64(picks) * inner / float64(total+nd)
}

// TestW31OrdinaryMonsterPicks: for the ordinary monster classes of the three
// difficulties (Act N [(N)|(H)] Good / Junk / Equip / Cpot / Cx style groups)
// the engine's mean drop count equals the analytic pick/NoDrop model. The
// pick rule is the exe's (5585d0): Picks rolls, each in [0, total+NoDrop),
// a roll below NoDrop drops nothing; NoDrop scales with the player count.
func TestW31OrdinaryMonsterPicks(t *testing.T) {
	rt := loadReal(t)
	d := &Dropper{TCs: rt.tcs}

	checked, compared := 0, 0

	for _, tc := range rt.tcs.list {
		if tc.Picks <= 0 || tc.NoDrop == 0 || !strings.HasPrefix(tc.Name, "Act ") || checked >= 60 {
			continue
		}

		if tc.Level > 0 && checked%3 != 0 { // sample, the table has hundreds
			checked++

			continue
		}

		checked++

		for _, players := range []int{1, 4} {
			want := expectedDrops(rt.tcs, tc.Name, players, 0)
			if math.IsNaN(want) || want > 3 {
				continue
			}

			compared++

			const n = 6000

			rng := d2rand.New(uint32(31 + checked))
			sum := 0

			for i := 0; i < n; i++ {
				out, err := d.Roll(&Context{RNG: rng, Players: players, ILvl: 50, MaxDrops: 100}, tc.Name)
				if err != nil {
					t.Fatalf("%s: %v", tc.Name, err)
				}

				sum += len(out)
			}

			got := float64(sum) / n
			if math.Abs(got-want) > 0.04+0.05*want {
				t.Errorf("%s players %d: mean %.3f, model %.3f", tc.Name, players, got, want)
			}
		}
	}

	if compared < 10 {
		t.Fatalf("only %d class/player pairs compared", compared)
	}

	t.Logf("%d class/player pairs compared", compared)
}

// TestW31LadderOnlyUniqueGate pins the ladder-only rule of
// ITEMGEN_PickUniqueItem (5547d0): a ladder-only unique row is a candidate
// when the game type byte or the +0x74 flag is set, or the row is not
// ladder-only; the SetItems picker has no ladder column (only set 29, the
// Cow King's set, is gated, by game flag bit 0).
func TestW31LadderOnlyUniqueGate(t *testing.T) {
	cases := []struct {
		g    *GameState
		want bool
	}{
		{nil, false}, {&GameState{}, false}, {&GameState{Ladder: true}, true},
		{&GameState{Hardcore: true}, true}, {&GameState{Ladder: true, Hardcore: true}, true},
	}

	for i, c := range cases {
		if got := c.g.allowsLadderOnly(); got != c.want {
			t.Errorf("case %d: %v, want %v", i, got, c.want)
		}
	}
}
