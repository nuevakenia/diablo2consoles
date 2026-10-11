package d2skill

import "testing"

func TestEnergyShieldWalk(t *testing.T) {
	// 50 percent, ratio 16 (1 mana per damage point), plenty of mana
	abs, left, ended := EnergyShieldWalk([8]int{100, 40, 0, 0, 0, 20, 0, 0}, 50, 16, 1000, false)
	if abs != [8]int{50, 20, 0, 0, 0, 10, 0, 0} || left != 920 || ended {
		t.Errorf("monster attacker: %v %d %v", abs, left, ended)
	}

	// player attacker: the three trailing fields are skipped
	abs, left, _ = EnergyShieldWalk([8]int{100, 40, 0, 0, 0, 20, 0, 0}, 50, 16, 1000, true)
	if abs[5] != 0 || left != 930 {
		t.Errorf("player attacker: %v %d", abs, left)
	}

	// mana runs out part way: the first type takes all, the second gets nothing and the shield ends
	abs, left, ended = EnergyShieldWalk([8]int{100, 100, 0, 0, 0, 0, 0, 0}, 100, 16, 100, false)
	if abs[0] != 100 || abs[1] != 0 || left != 0 || !ended {
		t.Errorf("exhaust: %v %d %v", abs, left, ended)
	}

	// ratio above 16 makes each point costlier; mana limits the first type
	abs, left, _ = EnergyShieldWalk([8]int{100}, 100, 32, 40, false)
	if abs[0] != 20 || left != 0 {
		t.Errorf("ratio: %v %d", abs, left)
	}
}
