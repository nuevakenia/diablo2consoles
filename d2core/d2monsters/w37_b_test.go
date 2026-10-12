package d2monsters

import "testing"

// attackReach: melee reach 7 is the verified value; ranged is the engine's distance.
func TestW37AttackReach(t *testing.T) {
	if attackReach(false) != meleeInRange || attackReach(true) != rangedInRange {
		t.Fatal("attackReach mismatch")
	}
	if meleeInRange != 7 {
		t.Fatalf("meleeInRange = %d", meleeInRange)
	}
}
