package d2portal

// Ported from Game.exe 1.14b (wave 24). VERIFIED = read in the decompile.
//
//   - ITEMACT_ServerUseItem 0x55bfd0 and ITEM_ServerUseItemEffect 0x5bce10: what the scroll / tome costs.
//   - PLAYER_TeleportToTownOnTownPortalUse 0x5bbe10 (the Town Portal use function, slot 2 of the item spell table at
//     0x73e818, stride 8 = {pre-check, use}), SERVER_PlaceUnitAtNearestFreeCell 0x56ae80 and
//     SERVER_PlaceUnitInFreeCellNearLevelPoint 0x56ac70: where the two ends stand and who owns them.
//   - SERVER_UsePortalObject 0x582700 and PLAYER_DestroyOwnPortals 0x532e50: who may enter and when a pair ends.

// Object classes (objects.txt rows) of the portal objects.
const (
	ClassTownPortal      = 0x3b // the pair of a Town Portal scroll / tome
	ClassPermanentPortal = 0x3c // quest and level portals; never destroyed by use
)

// NoOwner is the owner id of a portal nobody owns (0xffffffff).
const NoOwner uint32 = 0xffffffff

// NoParty is the party id of a player who is in no party (0xffff).
const NoParty = 0xffff

// Timing, in milliseconds on the tick counter (VERIFIED: GetTickCount against the last use + 5000).
const UseCooldownMs = 5000

// Cell search masks of the exe (VERIFIED constants, meaning of the bits not decoded).
const (
	// FieldCellKind, FieldCellMaskA, FieldCellMaskB: the nearest free cell search around the caster for the field end.
	FieldCellKind  = 3
	FieldCellMaskA = 0x3e01
	FieldCellMaskB = 0xc01
	// TownCellMask is the mask of the town end search (around the arrival point, entry kind 0xb, radius up to 50 sub-tiles).
	TownCellMask   = 0xbe11
	TownEntryKind  = 0xb
	TownSearchSpan = 50
	// ArrivalCellMask is the free cell mask used by the user of a portal on arrival (0x1c09).
	ArrivalCellMask = 0x1c09
)

// ItemKind is what pays for the cast.
type ItemKind int

// The two payments. The exe tells them apart by item type: 0x12 is the book (tome), 0x16 the scroll.
const (
	ItemScroll ItemKind = iota
	ItemTome
)

// UseCost is the result of using a scroll or tome of town portal.
type UseCost struct {
	// Allowed is false when the use never starts: a tome with fewer than one scroll left (stat 0x46 < 1), nothing happens
	// and nothing is consumed.
	Allowed bool
	// Consumed is true when the item pays: a scroll is deleted, a tome loses one of its scrolls (stat 0x46 - 1).
	Consumed bool
	// TomeAfter is the tome's count after the use (unchanged unless Consumed).
	TomeAfter int
}

// ItemCost ports the cost rule. castOK is the result of the use function: false when the hero stands in a town room or in
// level 0x88, or when no free cell was found (the hero only plays the refusing mode, 0x18 / 0x13); then nothing is paid.
// VERIFIED: the charge is taken only after the use function returned success.
func ItemCost(kind ItemKind, tomeCharges int, castOK bool) UseCost {
	if kind == ItemTome && tomeCharges < 1 {
		return UseCost{Allowed: false, TomeAfter: tomeCharges}
	}

	c := UseCost{Allowed: true, TomeAfter: tomeCharges}

	if !castOK {
		return c
	}

	c.Consumed = true
	if kind == ItemTome {
		c.TomeAfter = tomeCharges - 1
	}

	return c
}

