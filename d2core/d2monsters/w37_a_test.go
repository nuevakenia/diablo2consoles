package d2monsters

import "testing"

// w37: constants whose status was relabelled (engine choice vs exe value) keep their numbers.
func TestW37ReachConstants(t *testing.T) {
	if meleeInRange != 7 || rangedInRange != 20 {
		t.Fatalf("reach constants changed: %d %d", meleeInRange, rangedInRange)
	}
}
