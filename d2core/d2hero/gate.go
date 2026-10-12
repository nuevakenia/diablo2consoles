package d2hero

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2difficulty"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

// The load gate (SAVE_CheckCharFlagsAgainstGameMode 0x567a40, d2difficulty.CheckCharacterForGame) works on the status word
// of the character and on the flags of the game. This file gives a hero the status word and the game of a local session,
// so that every hero carries the flags the gate reads, whether it was imported from a .d2s or made in this engine.

// GateStatus is the d2s status word of the hero as the gate reads it: expansion, hardcore, died, ladder and the
// progression (bits 8..12). Heroes with a .d2s take the flags of its header; heroes made in this engine carry them in the
// HeroState (CreateNewHero stores them and writes them to the new .d2s). The progression is the larger of the stored one and
// the one the quest records give (ComputedProgression).
func (s *HeroState) GateStatus() uint32 {
	exp, hc, lad := s.Expansion, s.Hardcore, s.Ladder
	dead := s.IsDeadHardcore()

	if s.Imported != nil {
		exp, hc, lad = s.Imported.Expansion, s.Imported.Hardcore || s.Hardcore, s.Imported.Ladder || s.Ladder
		dead = dead || (s.Imported.Dead && hc)
	}

	var st uint32

	if exp {
		st |= d2s.StatusExpansion
	}

	if hc {
		st |= d2s.StatusHardcore
	}

	if dead && hc {
		st |= d2s.StatusDied
	}

	if lad {
		st |= d2s.StatusLadder
	}

	return d2difficulty.WithProgression(st, s.ComputedProgression())
}

// LocalGame is the game record of a single player session of this hero at a difficulty: the game mode follows the
// character (the menus build the game from the character's own expansion, hardcore and ladder flags; UNVERIFIED for the
// original's single player path, the shape of the record is VERIFIED), and there is no realm, so the ladder test does not
// run.
func (s *HeroState) LocalGame(l d2difficulty.Level) d2difficulty.GateGame {
	st := s.GateStatus()

	return d2difficulty.GateGame{
		Expansion:  st&d2s.StatusExpansion != 0,
		Ladder:     st&d2s.StatusLadder != 0,
		Hardcore:   st&d2s.StatusHardcore != 0,
		Difficulty: l,
	}
}

// CheckForGame runs the gate for a game.
func (s *HeroState) CheckForGame(g d2difficulty.GateGame) d2difficulty.GateCode {
	return d2difficulty.CheckCharacterForGame(s.GateStatus(), g)
}

// CheckLocal runs the gate for a single player session at a difficulty.
func (s *HeroState) CheckLocal(l d2difficulty.Level) d2difficulty.GateCode {
	return s.CheckForGame(s.LocalGame(l))
}
