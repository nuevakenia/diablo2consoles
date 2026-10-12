package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
)

// DamageToManaGain is 0x5bd7c0: mana + MulDiv(total, pct, 100), floor 0, cap max, unchanged at max.
func TestDamageToManaGain(t *testing.T) {
	const max = 100 << 8

	tests := []struct {
		name        string
		mana, total int32
		pct         int
		want        int32
	}{
		{"seven percent of 50", 10 << 8, 50 << 8, 7, 10<<8 + 50*256*7/100},
		{"capped at the maximum", 98 << 8, 100 << 8, 50, max},
		{"full mana stays", max, 40 << 8, 30, max},
		{"truncates toward zero", 0, 3, 50, 1},
	}

	for _, tt := range tests {
		if got := DamageToManaGain(tt.mana, max, tt.total, tt.pct); got != tt.want {
			t.Errorf("%s: got %d want %d", tt.name, got, tt.want)
		}
	}
}

// applyElemReduce is the per-type order of 0x579c90: flat (stat 35), resist percent, absorb (percent cap 40, then flat).
func TestApplyElemReduce(t *testing.T) {
	tot := &d2statlist.Totals{MagicReduction: 4}
	tot.ResistShown[d2statlist.ResFire] = 50
	tot.ResistShown[d2statlist.ResPoison] = 50
	tot.AbsorbPct[0], tot.AbsorbFlat[0] = 60, 2 // fire: percent capped at 40
	tot.MagicResist = -100

	tests := []struct {
		name          string
		e             int
		elem          string
		wantOut, heal int
	}{
		// (20-4)=16, 50% -> 8, absorb 40% of 8 = 3.2, flat 2 -> 2.8 left, 5.2 healed (8.8, truncated at the end)
		{"fire with absorb", 20, "fire", 2, 5},
		// poison has no flat reduction and no absorb: 20 * 50% = 10
		{"poison no flat", 20, "pois", 10, 0},
		// magic: (10-4)=6 doubled by -100 resist = 12
		{"negative resist adds", 10, "mag", 12, 0},
		// flat larger than the roll leaves a negative component (the Total decides)
		{"flat exceeds", 3, "ltng", -1, 0},
		{"zero roll", 0, "fire", 0, 0},
	}

	for _, tt := range tests {
		out, heal := applyElemReduce(tt.e, tt.elem, tot)
		if out != tt.wantOut || heal != tt.heal {
			t.Errorf("%s: got %d/%d want %d/%d", tt.name, out, heal, tt.wantOut, tt.heal)
		}
	}

	if out, heal := applyElemReduce(9, "fire", nil); out != 9 || heal != 0 {
		t.Errorf("nil totals: %d/%d", out, heal)
	}
}
