// Package d2groundexp holds the lifetime rules of items on the ground and of the
// saved (inactive) room entries, ported from Game.exe 1.14b.
//
// VERIFIED: ITEM_ComputeGroundExpiryFrame (0x556a80) gives the frame an item
// disappears; ITEM_PurgeExpiredGroundItems (0x556c00) removes live ground items
// whose stamp is non-zero and not above the game frame; SUNITINACTIVE_PurgeExpiredEntries
// (0x5407e0) removes saved items with a stamp non-zero and below the frame;
// GAME_PurgeInactiveUnitsPeriodic (0x52ad80) runs both for the five acts when
// the game frame is a multiple of 1500, so an item can outlive its stamp by up
// to 1499 frames.
package d2groundexp

// PurgePeriodFrames is the interval of the periodic purge (0x5dc).
const PurgePeriodFrames = 1500

// Lifetimes in 25 Hz frames (0x556a80).
const (
	NormalFrames    = 15000 // 10 minutes: normal, low, superior items, small gold
	MagicFrames     = 30000 // 20 minutes: magic items and socket fillers (gems, runes, jewels)
	SpecialFrames   = 45000 // 30 minutes: set, rare, unique, crafted, tempered; gold piles above BigGold
	BigGold         = 10000
	qualityMagic    = 4
	qualityFirstTop = 5 // 5..9 are the special qualities
	qualityLastTop  = 9
)

// Kind is what the stamp rule reads from an item. Quality uses the exe's
// numbering (1 low .. 4 magic, 5 set, 6 rare, 7 unique, 8 crafted, 9 tempered).
type Kind struct {
	Quest        bool // quest items never expire
	Quality      int
	Gold         bool // a gold pile (the exe treats it as an item of type 4)
	GoldAmount   int
	SocketFiller bool // itemtypes ancestor "sock" (type index 0x35)
}

// ExpiryFrame returns the frame at which an item dropped at frame now expires,
// or 0 for never.
func ExpiryFrame(now int, k Kind) int {
	if k.Quest {
		return 0
	}

	if k.Quality == qualityMagic {
		return now + MagicFrames
	}

	if k.Quality >= qualityFirstTop && k.Quality <= qualityLastTop {
		return now + SpecialFrames
	}

	if k.Gold {
		if k.GoldAmount > BigGold {
			return now + SpecialFrames
		}

		return now + NormalFrames
	}

	if k.SocketFiller {
		return now + MagicFrames
	}

	return now + NormalFrames
}

// IsPurgeFrame reports whether the periodic purge runs at this game frame.
func IsPurgeFrame(frame int) bool { return frame%PurgePeriodFrames == 0 }

// LiveExpired is the test of the live ground purge: stamp non-zero and <= frame.
func LiveExpired(stamp, frame int) bool { return stamp != 0 && stamp <= frame }

// SavedExpired is the test of the saved-entry purge: stamp non-zero and < frame.
func SavedExpired(stamp, frame int) bool { return stamp != 0 && stamp < frame }
