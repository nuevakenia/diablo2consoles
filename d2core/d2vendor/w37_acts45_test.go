package d2vendor

import "testing"

// w37: stock audit of the Act 4 and 5 vendors on the real tables (columns of the item tables via BasesFor).

func basesOf(t *testing.T, name string) map[string]bool {
	t.Helper()

	rec := loadRecords(t)

	v, ok := ByName(name)
	if !ok {
		t.Fatalf("%s is not a modelled vendor", name)
	}

	have := map[string]bool{}
	for _, b := range BasesFor(rec, v) {
		have[b.Code] = true
	}

	return have
}

func TestW37Act45Stock(t *testing.T) {
	type want struct {
		has, hasNot []string
	}

	cases := map[string]want{
		// Jamella and Malah are the potion and scroll sellers of their acts (Min/Max rows of the scrolls and tomes)
		"Jamella": {has: []string{"tsc", "isc", "tbk", "ibk", "hp3", "mp3", "key"}, hasNot: []string{"aar"}},
		"Malah":   {has: []string{"tsc", "isc", "tbk", "ibk", "hp3", "mp3", "key"}, hasNot: []string{"aar", "gix"}},
		// Halbu and Larzuk sell weapons and armor only, no potions or scrolls
		"Halbu":  {has: []string{"aar", "gsd"}, hasNot: []string{"tsc", "isc", "hp1", "mp1", "key"}},
		"Larzuk": {has: []string{"aar", "gsd"}, hasNot: []string{"tsc", "isc", "hp1", "mp1", "key"}},
		// Anya (Drehya): armor and throwing weapons, no potions or scrolls
		"Drehya": {has: []string{"aar", "jav"}, hasNot: []string{"tsc", "isc", "hp1", "mp1", "key"}},
	}

	for name, w := range cases {
		have := basesOf(t, name)

		for _, c := range w.has {
			if !have[c] {
				t.Errorf("%s should sell %s", name, c)
			}
		}

		for _, c := range w.hasNot {
			if have[c] {
				t.Errorf("%s should not sell %s", name, c)
			}
		}
	}

	// no trade window in the engine
	for _, n := range []string{"Qual-Kehk", "Tyrael", "Cain", "Nihlathak"} {
		if _, ok := ByName(n); ok {
			t.Errorf("%s should not be a trading vendor", n)
		}
	}
}

func TestW37Act45FreshScrollStock(t *testing.T) {
	rec := loadRecords(t)

	for _, name := range []string{"Jamella", "Malah"} {
		v, _ := ByName(name)

		for seed := uint32(1); seed <= 20; seed++ {
			s := GenerateSeeded(seed, BasesFor(rec, v), Options{PlayerLevel: 1, Tier: ActIndex(v)})
			found := map[string]bool{}

			for _, it := range s.Items {
				found[it.Code] = true
			}

			if !found["tsc"] || !found["isc"] {
				t.Errorf("%s seed %d: no tsc/isc in a fresh stock", name, seed)
			}
		}
	}
}
