package d2skill

import "testing"

func TestAuraPassiveStatsRealRows(t *testing.T) {
	reg := loadRealRegistry(t)

	has := func(ms []StatMod, stat string) (int, bool) {
		for _, m := range ms {
			if m.Stat == stat {
				return m.Value, true
			}
		}

		return 0, false
	}

	cases := []struct {
		name string
		stat string
		want bool
	}{
		{"Fanaticism", "damagepercent", true},        // 65, no passivestate
		{"Holy Fire", "firemaxdam", true},            // 66
		{"Holy Freeze", "coldmaxdam", true},          // 81
		{"Holy Shock", "lightmindam", true},          // 66
		{"Blessed Aim", "item_tohit_percent", false}, // has a passivestate: permanent passive path
		{"Resist Fire", "maxfireresist", false},
		{"Might", "damagepercent", false}, // no passive stats at all
	}

	for _, c := range cases {
		sk := reg.ByName(c.name)
		if sk == nil {
			t.Fatalf("%s missing", c.name)
		}

		got := auraPassiveStats(realEnv(reg, c.name, 10, nil), sk)
		if _, ok := has(got, c.stat); ok != c.want {
			t.Errorf("%s: %s present=%v want %v (%v)", c.name, c.stat, ok, c.want, got)
		}
	}

	if v, _ := has(auraPassiveStats(realEnv(reg, "Fanaticism", 10, nil), reg.ByName("Fanaticism")), "damagepercent"); v <= 0 {
		t.Errorf("Fanaticism damagepercent %d, want > 0", v)
	}
}
