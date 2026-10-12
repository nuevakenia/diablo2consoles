package d2vendor

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2trade"
)

// w36: stock audit of the Act 1-3 vendors on the real tables. Which vendor
// sells scrolls and tomes is data (the PermStoreItem rows and the Min/Max
// columns of misc.txt); the rules are the ones verified in verify_test.go.

func TestW36ScrollAndTomeStock(t *testing.T) {
	rec := loadRecords(t)

	scrollers := map[string]bool{"Akara": true, "Drognan": true, "Ormus": true}

	for _, name := range []string{"Akara", "Fara", "Drognan", "Elzix", "Lysander", "Hralti", "Alkor", "Ormus", "Asheara"} {
		v, ok := ByName(name)
		if !ok {
			t.Fatalf("%s is not a modelled vendor", name)
		}

		have := map[string]bool{}
		for _, b := range BasesFor(rec, v) {
			have[b.Code] = true
		}

		for _, code := range []string{"tsc", "isc", "tbk", "ibk"} {
			if have[code] != scrollers[name] {
				t.Errorf("%s: sells %s = %v, want %v", name, code, have[code], scrollers[name])
			}
		}

		if !scrollers[name] {
			continue
		}

		// Min 11 on the scroll rows: a fresh stock always offers both scrolls.
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

	// NPCs without a trade window in the engine
	for _, name := range []string{"Greiz", "Jerhyn", "Meshif", "Atma", "Warriv", "Cain"} {
		if _, ok := ByName(name); ok {
			t.Errorf("%s should not be a trading vendor", name)
		}
	}
}

func TestW36ScrollPrices(t *testing.T) {
	rec := loadRecords(t)

	// misc.txt costs: tsc 100, isc 80. The player pays cost * PlayerPays/1024.
	want := map[string]int{"tsc": 100, "isc": 80}

	for _, name := range []string{"Akara", "Drognan", "Ormus"} {
		v, _ := ByName(name)
		n := NPCPricing(rec, v, nil)

		for code, cost := range want {
			icr := rec.Item.All[code]
			if icr == nil {
				t.Fatalf("no %s row", code)
			}

			it := &d2trade.Item{BaseCost: cost, Quantity: 1, MaxStack: icr.MaxStack, Identified: true}
			if got := d2trade.ItemPrice(it, d2trade.Params{Mode: d2trade.ModeBuy, NPC: n}); got != cost {
				t.Errorf("%s sells %s for %d, want %d", name, code, got, cost)
			}

			if got := d2trade.ItemPrice(it, d2trade.Params{Mode: d2trade.ModeSell, NPC: n}); got != cost/2 {
				t.Errorf("%s pays %d for %s, want %d", name, got, code, cost/2)
			}
		}
	}
}

func TestW36PotionStock(t *testing.T) {
	rec := loadRecords(t)

	want := map[string][]string{
		"Akara": {"hp1", "mp1"}, "Drognan": {"hp1", "hp2", "mp1", "mp2"},
		"Lysander": {"hp1", "hp2", "mp1", "mp2"}, "Ormus": {"hp1", "hp2", "hp3", "mp1", "mp2", "mp3"},
	}

	for name, codes := range want {
		v, _ := ByName(name)
		have := map[string]bool{}

		for _, b := range BasesFor(rec, v) {
			if b.Permanent {
				have[b.Code] = true
			}
		}

		for _, c := range codes {
			if !have[c] {
				t.Errorf("%s does not stock %s", name, c)
			}
		}
	}
}
