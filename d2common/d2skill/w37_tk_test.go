package d2skill

import "testing"

// Telekinesis on a ground item (SRVDO_Telekinesis 0x5c79f0, w37): the six item types are picked up from afar, any
// other item does nothing, and the start gate measures the range to the item.
func TestTelekinesisItems(t *testing.T) {
	cf := newClassFixture(map[string]int{"Telekinesis": 5})
	cf.reg.Add(skillFromRow(row{"skill": "Telekinesis", "Id": "43", "srvdofunc": "21", "aurarangecalc": "25",
		"Param2": "35", "manashift": "8"}))
	cf.u.levels["Telekinesis"] = 5
	cf.u.roller = &seq{vals: make([]uint32, 20)}

	id := cf.id("Telekinesis")

	cases := []struct {
		name   string
		item   *TKItem
		start  bool
		pickup bool
	}{
		{"potion", &TKItem{ID: "i1", X: 10, Y: 0, Pick: true}, true, true},
		{"sword", &TKItem{ID: "i2", X: 10, Y: 0}, true, false},
		{"out of range", &TKItem{ID: "i3", X: 40, Y: 0, Pick: true}, false, false},
	}

	for _, tc := range cases {
		tg := Target{X: tc.item.X, Y: tc.item.Y, TKItem: tc.item}

		st := cf.p.Start(cf.u, id, tg)
		if st.OK != tc.start {
			t.Errorf("%s: start ok=%v reason=%q", tc.name, st.OK, st.Reason)
			continue
		}

		if !st.OK {
			continue
		}

		r := cf.p.Do(cf.u, id, tg)

		got := ""

		for _, e := range r.Effects {
			if e.Kind == "pickup_item" {
				got = e.ItemID
			}
		}

		if (got != "") != tc.pickup || (tc.pickup && got != tc.item.ID) {
			t.Errorf("%s: pickup %q", tc.name, got)
		}
	}

	// no unit and no item: refused as before
	if st := cf.p.Start(cf.u, id, Target{X: 3, Y: 3}); st.OK {
		t.Errorf("an empty target must be refused")
	}
}
