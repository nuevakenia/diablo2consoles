package d2skill

import "testing"

func TestMissileToHitPlayerIgnoresSkillColumn(t *testing.T) {
	f := newFixture(map[string]int{"Attack": 1})
	f.u.stats["item_tohit_percent"] = 35

	f.u.player = true
	if got := missileToHit(f.u, 100); got != 35 {
		t.Errorf("player missile to-hit %d, want the stat 0x77 percent 35 only", got)
	}

	f.u.player = false
	if got := missileToHit(f.u, 100); got != 100 {
		t.Errorf("non-player kept %d, want 100", got)
	}
}
