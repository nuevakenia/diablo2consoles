package d2difficulty

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"

// GateCode is a result of the load gate SAVE_CheckCharFlagsAgainstGameMode (Game.exe 0x567a40), which decides
// whether a saved character may enter a game. VERIFIED values: the function returns exactly these numbers.
type GateCode int

// The gate results.
const (
	GateOK             GateCode = 0
	GateNeedExpansion  GateCode = 8    // expansion character, classic game
	GateNeedClassic    GateCode = 9    // classic character, expansion game
	GateHardcoreDied   GateCode = 10   // hardcore character that died (never loads)
	GateHardcoreInSoft GateCode = 0xb  // hardcore character, softcore game
	GateSoftInHardcore GateCode = 0xc  // softcore character, hardcore game
	GateNeedNormalWon  GateCode = 0xd  // Nightmare game, progression too low
	GateNeedNightmare  GateCode = 0xe  // Hell game, progression too low
	GateLadderOver     GateCode = 0x19 // ladder character past its ladder time, ladder game refused
	GateNeedLadder     GateCode = 0x1a // non-ladder (or expired) character in a ladder game
)

// GateGame describes the game the character wants to enter (the fields of the game record the exe reads).
type GateGame struct {
	Expansion  bool  // game+0x70
	Ladder     bool  // game+0x74
	Hardcore   bool  // ARENA_GetFlags() & 0x800
	Difficulty Level // game+0x6d (0 Normal, 1 Nightmare, 2 Hell)
	// Realm is true when the realm interface exists (DAT_0087bd94, i.e. Battle.net); the ladder test only runs then.
	Realm bool
	// LadderExpired is CompareFileTime(char ladder time at +400, realm clock) > 0. UNVERIFIED which time each side
	// is (the char record field is not decoded); only the comparison shape is read from the exe.
	LadderExpired bool
}

// CheckCharacterForGame ports 0x567a40, in the exe's order: expansion match, ladder, hardcore, then the
// progression needed for the game's difficulty. status is the d2s status word (flags and progression).
func CheckCharacterForGame(status uint32, g GateGame) GateCode {
	expansion := status&d2s.StatusExpansion != 0

	switch {
	case !expansion && g.Expansion:
		return GateNeedClassic
	case expansion && !g.Expansion:
		return GateNeedExpansion
	}

	if g.Realm {
		if status&d2s.StatusLadder != 0 && g.LadderExpired {
			if !g.Ladder {
				return GateLadderOver
			}
		} else if g.Ladder {
			return GateNeedLadder
		}
	}

	if status&d2s.StatusHardcore == 0 {
		if g.Hardcore {
			return GateSoftInHardcore
		}
	} else {
		if status&d2s.StatusDied != 0 {
			return GateHardcoreDied
		}

		if !g.Hardcore {
			return GateHardcoreInSoft
		}
	}

	p := Progression(status)

	switch g.Difficulty {
	case Normal:
	case Nightmare:
		if (expansion && p < 5) || p < 4 {
			return GateNeedNormalWon
		}
	default:
		if (expansion && p < 10) || p < 8 {
			return GateNeedNightmare
		}
	}

	return GateOK
}
