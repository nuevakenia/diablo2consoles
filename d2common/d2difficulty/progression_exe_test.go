package d2difficulty

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

// The progression byte and the difficulty gates, pinned to Game.exe 1.14b: CHARINFO_RaiseProgression 0x5360d0
// (progression = mult * difficulty + act, mult 5 in the expansion and 4 in classic, kept when not lower) and
// SAVE_CheckCharFlagsAgainstGameMode 0x567a40 (Nightmare needs 5 / 4, Hell needs 10 / 8).
func TestProgressionRaiseMatchesTheExe(t *testing.T) {
	for _, c := range []struct {
		stored, act, diff int
		exp               bool
		want              int
	}{
		{0, 1, 0, true, 1},   // Andariel in Normal
		{4, 5, 0, true, 5},   // Baal in Normal
		{5, 1, 1, true, 6},   // Andariel in Nightmare
		{9, 5, 1, true, 10},  // Baal in Nightmare
		{14, 4, 2, true, 14}, // Diablo in Hell (4 + 10), equal: kept
		{15, 4, 2, true, 15}, // never lowered
		{3, 4, 0, false, 4},  // Diablo in classic Normal
		{4, 1, 1, false, 5},
		{8, 4, 1, false, 8},
	} {
		if got := RaiseProgression(c.stored, c.act, c.diff, c.exp); got != c.want {
			t.Errorf("RaiseProgression(%d, act %d, diff %d, exp %v) = %d, want %d", c.stored, c.act, c.diff, c.exp, got, c.want)
		}
	}
}

func TestDifficultyGatesMatchTheExe(t *testing.T) {
	var empty [3]d2s.QuestRecord

	for _, c := range []struct {
		exp                        bool
		nightmareMin, hellMin, max int
	}{{true, 5, 10, 15}, {false, 4, 8, 12}} {
		if GateProgression(Normal, c.exp) != 0 || GateProgression(Nightmare, c.exp) != c.nightmareMin || GateProgression(Hell, c.exp) != c.hellMin {
			t.Errorf("gates exp=%v: %d %d %d", c.exp, GateProgression(Normal, c.exp), GateProgression(Nightmare, c.exp), GateProgression(Hell, c.exp))
		}

		for p := 0; p <= c.max; p++ {
			u := Unlocked(empty, p, c.exp)

			if !u[Normal] || u[Nightmare] != (p >= c.nightmareMin) || u[Hell] != (p >= c.hellMin) {
				t.Errorf("progression %d exp=%v: unlocked %v", p, c.exp, u)
			}
		}
	}
}
