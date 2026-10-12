package d2skill

import (
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2animspeed"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
)

// Whirlwind movement (SRVST_038 0x5d7a00, V waves 21 and 22). The start function refuses to run while the caster
// has state 12 (WhirlwindRefusedState, the "inferno" row of states.txt), computes a path with path type 7, and sets
// the path velocity to UNIT_GetBaseMoveSpeedFixed (0x56c370): the class WalkVelocity of charstats shifted left by 8
// (6 << 8 for all seven classes), WITHOUT the movement percentage that normal walking applies. In the path unit
// of the exe (d2path.SubtilesPerTick: 8.8 velocity V<<8 covers V/16 subtile per 25 Hz tick) that is 0.375 subtile
// per frame, not the one subtile per frame the old model used. UNVERIFIED: PATH_UpdateUnitVelocityAndAnimRate may
// rescale the velocity later when the stats change (the aura state is added right after the path is set).

// WhirlwindRefusedState is the state id that makes SRVST_038 return 0 (the skill is not started).
const WhirlwindRefusedState = 12

// WhirlTickDistance is how far a whirling hero travels along its path per frame, in subtiles.
var WhirlTickDistance = d2path.SubtilesPerTick(d2animspeed.PlayerWalkVelocity)

// WhirlStepCost is the distance of one cell step of the path: 1 along an axis, the square root of two on a
// diagonal.
func WhirlStepCost(dx, dy int) float64 {
	if dx != 0 && dy != 0 {
		return math.Sqrt2
	}

	return 1
}
