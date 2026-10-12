package d2skill

import "testing"

// Strafe against a single enemy (SRVDO_012 0x5da6c0, wave 24): the scan for the next target hands the current
// unit back when it is alone, so the whole burst goes at it. The old port stopped after one arrow.
func TestStrafeLoneEnemyTakesTheBurst(t *testing.T) {
	cf := w23Fixture(map[string]int{"Strafe W23": 8})
	cf.u.ranged = "arrow"
	foe := cf.addFoe("only", 8, 0)

	_, r := cf.castOn("Strafe W23", foe)
	if !r.OK {
		t.Fatalf("cast: %s", r.Reason)
	}

	want := StrafeBurst(cf.reg.ByName("Strafe W23").Params[3]+8-1, 2+8/4, 1)
	if want < 2 || len(r.Missiles) != want {
		t.Errorf("%d arrows at a lone enemy, want %d", len(r.Missiles), want)
	}

	for _, m := range r.Missiles {
		if m.Spec.Name != "strafearrow" {
			t.Errorf("arrow %q", m.Spec.Name)
		}
	}
}
