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

func TestFirestormJitterIsDeterministicAndBounded(t *testing.T) {
	for i := 0; i < 20; i++ {
		a, b := FirestormJitter(40, i), FirestormJitter(40, i)
		if a != b {
			t.Fatalf("index %d not deterministic", i)
		}

		if a < -1.5708 || a > 1.5708 {
			t.Fatalf("index %d out of +-90 degrees: %f", i, a)
		}
	}

	if FirestormJitter(40, 0) == FirestormJitter(40, 1) && FirestormJitter(40, 1) == FirestormJitter(40, 2) {
		t.Fatal("the index must change the seed")
	}
}
