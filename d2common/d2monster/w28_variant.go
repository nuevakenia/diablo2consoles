package d2monster

// UniqueVariantRoll is MONREGION_RollGroupMemberVariant 0x5bbba0 (VERIFIED w28): whether the next group of a
// level is a unique / champion spawn (the exe's variant 0; every other draw gives the ordinary group,
// variant 1 is remapped to 2 by the caller 0x54cad0).
//
//   - spawned is the count of uniques of the level so far (+0x2c8), uMin / uMax the levels.txt MonUMin / MonUMax
//     of the difficulty (+0x2bc / +0x2bd);
//   - below uMin and with a known room total, Roll(100) < roomsSeen*100/rooms forces one (the ramp that
//     makes a level reach its minimum as the rooms are visited);
//   - below uMax, Roll(100) < 6 makes one;
//   - the last draw (> 35, ordinary group) is consumed whatever the result.
//
// roll(n) returns 0..n-1. rooms <= 0 skips the ramp (the host does not know the room total: UNVERIFIED what
// the word at region +0xc counts, it comes from DRLG_FindOrAllocLevel).
func UniqueVariantRoll(roll func(n int) int, spawned, uMin, uMax, roomsSeen, rooms int) bool {
	if spawned < uMin && rooms > 0 {
		if roll(100) < roomsSeen*100/rooms {
			return true
		}
	}

	if spawned < uMax {
		if roll(100) < 6 {
			return true
		}
	}

	roll(100)

	return false
}
