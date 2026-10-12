package d2vendor

import (
	"math"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/d2drop"
)

// The gamble quality odds of the shipped difficulty rows (GambleRare 10000,
// GambleSet 100, GambleUnique 50 of 100000, the roll of 0x5765d0): the roll
// is compared against the SUM of the three, so the windows are stacked
// (unique first, then set, then rare): unique 0.05%, set 0.10%, rare 10.00%
// (the three rows add up to 10150 of 100000), magic otherwise.
func TestW31GambleQualityOdds(t *testing.T) {
	p := ShippedGambleParams
	rng := d2rand.New(12345)

	const n = 1000000

	got := map[d2drop.Quality]int{}
	for i := 0; i < n; i++ {
		got[GambleQuality(rng, p)]++
	}

	sum := float64(p.Rare + p.Set + p.Unique)
	want := map[d2drop.Quality]float64{
		d2drop.QualityUnique: float64(p.Unique) / 100000,
		d2drop.QualitySet:    float64(p.Set) / 100000,
		d2drop.QualityRare:   (sum - float64(p.Unique+p.Set)) / 100000,
		d2drop.QualityMagic:  1 - sum/100000,
	}

	for q, w := range want {
		g := float64(got[q]) / n
		if math.Abs(g-w) > 4*math.Sqrt(w*(1-w)/n)+1e-5 {
			t.Errorf("quality %d: %.5f, expected %.5f", q, g, w)
		}
	}
}

// The exceptional swap chance of a gamble base item is
// ((ilvl - baseLevel)*Uber + 1)/10000 (0x5764c0).
func TestW31GambleUpgradeChance(t *testing.T) {
	p := ShippedGambleParams
	b := GambleBase{Code: "lsd", Exc: "cut", ExcLvl: 30, Elite: "5ls", EliteL: 60}
	rng := d2rand.New(777)

	const n = 400000

	exc := 0

	for i := 0; i < n; i++ {
		if UpgradeGambleBase(rng, b, 40, p) == "cut" {
			exc++
		}
	}

	want := float64((40-30)*p.Uber+1) / 10000
	if g := float64(exc) / n; math.Abs(g-want) > 0.004 {
		t.Errorf("exceptional chance %.4f, expected %.4f", g, want)
	}
}
