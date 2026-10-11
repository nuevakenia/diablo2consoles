package d2skill

import "testing"

func TestCastAtPointCreatesAStationaryControlMissile(t *testing.T) {
	cf := newClassFixture(map[string]int{"Armageddon9": 5})
	id := cf.id("Armageddon9")

	m := cf.p.CastAtPoint(cf.u, id, "armageddoncontrol", 14, 9)
	if m == nil {
		t.Fatal("no missile")
	}

	if int(m.X) != 14 || int(m.Y) != 9 || m.Spec.Name != "armageddoncontrol" {
		t.Fatalf("missile %v", m)
	}

	if cf.p.CastAtPoint(cf.u, 9999, "armageddoncontrol", 1, 1) != nil {
		t.Fatal("an unknown skill must not create a missile")
	}

	if cf.p.CastAtPoint(cf.u, id, "nosuchmissile", 1, 1) != nil {
		t.Fatal("an unknown missile must not be created")
	}
}