// OwnerRule is the entry rule on the owner of a portal. VERIFIED (SERVER_UsePortalObject 0x582700):
//   - an ownerless portal (0xffffffff) is free for everybody;
//   - the owner passes;
//   - anybody else needs to be in a party (party id 0xffff = none refuses);
//   - if the owner's unit is in the game it must be in the SAME party; if the owner has left the game the portal is free
//     for every player who is in some party.
func OwnerRule(owner, user uint32, userParty int, ownerPresent bool, ownerParty int) bool {
	if owner == NoOwner || owner == user {
		return true
	}

	if userParty == NoParty {
		return false
	}

	if ownerPresent && ownerParty != userParty {
		return false
	}

	return true
}

// UseOutcome is the verdict of DecideUse.
type UseOutcome int

// The verdicts.
const (
	UseOK UseOutcome = iota
	UseRefusedBusy
	UseRefusedHostile  // a player flagged by relation bit 8 may only use their own portal
	UseRefusedCooldown // less than 5 s since the last portal / waypoint use
	UseRefusedNotYours
	UseRefusedQuest // the destination level is locked by its Levels.txt quest column
)

// UseInput is what SERVER_UsePortalObject reads.
type UseInput struct {
	Busy bool // PLAYER_IsInteractionBusy
	// Hostile is PLAYER_IsAnyRelationFlag8 (UNVERIFIED which relation bit 8 is; the hostility toggle sets relation bits).
	Hostile bool
	// SinceLastUseMs is the time since the tick the player's relation record keeps (-1: never used one).
	SinceLastUseMs int64

	Class int // object class of the portal that was clicked

	PortalID uint32 // the clicked object
	LinkedID uint32 // the object at the other end (0 with HasLinked false: none)
	// HasLinked is false for a portal without a partner (a level portal whose destination sits in its object data).
	HasLinked bool

	Owner        uint32 // portal ownership id (+4 of its linked data), NoOwner if nobody
	User         uint32 // the player's unit id
	UserParty    int
	OwnerPresent bool
	OwnerParty   int

	// LastPortalID is the player's remembered portal (PlayerData +0x48), set by the cast to the FIELD end.
	LastPortalID uint32

	// QuestLocked is true when the destination level's Levels.txt quest id (QuestFlag in classic, QuestFlagEx in
	// expansion) is above 0 and bit 0 of that quest is not set in the user's quest record.
	QuestLocked bool
}

// UseResult is the verdict plus what happens to the pair.
type UseResult struct {
	Outcome UseOutcome
	// UseLinked is true when the destination is the position of the partner portal.
	UseLinked bool
	// DestroyPair is true when both ends are removed after the trip: the object class is not permanent, there is a
	// partner and the user's remembered portal is the partner. For the caster that is the trip back through the TOWN end
	// (the cast remembers the field end), so a town portal buys one round trip. VERIFIED code shape (0x582700 after the
	// move); the owner of the rule being the caster follows from 0x5bbe10 passing the field unit's id to
	// PLAYER_SetLastPortalUnitId.
	DestroyPair bool
}

// DecideUse ports the order of the checks of SERVER_UsePortalObject.
func DecideUse(in UseInput) UseResult {
	if in.Busy {
		return UseResult{Outcome: UseRefusedBusy}
	}

	if in.Hostile && in.User != in.Owner {
		return UseResult{Outcome: UseRefusedHostile}
	}

	if in.SinceLastUseMs >= 0 && in.SinceLastUseMs < UseCooldownMs {
		return UseResult{Outcome: UseRefusedCooldown}
	}

	if !OwnerRule(in.Owner, in.User, in.UserParty, in.OwnerPresent, in.OwnerParty) {
		return UseResult{Outcome: UseRefusedNotYours}
	}

	res := UseResult{UseLinked: in.HasLinked}

	if in.Class == ClassTownPortal && in.HasLinked && in.LastPortalID != in.PortalID && in.LastPortalID != in.LinkedID && in.QuestLocked {
		return UseResult{Outcome: UseRefusedQuest}
	}

	if in.HasLinked && in.Class != ClassPermanentPortal && in.LastPortalID == in.LinkedID {
		res.DestroyPair = true
	}

	return res
}
