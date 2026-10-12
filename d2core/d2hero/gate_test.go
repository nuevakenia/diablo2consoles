package d2hero

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2difficulty"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

// finishDifficulty marks the five act bosses done in a difficulty.
func finishDifficulty(s *HeroState, diff int) {
	q := &s.EnsureProgress().Quests[diff]
	for act := 1; act <= 5; act++ {
		quest := 6
		if act == 4 {
			quest = 2
		}

		q.SetCompleted(act, quest)
	}
}

func TestHeroGate(t *testing.T) {
	type tc struct {
		name string
		mk   func() *HeroState
		diff d2difficulty.Level
		want d2difficulty.GateCode
	}

	fresh := func() *HeroState { return &HeroState{HeroName: "A", Expansion: true} }

	tests := []tc{
		{"fresh normal", fresh, d2difficulty.Normal, d2difficulty.GateOK},
		{"fresh nightmare", fresh, d2difficulty.Nightmare, d2difficulty.GateNeedNormalWon},
		{"fresh hell", fresh, d2difficulty.Hell, d2difficulty.GateNeedNightmare},
		{"normal finished opens nightmare", func() *HeroState {
			h := fresh()
			finishDifficulty(h, 0)

			return h
		}, d2difficulty.Nightmare, d2difficulty.GateOK},
		{"nightmare finished opens hell", func() *HeroState {
			h := fresh()
			finishDifficulty(h, 0)
			finishDifficulty(h, 1)

			return h
		}, d2difficulty.Hell, d2difficulty.GateOK},
		{"dead hardcore", func() *HeroState {
			h := fresh()
			h.Hardcore = true
			h.Death = &DeathState{Died: true}

			return h
		}, d2difficulty.Normal, d2difficulty.GateHardcoreDied},
		{"living hardcore in its own game", func() *HeroState {
			h := fresh()
			h.Hardcore = true

			return h
		}, d2difficulty.Normal, d2difficulty.GateOK},
		{"imported flags win", func() *HeroState {
			h := &HeroState{HeroName: "B", Imported: &ImportedInfo{Expansion: true, Hardcore: true, Dead: true}}

			return h
		}, d2difficulty.Normal, d2difficulty.GateHardcoreDied},
	}

	for _, c := range tests {
		if got := c.mk().CheckLocal(c.diff); got != c.want {
			t.Errorf("%s: %v (%#x), want %v", c.name, got, int(got), c.want)
		}
	}
}

func TestHeroGateAgainstOtherGames(t *testing.T) {
	h := &HeroState{HeroName: "C", Expansion: true}

	// joining somebody else's game: the hero's flags against the host game's
	tests := []struct {
		name string
		g    d2difficulty.GateGame
		want d2difficulty.GateCode
	}{
		{"classic game", d2difficulty.GateGame{Expansion: false}, d2difficulty.GateNeedExpansion},
		{"hardcore game", d2difficulty.GateGame{Expansion: true, Hardcore: true}, d2difficulty.GateSoftInHardcore},
		{"normal game", d2difficulty.GateGame{Expansion: true}, d2difficulty.GateOK},
	}

	for _, c := range tests {
		if got := h.CheckForGame(c.g); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestGateStatusBits(t *testing.T) {
	h := &HeroState{Expansion: true, Hardcore: true, Ladder: true}
	st := h.GateStatus()

	for _, bit := range []uint32{d2s.StatusExpansion, d2s.StatusHardcore, d2s.StatusLadder} {
		if st&bit == 0 {
			t.Errorf("status %#x lacks %#x", st, bit)
		}
	}

	if st&d2s.StatusDied != 0 {
		t.Error("died bit on a living hero")
	}

	finishDifficulty(h, 0)

	if p := d2difficulty.Progression(h.GateStatus()); p != 5 {
		t.Errorf("progression %d, want 5", p)
	}
}

func TestGateCodeText(t *testing.T) {
	for _, c := range []d2difficulty.GateCode{8, 9, 10, 0xb, 0xc, 0xd, 0xe, 0x19, 0x1a} {
		if c.String() == "" || c.JoinResult() == int(c) {
			t.Errorf("code %#x: %q join %d", int(c), c.String(), c.JoinResult())
		}
	}

	if d2difficulty.GateOK.JoinResult() != 0 {
		t.Error("ok must stay 0")
	}
}
