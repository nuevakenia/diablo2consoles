package d2playertrade

import (
	"errors"
	"time"
)

// Session rules of the original player trade, read from the server functions
// of PlrTrade (w37): TRADE_ServerHandleTradeRequest 0x564b60,
// TRADE_CanChangeEquipmentDuringTrade 0x5655b0, TRADE_ServerOfferItemToTradeWindow
// 0x5664e0, TRADE_ServerFinishTrade 0x565750, TRADE_ServerAdvanceTradeState 0x566750.

// RecentTradeCooldown: a hero whose trade finished less than 5000 ms ago (the
// tick stored in the player data when the trade finished, 0x565750) can neither
// ask for nor be asked for a new trade; the asker gets status 9 (VERIFIED 0x564b60).
const RecentTradeCooldown = 5 * time.Second

// Request refusals.
var (
	ErrNotInTown = errors.New("both players must be in town to trade")
	ErrTradeBusy = errors.New("a player is busy with another trade")
	ErrTradeSoon = errors.New("a trade finished a moment ago")
)

// RequestCtx describes the two heroes at the moment of a trade request.
type RequestCtx struct {
	AskerInTown, TargetInTown bool
	// AskerBusy, TargetBusy: the hero is already in a trade state (player data
	// state field != 0, 0x564b60 tests both).
	AskerBusy, TargetBusy bool
	// SinceAsker, SinceTarget: time since that hero's last finished trade
	// (a large value if it never traded).
	SinceAsker, SinceTarget time.Duration
}

// CheckRequest applies the request rules of 0x564b60: the cooldown of either
// hero is tested first (status 9), then the busy state, then the town test:
// both heroes must stand in a town room (LEVEL_IsTownLevelIdFwd). When the town
// test fails the exe clears both heroes' target and nothing else happens.
func CheckRequest(c RequestCtx) error {
	switch {
	case c.SinceTarget < RecentTradeCooldown || c.SinceAsker < RecentTradeCooldown:
		return ErrTradeSoon
	case c.AskerBusy || c.TargetBusy:
		return ErrTradeBusy
	case !c.AskerInTown || !c.TargetInTown:
		return ErrNotInTown
	}

	return nil
}

// CanChangeEquipment is TRADE_CanChangeEquipmentDuringTrade (VERIFIED 0x5655b0):
// while a player trade is open (the target reference is a player) the hero may
// not change what it wears if any item of its is on inventory page 2 (the trade
// window); nothing else stops it.
func CanChangeEquipment(tradeOpen bool, itemsInTradeWindow int) bool {
	return !(tradeOpen && itemsInTradeWindow > 0)
}

// OfferResetsAccept: adding an item to the window while either side already
// accepted (state 4) sends the 0x79 packet twice per hero (accept cleared on
// both) and returns both to state 3 (VERIFIED 0x5664e0). SetOffer revokes both
// acceptances, which is the same rule.
func OfferResetsAccept(stateA, stateB int) bool { return stateA == 4 || stateB == 4 }
