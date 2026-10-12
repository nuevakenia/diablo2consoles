package d2quest

// Wave 27 leftovers of the quest audit (second pass over Game.exe 1.14b, read-only Ghidra). Addresses are in my own
// words; VERIFIED means read in the exe, UNVERIFIED is called out where it applies.

// Game type byte of the server game (Game+0x6a, written by GAME_CreateServerGame 0x52e680 from the create request).
// VERIFIED: the client (MSG_SendCreateGameRequest 0x4739e0) sends 3 when its network mode (DAT_00798698) is 0, 1 for
// mode 6, 2 for mode 8 and 0 for every other mode; the game-session loop (0x44aa80) gives modes 6 and 8 a server
// thread of their own and Warden only to modes 2 and 3. GAME_CreateServerGame sets client flag 0x20 for type 1.
// UNVERIFIED: which menu entry maps to which mode. The type that gives Anya the higher tier (3) is the mode 0 one,
// which has neither a server thread nor Warden, so it is not the local host game this port always plays (modelled
// as type 1, GameTypeHost).
const (
	GameTypeRealm = 0 // mode 2 and others
	GameTypeHost  = 1 // mode 6: the host with its own server thread (the local game of this port)
	GameTypeOpen  = 2 // mode 8
	GameTypeMode0 = 3 // mode 0
)

// AnyaTier is the index of Anya's item table in anyaRewards (0 normal, 1 exceptional, 2 elite). VERIFIED 0x587460
// (message 0x4ea8): Nightmare gives the exceptional list when the game type is 3 or the hero level is above 45 (0x2d),
// Hell the elite list when the game type is 3 or the level is above 65 (0x41); everything else the normal list.
func AnyaTier(difficulty, heroLevel, gameType int) int {
	switch {
	case difficulty == Nightmare && (gameType == GameTypeMode0 || heroLevel > 45):
		return 1
	case difficulty == Hell && (gameType == GameTypeMode0 || heroLevel > 65):
		return 2
	}

	return 0
}

// AnyaRewardCodesFor is AnyaRewardCodes with the game type byte of the game (Game+0x6a).
func AnyaRewardCodesFor(class, difficulty, heroLevel, gameType int) []string {
	if class < 0 || class >= len(anyaRewards) {
		return nil
	}

	return anyaRewards[class][AnyaTier(difficulty, heroLevel, gameType)]
}

// EveGoldRange is the gold of one pile paid by Baal's death: VERIFIED 0x58bce0 (disassembly 0x58bdc4..0x58be1a):
// the roll is taken between difficulty*6000+1500 and difficulty*6000+3000 (the upper end capped to 0xffff) and made
// into standard gold piles at the hero.
func EveGoldRange(difficulty int) (lo, hi int) {
	lo = difficulty*6000 + 1500
	hi = difficulty*6000 + 3000

	if hi > 0xffff {
		hi = 0xffff
	}

	return lo, hi
}

// EveGoldReward is the effect that carries the Baal gold; the engine rolls between Value (low) and Quality (high).
// VERIFIED 0x58bce0: one pile per hero granted by this kill (the node counter at +0x88 that 0x58bae0 bumps), and only
// when the killer did not have the quest done (slot 40 bit 0) before the kill.
func EveGoldReward(difficulty int) Effect {
	lo, hi := EveGoldRange(difficulty)

	return Effect{Kind: EffectReward, Code: "gold-pile", Value: lo, Quality: hi,
		Note: "Baal's death drops gold piles on the hero (VERIFIED 0x58bce0)"}
}

// Betrayal of Harrogath, the Nihlathak kill (0x589600 with the iterators 0x589300, 0x5894e0, 0x589530, 0x589590).
// VERIFIED: a hero in or next to the corpse room whose slot-38 bits 14, 0 and 1 are clear and whose Prison of Ice
// (slot 37) is done or pending gets reward pending and the goal bit; party members of a hero with the goal bit get the
// same anywhere in Act 5 under the same Prison rule; every hero that ends without bit 0, bit 1 or the goal bit gets
// bit 14 ("missed it"), which also blocks any later grant (0x589300 and 0x5894e0 refuse bit 14).
const betrayalMissedBit = FlagCompletedNow // slot-38 bit 14

