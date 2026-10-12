package d2difficulty

import "fmt"

// String names a gate result for logs and the refusal text.
func (c GateCode) String() string {
	switch c {
	case GateOK:
		return "ok"
	case GateNeedExpansion:
		return "an expansion character cannot enter a classic game"
	case GateNeedClassic:
		return "a classic character cannot enter an expansion game"
	case GateHardcoreDied:
		return "this hardcore character has died"
	case GateHardcoreInSoft:
		return "a hardcore character cannot enter a softcore game"
	case GateSoftInHardcore:
		return "a softcore character cannot enter a hardcore game"
	case GateNeedNormalWon:
		return "the character has not finished Normal"
	case GateNeedNightmare:
		return "the character has not finished Nightmare"
	case GateLadderOver:
		return "the ladder character's ladder period is over"
	case GateNeedLadder:
		return "only a ladder character can enter a ladder game"
	}

	return fmt.Sprintf("gate result %d", int(c))
}

// JoinResult maps a gate result to the join result code the table at 0x6e2500 gives it (numbers read in wave 23; the
// table is used by SAVE_ParseD2sModernFormat 0x5690cf; the strings behind the codes were not read). Unlisted results map
// to themselves.
func (c GateCode) JoinResult() int {
	switch c {
	case GateNeedExpansion:
		return 24
	case GateNeedClassic:
		return 23
	case GateHardcoreDied:
		return 21
	case GateHardcoreInSoft:
		return 20
	case GateSoftInHardcore:
		return 19
	case GateNeedNormalWon:
		return 17
	case GateNeedNightmare:
		return 18
	case GateLadderOver:
		return 26
	case GateNeedLadder:
		return 25
	}

	return int(c)
}
