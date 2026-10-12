package d2combat

import "testing"

// Wave 31: open wounds damage per attacker level and the PvP damage scale (exe: callback 0x5bda80 with helper
// 0x5bd9c0; pre-step 0x579d70 of the resist routine).

func TestOpenWoundsPerLevel(t *testing.T) {
	// per frame value (the life regeneration stat 0x4a, 8.8 fixed point) at the attacker level; the slopes are
	// 9, 18, 27, 36, 45 per level between the breakpoints 15, 30, 45, 60, plus 40.
	for _, tc := range []struct{ level, perFrame int }{
		{1, 40}, {2, 49}, {15, 166}, {16, 184}, {30, 436}, {45, 841}, {60, 1381}, {61, 1426}, {98, 3091}, {99, 3136},
	} {
		if got := OpenWoundsBaseExe(tc.level, [5]int{9, 18, 27, 36, 45}) + 40; got != tc.perFrame {
			t.Errorf("level %d: %d per frame, want %d", tc.level, got, tc.perFrame)
		}
	}

	// total damage of one proc: the state lasts 200 frames and each frame removes value/256 life
	total := func(level int, kind int, flagC bool, ev int) int {
		o := RollOpenWoundsExe(fixedRoller(0), OpenWoundsInput{Chance: 100, AttackerLevel: level, DefenderKind: kind, DefenderFlagC: flagC, Event: ev})
		if !o.Hit {
			t.Fatalf("no hit at level %d", level)
		}

		return o.Value * OpenWoundsLength / 256
	}

	for _, tc := range []struct {
		level, kind int
		flagC       bool
		ev          int
		want        int
	}{
		{1, 1, false, 0, 31},    // 40 per frame on a plain monster
		{30, 1, false, 0, 340},  // 436 per frame
		{99, 1, false, 0, 2450}, // 3136 per frame
		{99, 1, true, 0, 1225},  // champion or unique: halved
		{99, 0, false, 0, 612},  // against a player a quarter: 784 per frame
		{99, 0, false, EventMissile, 306},
	} {
		if got := total(tc.level, tc.kind, tc.flagC, tc.ev); got != tc.want {
			t.Errorf("level %d kind %d flagC %v event %d: total %d, want %d", tc.level, tc.kind, tc.flagC, tc.ev, got, tc.want)
		}
	}

	// a level below 1 counts as 1
	if a, b := total(0, 1, false, 0), total(1, 1, false, 0); a != b {
		t.Errorf("level 0: %d vs %d", a, b)
	}
}

type fixedRoller int

func (f fixedRoller) Roll(n int32) uint32 { return uint32(int32(f) % n) }

func TestPvPScaleConstants(t *testing.T) {
	// exe 0x579d70: a player (or a mercenary / flagged unit) hitting a player deals 17 percent (0x11); a monster
	// hitting a player 100; two player controlled combatants where the defender is not a player 25 (0x19).
	for _, tc := range []struct {
		name                               string
		same, aPlayer, dPlayer, aCtl, dCtl bool
		want                               int
	}{
		{"same unit", true, true, true, true, true, 100},
		{"hero hits hero", false, true, true, true, true, 17},
		{"monster hits hero", false, false, true, false, true, 100},
		{"mercenary hits hero", false, false, true, true, true, 17},
		{"merc hits pet", false, false, false, true, true, 25},
		{"hero hits monster", false, true, false, true, false, 100},
	} {
		if got := CombatantScalePercent(tc.same, tc.aPlayer, tc.dPlayer, tc.aCtl, tc.dCtl); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}

	if PvPPercent() != 17 {
		t.Errorf("pvp percent %d", PvPPercent())
	}

	// raw 1000 -> 170 before the flat reduction and the physical resist
	if got := PvPDamage(1000); got != 170 {
		t.Errorf("pvp damage %d", got)
	}

	if got := PvPReceive(PvPDamage(1000), 50, 20); got != 75 {
		t.Errorf("pvp receive %d", got)
	}
}
