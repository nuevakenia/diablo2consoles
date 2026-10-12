package d2player

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

// Spending attribute points on the character screen. UNVERIFIED against Game.exe (Ghidra was not reachable in
// wave 36): the add-stat button handler UI_HandleStatButtonMouseDownAlt 0x4a2ef0 and the allocate packet
// (0x3a, SERVER_HandleC2S3A_AllocateStatPoints 0x549b40) were not read. The multiples below are the long-standing
// behaviour of 1.10+ clients as remembered from play, not from the exe.
const (
	statSpendShiftPoints = 5 // UNVERIFIED: shift-click spends five points at once
)

// StatSpendCount returns how many points one click on an add-stat button spends, given the held modifiers and
// the points still unspent: a plain click spends 1, shift 5, ctrl all of them (UNVERIFIED), never more than remain.
func StatSpendCount(mod d2enum.KeyMod, remaining int) int {
	if remaining <= 0 {
		return 0
	}

	n := 1

	switch {
	case mod&d2enum.KeyModControl != 0:
		n = remaining
	case mod&d2enum.KeyModShift != 0:
		n = statSpendShiftPoints
	}

	if n > remaining {
		n = remaining
	}

	return n
}

// StatColour names the text colour of an attribute value against its base: UNVERIFIED which palette entries.
type StatColour int

// The attribute value colours.
const (
	StatColourNormal    StatColour = iota // current == base
	StatColourIncreased                   // items or states raise it (blue in the original, UNVERIFIED)
	StatColourDecreased                   // a penalty lowers it (red, UNVERIFIED)
)

// StatValueColour compares a displayed attribute to its base value.
func StatValueColour(current, base int) StatColour {
	switch {
	case current > base:
		return StatColourIncreased
	case current < base:
		return StatColourDecreased
	}

	return StatColourNormal
}
