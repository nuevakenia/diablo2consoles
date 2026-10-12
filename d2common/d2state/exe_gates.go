package d2state

// Target gates of the hit state appliers, ported in wave 23 from Game.exe 1.14b (COMBAT_ApplyStunState 0x578830,
// COMBAT_ApplyColdSlowState 0x578ca0, COMBAT_ApplyFreezeOrColdSlowState 0x578f50). Notes: d2-re-notes/w23-states.md.

// Target describes the unit a hit lands on, as far as the appliers care.
type Target struct {
	// Monster is unit type 1; false is a player (type 0) or anything else.
	Monster bool
	// Boss is the monstats "boss" bit (MONSTER_IsBossMonster 0x63fa40: dwBitsc & 0x40). Verified.
	Boss bool
	// Merc is MONSTER_GetHirelingClassIndex (0x63fed0) != 0: the monster is a mercenary. Verified.
	Merc bool
	// DataFlag8 is bit 8 of the monster unit data flags (+0x16, MONSTER_HasDataFlag16(unit, 8)). The meaning
	// of the bit is UNVERIFIED (the same word has bit 2 on the kill announcement path, bit 4 on champion
	// minions); the effect is verified.
	DataFlag8 bool
	// ZeroSpeed is monstats Velocity == 0 (stun skips immobile monsters). Verified.
	ZeroSpeed bool
	// Uninterruptable is state 0x36 on the unit (freeze skips it). Verified.
	Uninterruptable bool
}

// StunSkipPct is the chance in percent that the exe lets a stun through on a DataFlag8 monster: the roll
// seed%100 < 90 returns, so only 10 percent of the stuns land. (Wave 20 notes called it a 10 percent skip;
// the comparison reads the other way round.) Verified at 0x578830.
const StunSkipPct = 90

// MercStunFrames is the length a stun is cut to for a mercenary when the stun is longer than 12 frames.
const MercStunFrames = 13

// StunFor returns the stun length the exe applies to the target and whether a stun is applied at all.
// roll100 is the seed%100 roll the exe draws only for a DataFlag8 monster (pass any value otherwise). A
// monster that is not mercenary and not a boss takes the plain 250 cap; a mercenary over 12 frames takes 13
// (instead of the cap); players take the cap. Verified at 0x578830.
func StunFor(frames int, t Target, roll100 int) (int, bool) {
	if frames < 1 {
		return 0, false
	}

	if t.Monster {
		if t.DataFlag8 && roll100 < StunSkipPct {
			return 0, false
		}

		if t.Boss || t.ZeroSpeed {
			return 0, false
		}

		if t.Merc && frames > 12 {
			return MercStunFrames, true
		}
	}

	return StunLength(frames), true
}

// FreezeRoute says what a freeze length becomes on a target.
type FreezeRoute int

const (
	// RouteFreeze is the real freeze state (state 1).
	RouteFreeze FreezeRoute = iota
	// RouteChill hands the length to the chill applier (state 0xb).
	RouteChill
	// RouteNone applies nothing.
	RouteNone
)

// RouteFreezeFor is the head of 0x578f50, verified: a player gets chill; a monster with state 0x36 gets
// nothing; a boss, DataFlag8 monster or mercenary gets chill; anything else is frozen (when its ColdEffect is
// negative, checked later).
func RouteFreezeFor(t Target) FreezeRoute {
	if !t.Monster {
		return RouteChill // unit type 0 (player): chill, never freeze
	}

	if t.Uninterruptable {
		return RouteNone
	}

	if t.Boss || t.DataFlag8 || t.Merc {
		return RouteChill
	}

	return RouteFreeze
}

// ShatterVisualPct is the percent chance that a chill application on a monster turns the shatter visual (state
// 0x6b, states.txt shatter) on; the other 80 percent turn it off. The monster class must not carry
// monstats2 flag bit 0x13. Verified at 0x578ca0 (the roll runs on every chill call with a positive length).
const ShatterVisualPct = 20

// ShatterVisual reports whether the chill call leaves state 0x6b on. noFlash is MONSTATS2_TestFlagBit(class, 0x13).
func ShatterVisual(roll100 int, monster, noFlash bool) bool {
	return monster && !noFlash && roll100 < ShatterVisualPct
}
