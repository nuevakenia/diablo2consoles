package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// w37: constants whose status was relabelled (engine choice vs exe value) keep their numbers.
func TestW37ReachConstants(t *testing.T) {
	if meleeInRange != 7 || rangedInRange != 20 {
		t.Fatalf("reach constants changed: %d %d", meleeInRange, rangedInRange)
	}
}

// w37: the level threat is off without a host callback, and reads the callback with the brain level otherwise.
func TestW37LevelThreat(t *testing.T) {
	d := &Director{}
	b := &d2monster.Brain{LevelID: 5}

	if d.LevelThreat(b) != 0 {
		t.Fatal("threat must be 0 without a callback")
	}

	d.opt.LevelThreat = func(id int) int { return id * 2 }

	if d.LevelThreat(b) != 10 {
		t.Fatal("threat must follow the callback")
	}
}
