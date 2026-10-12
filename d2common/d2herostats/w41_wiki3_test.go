package d2herostats

import "testing"

// Wiki round 3 (w41): kill experience level-difference rules (exe 0x57c300).
func TestW41KillXPLevelRules(t *testing.T) {
	const xp = 25600

	// Below clvl 25 a monster above the character uses the 1/256 table.
	for d, want := range []int{256, 256, 256, 256, 256, 256, 225, 174, 92, 38, 5} {
		if got := LevelScaleXP(xp, 20+d, 20); got != xp*want/256 {
			t.Errorf("monster +%d: %d", d, got)
		}
	}

	// A far-above monster is capped at the +10 entry.
	if LevelScaleXP(xp, 60, 20) != xp*5/256 {
		t.Error("monster +40 below clvl 25 uses the +10 entry")
	}

	// From clvl 25 a monster above the character scales by clvl/mlvl.
	if got := LevelScaleXP(xp, 60, 30); got != xp*30/60 {
		t.Errorf("clvl 30 vs mlvl 60: %d", got)
	}

	// Character above the monster: 100% up to 5, then 81/62/43/24/5 percent.
	for d, want := range []int{256, 256, 256, 256, 256, 256, 207, 159, 110, 61, 13, 13} {
		if got := LevelScaleXP(xp, 50-d, 50); got != xp*want/256 {
			t.Errorf("char +%d: %d", d, got)
		}
	}

	// Recipients at max level get nothing; a zero experience monster still counts 1.
	if KillXP(1000, 80, 99, 99, 0) != 0 || KillXP(0, 80, 50, 99, 0) != 1 {
		t.Error("max level recipient 0, zero xp 1")
	}
}
