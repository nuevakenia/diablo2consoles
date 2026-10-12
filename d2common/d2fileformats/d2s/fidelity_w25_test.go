package d2s

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
)

// Second fidelity pass (wave 25): every saved stat of the real ItemStatCost at its extreme values, plus
// item kinds in one table. Synthetic data only; real tables are optional ($D2_TABLES).

// TestStatSweepRealTables writes one item per saved stat (with min and max value and param) and requires the
// bits to survive a parse and a re-encode unchanged.
func TestStatSweepRealTables(t *testing.T) {
	tb := robustTables()
	if tb == nil {
		t.Skip("no real tables")
	}

	followers := map[int]bool{}

	for _, fs := range groupFollowers {
		for _, f := range fs {
			followers[f] = true
		}
	}

	ids := make([]int, 0, len(tb.stats))
	for id := range tb.stats {
		ids = append(ids, id)
	}

	sort.Ints(ids)

	for _, id := range ids {
		st := tb.stats[id]
		if st.SaveBits == 0 || st.SaveBits > 32 || followers[id] {
			continue
		}

		maxV := int64(1)<<uint(st.SaveBits) - 1 - int64(st.SaveAdd)
		minV := -int64(st.SaveAdd)

		for _, v := range []int64{minV, maxV} {
			for _, pm := range []uint32{0, uint32(1)<<uint(st.SaveParamBits) - 1} {
				name := fmt.Sprintf("stat%d/%s/v=%d/p=%d", id, st.Name, v, pm)
				props := []Property{{ID: id, Value: v, Param: pm}}

				for _, f := range groupFollowers[id] {
					fs := tb.stats[f]
					props = append(props, Property{ID: f, Value: int64(1)<<uint(fs.SaveBits) - 1 - int64(fs.SaveAdd)})
				}

				it, err := NewItem(Item{Identified: true, Location: LocationStored, Page: 1, Code: "cap", ID: 5, Level: 9,
					Quality: QualityMagic, MagicPrefix: 1, MagicSuffix: 1, Defense: 3, MaxDurability: 12, Durability: 12,
					Properties: props}, tb)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}

				b, err := EncodeItem(&it, tb)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}

				list := append([]byte{'J', 'M', 1, 0}, b...)

				back, n, err := ParseItemList(list, tb)
				if err != nil || n != len(list) {
					t.Fatalf("%s: parse: %v (%d of %d)", name, err, n, len(list))
				}

				if !itemsEqual(it, back[0]) {
					t.Fatalf("%s: changed\n%+v\n%+v", name, it.Properties, back[0].Properties)
				}

				again, err := EncodeItem(&back[0], tb)
				if err != nil || !bytes.Equal(again, b) {
					t.Fatalf("%s: re-encode differs (%v)", name, err)
				}
			}
		}
	}
}
