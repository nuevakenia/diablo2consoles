package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

func restoreHero(health, maxHealth, mana, maxMana int) *heroUnit {
	return &heroUnit{p: &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: health, MaxHealth: maxHealth, Mana: mana, MaxMana: maxMana}}}
}

// SrvDoFunc 29 / 33 (0x5ad910 / 0x5ada80): maximum * pct / 100 whole points, clamped to the maximum; nothing when the
// stat is already full.
func TestRestorePct(t *testing.T) {
	cases := []struct {
		name          string
		mana          bool
		cur, max, pct int
		wantCur       int
		wantApplied   bool
	}{
		{"life 7 percent of 250", false, 100, 250, 7, 117, true},
		{"life clamps to the maximum", false, 245, 250, 12, 250, true},
		{"full life is untouched", false, 250, 250, 12, 250, false},
		{"mana 4 percent of 100", true, 10, 100, 4, 14, true},
		{"full mana is untouched", true, 100, 100, 8, 100, false},
	}

	for _, c := range cases {
		var h *heroUnit
		if c.mana {
			h = restoreHero(1, 1, c.cur, c.max)
		} else {
			h = restoreHero(c.cur, c.max, 1, 1)
		}

		if got := h.RestorePct(c.mana, c.pct); got != c.wantApplied {
			t.Errorf("%s: applied %v", c.name, got)
		}

		have := h.p.Stats.Health
		if c.mana {
			have = h.p.Stats.Mana
		}

		if have != c.wantCur {
			t.Errorf("%s: %d, want %d", c.name, have, c.wantCur)
		}
	}
}

// thinkCycleOfLife gate: the life cycler wants the owner below its maximum life, the mana cycler below its maximum mana.
func TestCycleNeeds(t *testing.T) {
	h := restoreHero(50, 100, 30, 30)

	if !cycleNeeds(h, false) || cycleNeeds(h, true) {
		t.Fatal("hurt but full of mana: only the life cycler works")
	}

	h = restoreHero(100, 100, 10, 30)

	if cycleNeeds(h, false) || !cycleNeeds(h, true) {
		t.Fatal("full life but low mana: only the mana cycler works")
	}
}
