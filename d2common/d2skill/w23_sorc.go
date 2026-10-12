package d2skill

import "math"

// StaticFieldResistComp is the tail of the Static Field callback (0x5c7750,
// VERIFIED wave 23). After the damage is known (8.8 fixed point), the callback
// reads the target's resistance of the skill's damage type; when it is below
// zero the damage is divided by (100 - res) / 100 before the damage struct is
// built, so the later resist step (which multiplies by the same factor) gives
// back the intended amount. A negative resistance therefore never raises
// Static Field; zero and positive resistances are left to the normal resist
// step.
func StaticFieldResistComp(dmg8, res int) int {
	if res >= 0 {
		return dmg8
	}

	return dmg8 * 100 / (100 - res)
}

// Calc1 evaluates calc1 of a skill for a unit at a level (the exe evaluates it
// at the wearer's level when an armor event fires, e.g. the freeze length of
// Frozen Armor, handler 0x5c8e20).
func (p *Pipeline) Calc1(u Unit, skillID, lvl int) int {
	sk := p.Skills.ByID(skillID)
	if sk == nil {
		return 0
	}

	return p.env(sk, lvl, u).eval(sk.Calc[1])
}

// SrvMissileA is the srvmissilea column of a skill ("" when unknown).
func (p *Pipeline) SrvMissileA(skillID int) string {
	sk := p.Skills.ByID(skillID)
	if sk == nil {
		return ""
	}

	return sk.SrvMissileA
}

// BlazeTrailStep reports whether the player has moved to a new subtile since
// the last trail check: the stand-in for the path's reached-node flag that
// SKILL_SpawnTrailMissileWhileMoving (0x5c7eb0) waits for before it drops the
// Blaze fire (VERIFIED: nothing is dropped while the player stands still).
func BlazeTrailStep(lastX, lastY, x, y int) bool { return lastX != x || lastY != y }

// NovaRingOffset is the i-th destination offset of SKILL_ServerCreateNovaRing (0x56b170, VERIFIED wave 23): the
// two 64 entry tables at 0x6e2580 (x) and 0x6e2680 (y) hold a circle of radius 30 subtiles in steps of 5.625
// degrees, each coordinate truncated toward zero (entry 1 is (29, 2), entry 16 is (0, 30), entry 32 is (-30, 0)).
// Every missile of the ring starts on the caster and flies toward its offset (create kind 3: explicit start,
// destination relative to it).
func NovaRingOffset(i int) (dx, dy int) {
	a := 2 * math.Pi * float64(((i%novaCount)+novaCount)%novaCount) / novaCount

	return int(30 * math.Cos(a)), int(30 * math.Sin(a))
}
