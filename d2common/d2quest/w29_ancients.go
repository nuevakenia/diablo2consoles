package d2quest

// Wave 29: the state machine of the Rite of Passage fight (the Arreat Summit altar, the statues, the Ancients). The
// exe facts are in w27_quests2.go (0x58b1e0, 0x58ace0, 0x58ad30, 0x589f10, 0x58a7b0, 0x58a600, 0x58a4f0); this file
// only turns them into one value the engine drives. Anything beyond those facts is marked UNVERIFIED.

// Ancient classes and the statue objects that spawn them (VERIFIED: notes w27-quests2.md).
const (
	AncientCount  = 3
	LevelSummit   = LevelArreatSummit // the level of the fight (120, 0x78 in the exe)
	AncientsAltar = 546               // objects.txt row of the Altar of the Heavens (UNVERIFIED as the 0x58b1e0 object id)
	AncientsDoor  = 547               // objects.txt row of the door to the Worldstone Keep (UNVERIFIED as the gate object)
)

// IsAncientClass says whether a monster class is one of the three Ancients (540, 541, 542).
func IsAncientClass(class int) bool { return class >= NPCAncient1 && class <= NPCAncient3 }

// AncientsFight is the per-game state of the fight. The zero value is a fresh game.
type AncientsFight struct {
	Portals int  // town portals open in level 120 (VERIFIED 0x58ace0: the portal counter)
	Spawned int  // Ancients woken so far
	Deaths  int  // Ancients killed
	Armed   bool // the statues were woken (the fight is on, the Ancients attack)
	Done    bool // the third Ancient died

	rearmOnClose bool
}

// MayStart says whether a statue may wake its Ancient now.
func (f *AncientsFight) MayStart() bool { return !f.Done && AncientsMayStart(f.Portals, f.Spawned) }

// Wake records one Ancient woken and arms the fight.
func (f *AncientsFight) Wake() {
	f.Spawned++
	f.Armed = true
}

// PortalsChanged sets the number of town portals open in level 120. VERIFIED 0x58ace0: a portal while the fight is
// not done clears the armed flag and resets the Ancients (reset is true); closing the last one re-arms a fight that
// was armed (rearm is true; the caller wakes the Ancients again).
func (f *AncientsFight) PortalsChanged(open int) (reset, rearm bool) {
	if f.Done {
		f.Portals = open

		return false, false
	}

	was := f.Portals
	f.Portals = open

	switch {
	case was == 0 && open > 0:
		wasArmed := f.Armed
		f.Armed, f.Spawned, f.Deaths = false, 0, 0
		f.rearmOnClose = wasArmed

		return true, false
	case was > 0 && open == 0 && f.rearmOnClose:
		f.rearmOnClose = false

		return false, true
	}

	return false, false
}

// AncientDied records a death; it reports true for the third one, which ends the fight (VERIFIED 0x58a7b0).
func (f *AncientsFight) AncientDied() bool {
	if f.Done {
		return false
	}

	f.Deaths++
	if f.Deaths >= AncientCount {
		f.Done, f.Armed = true, false

		return true
	}

	return false
}

// Running says whether the fight is on and not finished: the altar refuses then (VERIFIED 0x58b1e0).
func (f *AncientsFight) Running() bool { return f.Armed && !f.Done }

// AltarWorks combines the hero rule and the fight rule of the altar.
func (f *AncientsFight) AltarWorks(riteDone, ritePending bool) bool {
	return RiteAltarOpens(riteDone, ritePending) && !f.Running()
}

// RiteRewardRecipient says whether a hero gets the Rite reward when the third Ancient dies: standing in level 120
// (or a party member of a hero who does, anywhere in Act 5), at least level 20*(difficulty+1), and without the done
// bit (VERIFIED 0x58a7b0, 0x58a600, 0x58a4f0).
func RiteRewardRecipient(level, difficulty int, inSummitOrPartyInAct5, alreadyDone bool) bool {
	return inSummitOrPartyInAct5 && !alreadyDone && level >= RiteLevelGate(difficulty)
}
