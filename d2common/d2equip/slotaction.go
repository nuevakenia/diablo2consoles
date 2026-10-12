package d2equip

// SlotAction is the verdict of INV_GetEquipActionForSlot (0x63eee0) and, for the hand slots,
// INV_ClassifyHandSwapAction (0x63ee20), both VERIFIED. The numbers are the exe's: the click handler
// INV_HandleBodySlotClick (0x48cbb0) switches on them.
type SlotAction int

// The actions.
const (
	// ActionNone: not allowed (the hero says a refusal line, see RefusalVoice).
	ActionNone SlotAction = 0
	// ActionPlace: the cursor item goes into the empty slot.
	ActionPlace SlotAction = 1
	// ActionPlaceMoveOther: the slot is empty but the other hand's item cannot be held together with the cursor
	// item (a two-hander against a shield): the item goes in and the OTHER hand's item ends up on the cursor
	// (packet 0x1b, ITEMACT_ServerSwapTwoHandedBody 0x561b70 puts it on the cursor, not into the inventory).
	ActionPlaceMoveOther SlotAction = 2
	// ActionPickUp: empty cursor, the slot holds an item: it goes to the cursor.
	ActionPickUp SlotAction = 3
	// ActionPickUpTwoHander: empty cursor, the slot is empty but the other hand holds a two-handed weapon: that
	// weapon is picked up.
	ActionPickUpTwoHander SlotAction = 4
	// ActionSwap: the cursor item replaces the slot's item, which goes to the cursor.
	ActionSwap SlotAction = 5
	// ActionCombine: the cursor item stacks onto the slot's item (ammunition onto ammunition, CanStack).
	ActionCombine SlotAction = 6
	// ActionSwapMoveOther: swap, and the other hand's item (not holdable with the new one) goes into the inventory;
	// only offered when it fits there (INV_CanFitItemAtBodySlot 0x63dbd0), else the action is None.
	ActionSwapMoveOther SlotAction = 7
)

// Hero voice ids of the refusal lines (UNITSND_PlayUnitSoundById 0x4c8470: 0x13 impossible, 0x14 cantuseyet).
const (
	VoiceImpossible = 0x13
	VoiceCantUse    = 0x14
)

// SlotActionFor classifies a click on a body slot. cursor is the held item (nil for an empty cursor); skipReq is the
// exe's fourth argument that skips INV_CheckItemRequirements (used to tell a requirement failure from an outright
// "does not fit"); displacedFits says whether the other hand's item has a free place in the inventory (only read for
// ActionSwapMoveOther); stackable says whether the cursor item stacks onto the slot's item (CanStack).
func (r Rules) SlotActionFor(h Hero, body map[Loc]*Item, cursor *Item, loc Loc, skipReq, displacedFits, stackable bool) SlotAction {
	slot := body[loc]

	if cursor != nil {
		if loc == LocNone || int(loc) >= NumLocs || !r.FitsLoc(cursor.Type, loc) {
			return ActionNone
		}

		if !skipReq && !Check(h, cursor, r.Types).OK() {
			return ActionNone
		}
	}

	if !loc.IsHand() {
		switch {
		case cursor == nil && slot != nil:
			return ActionPickUp
		case cursor == nil, loc == LocNone:
			return ActionNone
		case slot != nil:
			return ActionSwap
		}

		return ActionPlace
	}

	other := body[loc.Other()]

	if cursor == nil {
		switch {
		case slot != nil:
			return ActionPickUp
		case other != nil && other.Weapon && other.TwoHanded:
			return ActionPickUpTwoHander
		}

		return ActionNone
	}

	d := r.placeHand(h, body, cursor, loc)
	if !d.OK {
		return ActionNone
	}

	if slot == nil {
		if len(d.Displaced) > 0 {
			return ActionPlaceMoveOther
		}

		return ActionPlace
	}

	switch {
	case stackable:
		return ActionCombine
	case len(d.Displaced) == 0:
		return ActionSwap
	case displacedFits:
		return ActionSwapMoveOther
	}

	return ActionNone
}

// VoiceForReason maps a refusal reason to the line of RefusalVoice: strength, dexterity, level and identification
// are "cantuseyet"; a wrong class, a wrong slot, a hand pair that does not fit and no room for the displaced
// item are "impossible" (VERIFIED, INV_HandleBodySlotClick 0x48cbb0). The hero's "cantcarry" (0x17) is not played
// by the body slot or grid clicks at all.
func VoiceForReason(r Reason) int {
	switch r {
	case ReasonStrength, ReasonDexterity, ReasonLevel, ReasonUnidentified:
		return VoiceCantUse
	case ReasonOK:
		return 0
	}

	return VoiceImpossible
}

// RefusalVoice is the hero line INV_HandleBodySlotClick plays for a click it refuses (VERIFIED, 0x48cbb0): with an
// item on the cursor and ActionNone, the item that WOULD be allowed if the requirements were skipped gets
// "cantuseyet" (0x14) when it has no class restriction or the hero's class (so strength, dexterity, level or
// identification is what is missing); everything else, a wrong class, a wrong slot, a hand pair that does not fit,
// no room for the displaced item, gets "impossible" (0x13). It returns 0 when no line is played (the action is not
// None or the cursor is empty).
func (r Rules) RefusalVoice(h Hero, body map[Loc]*Item, cursor *Item, loc Loc, displacedFits, stackable bool) int {
	if cursor == nil || r.SlotActionFor(h, body, cursor, loc, false, displacedFits, stackable) != ActionNone {
		return 0
	}

	if r.SlotActionFor(h, body, cursor, loc, true, displacedFits, stackable) != ActionNone && Check(h, cursor, r.Types).ClassOK {
		return VoiceCantUse
	}

	return VoiceImpossible
}
