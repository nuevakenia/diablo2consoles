package d2state

// StateAttractID is state 0x39 (row 57 of states.txt) and AttractName its name.
const (
	StateAttractID = 0x39
	AttractName    = "attract"
)

// CurseGate is the curse branch of SKILL_CreateTimedStateStatList (0x56c740), VERIFIED: for a state in the
// curse mask, curse_resistance (stat 0x6d) of 100 or more refuses, a lower nonzero value shortens the length
// by MulDiv(length, resist, 100), and a unit that has state 0x39 (Attract) refuses AFTER that; a non-curse state
// is never checked for it. It returns the final length and whether the state may be created.
func CurseGate(curse bool, curseResist, frames int, hasAttract bool) (int, bool) {
	if !curse {
		return frames, true
	}

	if curseResist >= ImmuneResist {
		return 0, false
	}

	if curseResist != 0 {
		frames = CurseLength(frames, curseResist)
	}

	if hasAttract {
		return 0, false
	}

	return frames, true
}

// attractRefuses says whether a unit that carries an active Attract refuses the state: it does for every state
// of the curse column (Attract itself included, so a second Attract does not refresh the first).
func (s *Set) attractRefuses(frame int, name string) bool {
	d, ok := s.defs[name]
	if !ok || !d.Curse || isShrine(name) {
		return false
	}

	return s.Active(frame, AttractName)
}
