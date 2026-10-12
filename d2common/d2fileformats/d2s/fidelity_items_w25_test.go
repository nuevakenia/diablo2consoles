package d2s

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
)

// TestItemCodeSweepRealTables builds an item for every code of the real armor/weapons/misc tables in every
// quality (plus ethereal, personalised, socketed with a child, runeword and charged variants) and requires a
// byte-exact encode/parse/encode round trip. Synthetic items only.
func TestItemCodeSweepRealTables(t *testing.T) {
	tb := robustTables()
	if tb == nil {
		t.Skip("no real tables")
	}

	codes := make([]string, 0, len(tb.items))
	for c := range tb.items {
		codes = append(codes, c)
	}

	sort.Strings(codes)

	child := Item{Code: "r01", Location: LocationSocketed, ID: 77, Identified: true}
	if tb.ItemKindOf("r01") == 0 {
		child.Code = "key"
	}

	// 204 = item_charged_skill: param is skill (6) + level (10), value is charges (8) + max charges (8)
	charged := Property{ID: 204, Param: 0x1<<6 | 5, Value: 0x0505}
	if _, _, _, _, ok := tb.StatSaveInfo(204); !ok {
		t.Fatal("stat 204 not saved")
	}

	variants := []struct {
		name string
		edit func(*Item)
	}{
		{"plain", func(*Item) {}},
		{"ethereal", func(it *Item) { it.Ethereal = true }},
		{"personal", func(it *Item) { it.Personalized, it.PersonalName = true, "Sorc-Name" }},
		{"socketed", func(it *Item) { it.TotalSockets, it.SocketCount, it.Children = 1, 1, []Item{child} }},
		{"runeword", func(it *Item) {
			it.RunewordID, it.TotalSockets = 27, 1
			it.SocketCount, it.Children = 1, []Item{child}
			it.RunewordProperties = []Property{{ID: 0, Value: 4}}
		}},
		{"charged", func(it *Item) { it.Properties = []Property{charged} }},
	}

	for _, code := range codes {
		info := tb.items[code]

		for q := QualityLow; q <= QualityCrafted; q++ {
			for _, v := range variants {
				it := Item{Identified: true, Location: LocationStored, Page: 5, X: 3, Y: 2, Code: code, ID: 0xdeadbeef,
					Level: 99, Quality: q, MaxDurability: 255, Durability: 254}

				switch q {
				case QualityLow:
					it.LowQualityID = 7
				case QualityHigh:
					it.HighQuality = 7
				case QualityMagic:
					it.MagicPrefix, it.MagicSuffix = 2047, 2047
				case QualitySet:
					it.SetID, it.SetListMask = 4095, 0x1f
					it.SetProperties = [][]Property{{{ID: 0, Value: 1}}, {{ID: 0, Value: 2}}, {{ID: 0, Value: 3}},
						{{ID: 0, Value: 4}}, {{ID: 0, Value: 5}}}
				case QualityUnique:
					it.UniqueID = 4095
				case QualityRare, QualityCrafted:
					it.RareName1, it.RareName2, it.RareMask = 255, 255, 0x3f
					it.RareAffixes = [6]uint16{2047, 1, 2047, 1, 2047, 1}
				}

				if info.kind == KindArmor {
					it.Defense = 2000
				}

				if info.noDurability || info.kind == KindMisc {
					it.MaxDurability, it.Durability = 0, 0
				}

				if info.stackable {
					it.Quantity = 511
				}

				if tb.IsTome(code) {
					it.TomeBits = 31
				}

				if info.compact {
					// simple items hold only position and code
					if v.name != "plain" || q != QualityNormal {
						continue
					}

					it = Item{Identified: true, Location: LocationStored, Page: 5, X: 3, Y: 2, Code: code}
				}

				v.edit(&it)

				name := fmt.Sprintf("%s/q%d/%s", code, q, v.name)

				built, err := NewItem(it, tb)
				if err != nil {
					// a variant that is meaningless for the kind (simple items hold nothing) may be refused
					if info.compact {
						continue
					}

					t.Fatalf("%s: %v", name, err)
				}

				b, err := EncodeItem(&built, tb)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}

				list := append([]byte{'J', 'M', 1, 0}, b...)

				back, n, err := ParseItemList(list, tb)
				if err != nil || n != len(list) {
					t.Fatalf("%s: parse %v (%d of %d)", name, err, n, len(list))
				}

				if !itemsEqual(built, back[0]) {
					t.Fatalf("%s: changed\n%+v\n%+v", name, built, back[0])
				}

				if again, err := EncodeItem(&back[0], tb); err != nil || !bytes.Equal(again, b) {
					t.Fatalf("%s: re-encode differs (%v)", name, err)
				}
			}
		}
	}
}
