package d2skill

import "testing"

// Fend (SRVST_009, 0x5d99b0): the hit count is min(calc1, enemies in reach), Zeal (SRVST_037, 0x5d9ac0) keeps calc1.
func TestFendCountsEnemies(t *testing.T) {
	cf := newClassFixture(map[string]int{"Fend": 3, "Zeal": 7})
	z := cf.addFoe("z", 3, 0)
	cf.u.roller = &seq{vals: make([]uint32, 200)}
	cf.u.ar = 100000

	_, r := cf.castOn("Fend", z)
	if len(r.Melees) != 1 {
		t.Errorf("one enemy: fend strikes %d, want 1", len(r.Melees))
	}

	for i := 0; i < 14; i++ {
		cf.addFoe("x"+string(rune('a'+i)), 2+i%3, i%3)
	}

	_, r = cf.castOn("Fend", z)
	if len(r.Melees) != 12 {
		t.Errorf("many enemies: fend strikes %d, want calc1 = 12", len(r.Melees))
	}

	_, r = cf.castOn("Zeal", z)
	if len(r.Melees) != 5 {
		t.Errorf("zeal strikes %d, want 5", len(r.Melees))
	}
}
