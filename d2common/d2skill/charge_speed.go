package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"

// Charge movement (SRVST_031 0x5cdbb0, V wave 22; the engine still moves the paladin to the target in one step,
// so this is the number the timed rush needs, not yet wired). The start function walks the path with the velocity
//
//	base * (Param1 + max(stat 0x43, 50)) / 100
//
// where base is the class velocity byte of charstats at record +0x41 (shifted left by 8; the run velocity column,
// 9 in the shipped rows, UNVERIFIED which column sits at +0x41), Param1 the skill's "percent increase in velocity"
// (150) and stat 0x43 the unit's velocity percentage, floored at 50. The rush then repeats until the target is in
// melee range (SRVDO_067) with a path step counter of 20 for monsters.

// ChargeVelocity is the 8.8 path velocity of a Charge for the base velocity column, the skill's Param1 and the
// unit's velocity percentage.
func ChargeVelocity(baseVel, param1, velocityPct int) int {
	if velocityPct < 50 {
		velocityPct = 50
	}

	return (baseVel << 8) * (param1 + velocityPct) / 100
}

// ChargeSubtilesPerTick is how far that velocity carries the unit per frame, in subtiles.
func ChargeSubtilesPerTick(vel int) float64 { return d2path.SubtilesPerTick(vel) }