// BetrayalEligible says whether the Nihlathak kill grants the quest to a hero with this Prison of Ice state.
func BetrayalEligible(prisonDoneOrPending, missedBit bool) bool {
	return prisonDoneOrPending && !missedBit
}

// Rite of Passage, the Ancients and the summit gate.
//
// VERIFIED (0x58b1e0, the altar object): the altar that opens the way to the Worldstone Keep works only for a hero
// whose Rite record has bit 0 or bit 1 (done / reward pending), and not while the fight of this game is still going
// (the node's done byte clear and the node active, or a fight flag at +0x48 set while the done byte is clear). A hero
// without the bits gets the object reset and nothing happens.
//
// VERIFIED (0x58ace0, 0x58ad30, 0x589f10): a town portal opened inside level 120 while the fight is not done counts
// up a portal counter, clears the fight-armed flags and resets the Ancients; a statue spawns its Ancient only while
// that counter is 0 and fewer than three have spawned; closing the last portal re-arms the fight when it was armed.
//
// VERIFIED (0x58a7b0, 0x58a600, 0x58a4f0): only the third Ancient death ends the fight; the hero that struck it, and
// every hero on level 120 (and their party members anywhere in Act 5) at or above level 20*(difficulty+1) whose
// slot-39 bit 0 is clear, get bits 0 and 13 and the experience (RiteExperience).

// RiteAltarOpens is the hero-side rule of the altar.
func RiteAltarOpens(riteDone, ritePending bool) bool { return riteDone || ritePending }

// AncientsMayStart says whether a statue may spawn its Ancient: no town portal open in the arena and fewer than
// three Ancients already out.
func AncientsMayStart(portalsOpen, spawned int) bool { return portalsOpen == 0 && spawned < 3 }

// RiteAltarOpen is RiteAltarOpens for the hero of the game.
func (g *Game) RiteAltarOpen() bool {
	q := g.byID[QuestRite]
	if q == nil {
		return false
	}

	return RiteAltarOpens(g.get(q, FlagRewardGranted), g.get(q, FlagRewardPending))
}

// Search for Cain (0x58ffb0, 0x591010, 0x590eb0, 0x590e30). VERIFIED:
//   - Operating the gibbet (0x591010) spawns monster class 0x92 (146, Cain) next to it and gives every hero standing
//     in Tristram (level 0x26) whose slot-4 bits 0 and 1 are clear the goal bit (13) and reward pending (bit 1); a
//     party member of such a hero in any Act 1 level gets the same.
//   - Akara's message 0x76 (118) needs bit 1; it sets bit 0, clears bit 1 and creates the ring (item code "rin"):
//     item level 7 and magic quality in Normal, 30 and rare in Nightmare, 60 and rare in Hell (quality byte 4 / 6).
//     The quest log is then refreshed; when the goal bit 13 is set the node goes to state 6.

// CainRing is the item level and quality (0 normal, 1 magic, 2 rare) of Akara's ring by difficulty.
func CainRing(difficulty int) (ilvl, quality int) {
	switch difficulty {
	case Nightmare:
		return 30, 2
	case Hell:
		return 60, 2
	}

	return 7, 1
}

// Per-difficulty behaviour. VERIFIED by reading the handlers above: each reads the hero's quest record through the
// difficulty index of the game (player data + 0x10 + difficulty*4), so every difficulty repeats the quest from
// scratch; the reward values that depend on the difficulty are the Cain ring (CainRing), Anya's tier (AnyaTier), the
// Rite experience and level gate (RiteExperience, RiteLevelGate) and the Baal gold (EveGoldRange). The Malah potion
// counter and the scroll re-issue flag live in the game node, not the record: once per game.
