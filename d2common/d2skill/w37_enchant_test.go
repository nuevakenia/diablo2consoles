package d2skill

import "testing"

// Enchant (SRVDO_025 0x5c8180, w37): the timed state goes to an allied target unit, else to the caster.
func TestEnchantAllyTarget(t *testing.T) {
	cf := newClassFixture(map[string]int{"Enchant": 5})
	cf.reg.Add(skillFromRow(row{"skill": "Enchant", "Id": "52", "srvdofunc": "25", "aurastate": "enchant",
		"auralencalc": "par1", "Param1": "300", "aurastat1": "firemindam", "aurastatcalc1": "lvl", "manashift": "8"}))
	cf.u.levels["Enchant"] = 5

	id := cf.id("Enchant")
	ally := cf.addFoe("merc", 3, 0)

	// no ally: the caster
	tg := Target{X: 3, Y: 0}
	cf.p.Start(cf.u, id, tg)

	r := cf.p.Do(cf.u, id, tg)
	if e := effectOf(t, r, "self_state"); e.State != "enchant" {
		t.Fatalf("self cast %+v", e)
	}

	// an ally under the cursor receives it instead
	tg.Ally = ally
	cf.p.Start(cf.u, id, tg)

	r = cf.p.Do(cf.u, id, tg)

	e := effectOf(t, r, "unit_state")
	if e.State != "enchant" || e.Target != ally || e.Frames != 300 {
		t.Fatalf("ally cast %+v", e)
	}

	for _, ef := range r.Effects {
		if ef.Kind == "self_state" {
			t.Errorf("the caster must not get the state as well: %+v", ef)
		}
	}
}
