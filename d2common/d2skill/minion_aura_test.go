package d2skill

import "testing"

func init() {
	rows = append(rows,
		row{"skill": "Test Fire Golem", "Id": "520", "charclass": "nec", "srvdofunc": "56", "summon": "firegolem",
			"pettype": "firegolem", "petmax": "1", "summode": "NU", "calc1": "0", "manashift": "8",
			"sumskill1": "Test Holy Fire", "sumsk1calc": "min(lvl,30)"},
		row{"skill": "Test Holy Fire", "Id": "521", "srvdofunc": "66", "aurastate": "holyfire", "aurafilter": "3",
			"aurarangecalc": "par1", "Param1": "6", "etype": "fire", "EMin": "5", "EMax": "9", "perdelay": "25", "manashift": "8"},
	)
}

// A golem's aura monster skill is found in the sumskill list with its level, and evaluates to the aura effect.
func TestFireGolemOrderCarriesAura(t *testing.T) {
	cf := newClassFixture(map[string]int{"Test Fire Golem": 4})

	_, r := cf.cast("Test Fire Golem", 5, 5)
	if !r.OK {
		t.Fatal(r.Reason)
	}

	o := effectOf(t, r, "summon").Summon
	if o.AuraSkill != "Test Holy Fire" || o.AuraLevel != 4 {
		t.Fatalf("aura %q level %d", o.AuraSkill, o.AuraLevel)
	}

	ef := cf.p.MinionAura(cf.u, o.AuraSkill, o.AuraLevel)
	if ef == nil || ef.Radius != 6 || ef.Level != 4 {
		t.Fatalf("effect %+v", ef)
	}

	if cf.p.MinionAura(cf.u, "Test Fire Golem", 4) != nil {
		t.Error("a summon skill is not an aura")
	}
}

func TestCreateAtTargetInRange(t *testing.T) {
	// UNIT_GetDistanceToPoint metric: max + min/2, the missile is made below 0x65
	tests := []struct {
		dx, dy int
		want   bool
	}{{0, 0, true}, {100, 0, true}, {101, 0, false}, {-100, 0, true}, {90, 30, false}, {80, 30, true}}

	for _, tc := range tests {
		if got := CreateAtTargetInRange(10, 10, 10+tc.dx, 10+tc.dy); got != tc.want {
			t.Errorf("(%d,%d) = %v", tc.dx, tc.dy, got)
		}
	}
}
