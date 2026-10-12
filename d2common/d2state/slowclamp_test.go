package d2state

import "testing"

func slowOf(mods []StatMod, name string) (int, bool) {
	for _, m := range mods {
		if m.Stat == name {
			return m.Value, true
		}
	}

	return 0, false
}

// Holy Freeze: velocitypercent, attackrate and other_animrate all get -dm34 (25..60); the first two are raised to the
// target's cold effect, the third is written last and stays as it is.
func TestClampAuraSlow(t *testing.T) {
	holy := func(v int) []StatMod {
		return []StatMod{{Stat: "velocitypercent", Value: v}, {Stat: "attackrate", Value: v}, {Stat: "other_animrate", Value: v}}
	}

	for _, c := range []struct {
		name               string
		slow, cold         int
		wantVel, wantAtk   int
		wantAnim, wantMods int
	}{
		{"weak slow passes", -30, -50, -30, -30, -30, 3},
		{"strong slow capped at -50", -60, -50, -50, -50, -60, 3},
		{"cold effect -25 caps at -25", -45, -25, -25, -25, -45, 3},
		{"cold effect 0 gives no slow", -40, 0, 0, 0, -40, 3},
	} {
		got := ClampAuraSlow(holy(c.slow), c.cold)

		v, _ := slowOf(got, "velocitypercent")
		a, _ := slowOf(got, "attackrate")
		n, _ := slowOf(got, "other_animrate")

		if v != c.wantVel || a != c.wantAtk || n != c.wantAnim || len(got) != c.wantMods {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// Stat 0x44 alone also writes stat 0x45; unrelated stats and positive values are untouched.
func TestClampAuraSlowOtherStats(t *testing.T) {
	got := ClampAuraSlow([]StatMod{{"attackrate", -80}, {"fireresist", -30}, {"velocitypercent", 20}}, -50)

	if a, _ := slowOf(got, "attackrate"); a != -50 {
		t.Errorf("attackrate %d", a)
	}

	if n, ok := slowOf(got, "other_animrate"); !ok || n != -50 {
		t.Errorf("other_animrate %d %v", n, ok)
	}

	if f, _ := slowOf(got, "fireresist"); f != -30 {
		t.Errorf("fireresist %d", f)
	}

	if v, _ := slowOf(got, "velocitypercent"); v != 20 {
		t.Errorf("a speed-up must pass, got %d", v)
	}
}
