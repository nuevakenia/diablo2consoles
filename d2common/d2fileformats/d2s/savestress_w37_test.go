package d2s

import (
	"fmt"
	"math/rand"
	"testing"
)

// Round-trip stress (wave 37): random heroes with random items, built from nothing, are written, parsed and
// written again; the two files must be byte identical and every item must come back as it went in. The
// generator is seeded, so a failure names the seed that reproduces it. Item kinds covered: armor, weapons,
// misc, every quality, socketed items with children, runewords, ethereal, personalised, charms (magic
// misc items with properties), quest items, stackables, belt potions, stash and cube pages, ears,
// corpse, mercenary items and the golem.

type stressCount map[string]int

func stressCodes(tb *ItemTables) (armor, weapons, misc, quest, charms, gems []string) {
	pick := func(into *[]string, kind ItemKind, codes ...string) {
		for _, c := range codes {
			if tb.ItemKindOf(c) == kind {
				*into = append(*into, c)
			}
		}
	}

	pick(&armor, KindArmor, "cap", "skp", "plt", "buc", "lgl", "vbt", "vbl")
	pick(&weapons, KindWeapon, "bow", "sbw", "hax", "wnd", "clb", "kit")
	pick(&misc, KindMisc, "key", "tbk", "hp1", "mp1", "rin", "amu", "isc", "tsc")
	pick(&quest, KindMisc, "box", "hst", "vip", "bks", "bkd", "j34", "g33", "ass", "tbk")
	pick(&charms, KindMisc, "cm1", "cm2", "cm3", "key")
	pick(&gems, KindMisc, "gcv", "gcw", "r01", "r05", "r10", "r20", "key")

	return
}

// randProps returns up to four properties from ids every table set knows.
func randProps(rng *rand.Rand) []Property {
	var out []Property

	for i := rng.Intn(5); i > 0; i-- {
		if rng.Intn(3) == 0 {
			out = append(out, Property{ID: 97, Param: uint32(rng.Intn(500)), Value: int64(1 + rng.Intn(5))})
		} else {
			out = append(out, Property{ID: 0, Value: int64(rng.Intn(60) - 10)})
		}
	}

	return out
}

