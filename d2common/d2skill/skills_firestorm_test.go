package d2skill

import "testing"

func init() {
	rows = append(rows,
		row{"skill": "Firestorm9", "Id": "727", "charclass": "dru", "srvdofunc": "117", "srvmissilea": "firestormmaker",
			"calc1": "par1", "Param1": "4", "manashift": "8", "EType": "fire", "EMin": "3", "EMax": "6", "HitShift": "2"},
	)
}

func TestFirestormCastsCalc1Emitters(t *testing.T) {
	cf := newClassFixture(map[string]int{"Firestorm9": 5})
	_, r := cf.cast("Firestorm9", 12, 5)

	if len(r.Missiles) != 4 {
		t.Fatalf("calc1 = 4 emitters, got %d", len(r.Missiles))
	}

	for _, m := range r.Missiles {
		if m.Spec.Name != "firestormmaker" {
			t.Fatalf("emitter %q", m.Spec.Name)
		}
	}
}

func TestFirestormExtraEmittersWalkPathType10(t *testing.T) {
	cf := newClassFixture(map[string]int{"Firestorm9": 5})
	_, r := cf.cast("Firestorm9", 12, 5)

	if len(r.Missiles) != 4 {
		t.Fatalf("want 4 emitters, got %d", len(r.Missiles))
	}

	// SRVDO_117 casts the first emitter straight (SKILL_ServerCastMissile),
	// only the other calc1 - 1 go through the path type 10 hook 0x5c7340.
	for i, m := range r.Missiles {
		if got := m.OnWalkPath(); got != (i > 0) {
			t.Errorf("emitter %d on a walk path = %v", i, got)
		}
	}
}