func stressItem(t *testing.T, rng *rand.Rand, tb *ItemTables, ids *uint32, count stressCount, pool []string, loc, equip, page uint8) (Item, bool) {
	t.Helper()

	*ids += 0x01010101
	code := pool[rng.Intn(len(pool))]
	it := Item{Identified: rng.Intn(5) != 0, Location: loc, Equipped: equip, Page: page, X: uint8(rng.Intn(10)), Y: uint8(rng.Intn(8)),
		Code: code, ID: *ids, Level: uint8(1 + rng.Intn(99)), Quality: QualityNormal}

	kind := tb.ItemKindOf(code)
	if kind == KindArmor || kind == KindWeapon {
		if kind == KindArmor {
			it.Defense = rng.Intn(200)
		}

		it.MaxDurability = uint16(10 + rng.Intn(240))
		it.Durability = uint16(rng.Intn(int(it.MaxDurability) + 1))
	}

	if tb.IsStackable(code) {
		it.Quantity = uint16(1 + rng.Intn(100))
	}

	if kind != KindMisc {
		it.Quality = uint8(QualityLow + uint8(rng.Intn(int(QualityCrafted))))
	} else if rng.Intn(3) == 0 && map[string]bool{"rin": true, "amu": true, "cm1": true, "cm2": true, "cm3": true}[code] {
		it.Quality = QualityMagic // charms and jewellery; other misc items (scrolls, potions, keys) are simple
	}

	switch it.Quality {
	case QualityLow:
		it.LowQualityID = uint8(rng.Intn(8))
	case QualityHigh:
		it.HighQuality = uint8(rng.Intn(8))
	case QualityMagic:
		it.MagicPrefix, it.MagicSuffix = uint16(rng.Intn(400)), uint16(rng.Intn(400))
		it.Properties = randProps(rng)
	case QualitySet:
		it.SetID, it.SetListMask = uint16(1+rng.Intn(100)), uint8(rng.Intn(32))
		it.Properties = randProps(rng)

		for i := bitsSet(it.SetListMask); i > 0; i-- {
			it.SetProperties = append(it.SetProperties, randProps(rng))
		}
	case QualityUnique:
		it.UniqueID = uint16(1 + rng.Intn(400))
		it.Properties = randProps(rng)
	case QualityRare, QualityCrafted:
		it.RareName1, it.RareName2 = uint8(1+rng.Intn(100)), uint8(1+rng.Intn(100))
		it.RareMask = uint8(rng.Intn(64))

		for i := range it.RareAffixes {
			if it.RareMask>>uint(i)&1 == 1 {
				it.RareAffixes[i] = uint16(1 + rng.Intn(300))
			}
		}

		it.Properties = randProps(rng)
	}

	if kind != KindMisc {
		it.Ethereal = rng.Intn(6) == 0
		if rng.Intn(8) == 0 {
			it.Personalized, it.PersonalName = true, fmt.Sprintf("Hero%d", rng.Intn(100))
		}

		// sockets, runewords
		switch rng.Intn(5) {
		case 0:
			n := uint8(1 + rng.Intn(4))
			it.TotalSockets = n

			for s := uint8(0); s < n && rng.Intn(4) != 0; s++ {
				*ids += 0x01010101
				child, err := NewItem(socketable(tb, rng, *ids), tb)
				if err == nil {
					it.Children = append(it.Children, child)
				}
			}

			it.SocketCount = uint8(len(it.Children))

			count["socketed"]++
		case 1:
			if it.Quality == QualityNormal {
				it.RunewordID, it.TotalSockets = uint16(1+rng.Intn(100)), uint8(2+rng.Intn(3))
				it.RunewordProperties = randProps(rng)
				it.Properties = nil

				for s := uint8(0); s < it.TotalSockets; s++ {
					*ids += 0x01010101
					child, err := NewItem(socketable(tb, rng, *ids), tb)
					if err == nil {
						it.Children = append(it.Children, child)
					}
				}

				it.SocketCount = uint8(len(it.Children))

				count["runeword"]++
			}
		}
	}

	out, err := NewItem(it, tb)
	if err != nil {
		count["rejected by NewItem"]++

		return Item{}, false
	}

	count[fmt.Sprintf("kind%d/q%d", kind, out.Quality)]++

	return out, true
}

// socketable is a rune or gem to put into a socket.
func socketable(tb *ItemTables, rng *rand.Rand, id uint32) Item {
	it := Item{Code: gemsFor(tb, rng), Location: LocationSocketed, Quality: QualityNormal, Identified: true, ID: id}
	if tb.IsStackable(it.Code) {
		it.Quantity = 1
	}

	return it
}

func gemsFor(tb *ItemTables, rng *rand.Rand) string {
	_, _, _, _, _, gems := stressCodes(tb)

	return gems[rng.Intn(len(gems))]
}

func bitsSet(b uint8) int {
	n := 0
	for ; b != 0; b &= b - 1 {
		n++
	}

	return n
}

// stressHero builds one random hero with item lists of every place.
func stressHero(t *testing.T, seed int64, tb *ItemTables, count stressCount) *Character {
	t.Helper()

	rng := rand.New(rand.NewSource(seed))
	c := matrixBase(t, tb)
	c.Header.Class = Class(rng.Intn(int(Assassin) + 1))
	c.Header.Status = 0
	expansion := rng.Intn(4) != 0

	if expansion {
		c.Header.Status = StatusExpansion
	}

	if rng.Intn(5) == 0 {
		c.Header.Status |= StatusHardcore
	}

	c.Header.Level = uint8(1 + rng.Intn(99))
	c.Header.MapSeed = rng.Uint32()

	armor, weapons, misc, quest, charms, _ := stressCodes(tb)
	all := append(append(append([]string{}, armor...), weapons...), misc...)
	var ids uint32 = uint32(seed) << 8

	add := func(into *[]Item, n int, pool []string, loc, equip, page uint8) {
		for i := 0; i < n; i++ {
			e := equip
			if loc == LocationEquipped {
				e = uint8(1 + rng.Intn(12))
			}

			if it, ok := stressItem(t, rng, tb, &ids, count, pool, loc, e, page); ok {
				*into = append(*into, it)
			}
		}
	}

	add(&c.Items, rng.Intn(10), all, LocationEquipped, 0, 0)
	add(&c.Items, rng.Intn(40), all, LocationStored, 0, 1)
	add(&c.Items, rng.Intn(15), quest, LocationStored, 0, 1)
	add(&c.Items, rng.Intn(10), charms, LocationStored, 0, 1)
	add(&c.Items, rng.Intn(60), all, LocationStored, 0, 5)
	add(&c.Items, rng.Intn(12), all, LocationStored, 0, 4)

	var belt []string

	for _, code := range []string{"hp1", "mp1", "key"} {
		if tb.ItemKindOf(code) == KindMisc {
			belt = append(belt, code)
		}
	}

	add(&c.Items, rng.Intn(9), belt, LocationBelt, 0, 0)

	if rng.Intn(4) == 0 {
		c.Items = append(c.Items, Item{Identified: true, Ear: true, Code: "ear", Version: DefaultItemVersion,
			EarInfo:  &EarInfo{Class: uint8(rng.Intn(7)), Level: uint8(1 + rng.Intn(99)), Name: fmt.Sprintf("Ear%d", rng.Intn(1000))},
			Location: LocationStored, Page: 1, X: uint8(rng.Intn(10)), Y: uint8(rng.Intn(4))})
	}

	if rng.Intn(3) == 0 {
		add(&c.Corpse, 1+rng.Intn(5), all, LocationEquipped, 0, 0)
		c.HasCorpse = len(c.Corpse) > 0
	}

	if expansion && rng.Intn(2) == 0 {
		c.Header.Mercenary = Mercenary{ID: rng.Uint32() | 1, NameID: uint16(rng.Intn(50)), Type: uint16(rng.Intn(100)),
			Experience: rng.Uint32() % 1000000, Dead: rng.Intn(2) == 0}
		add(&c.MercItems, rng.Intn(5), all, LocationEquipped, 0, 0)
	}

	if expansion && c.Header.Class == Necromancer && rng.Intn(2) == 0 {
		var golem []Item

		add(&golem, 1, armor, LocationStored, 0, 0)

		if len(golem) > 0 {
			c.Golem = &golem[0]
		}
	}

	count["heroes"]++

	return c
}

func TestStressRandomHeroesRoundTrip(t *testing.T) {
	const heroes = 120

	for _, ts := range matrixTableSets(t) {
		count := stressCount{}

		for seed := int64(1); seed <= heroes; seed++ {
			c := stressHero(t, seed, ts.tb, count)
			label := fmt.Sprintf("%s/seed=%d", ts.name, seed)

			// merc items need the expansion bit; stressHero only adds them for expansion heroes
			out := roundTripExact(t, label, c, ts.tb)

			back, err := Parse(out, ts.tb)
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}

			for i := range c.Items {
				if !itemsEqual(c.Items[i], back.Items[i]) {
					t.Fatalf("%s: item %d (%s) changed:\n%+v\n%+v", label, i, c.Items[i].Code, c.Items[i], back.Items[i])
				}
			}

			for i := range c.MercItems {
				if !itemsEqual(c.MercItems[i], back.MercItems[i]) {
					t.Fatalf("%s: merc item %d changed", label, i)
				}
			}

			for i := range c.Corpse {
				if !itemsEqual(c.Corpse[i], back.Corpse[i]) {
					t.Fatalf("%s: corpse item %d changed", label, i)
				}
			}

			if (c.Golem == nil) != (back.Golem == nil) {
				t.Fatalf("%s: golem lost or invented", label)
			}
		}

		// the generator must really have produced the interesting cases
		for _, want := range []string{"socketed", "runeword"} {
			if count[want] == 0 {
				t.Errorf("%s: the generator made no %s item", ts.name, want)
			}
		}

		if count["heroes"] != heroes {
			t.Errorf("%s: %d heroes", ts.name, count["heroes"])
		}

		t.Logf("%s: %v", ts.name, count)
	}
}
