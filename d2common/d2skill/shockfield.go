package d2skill

// Shock Web / Shock Field (w23). SRVDO_043_ShockField 0x5d46f0 and SKILL_SpawnShockFieldMissiles 0x5d4540,
// VERIFIED:
//
//   - The count is SRVDO_EvalSkillCalcOrStateValue (prgcalc1 for this skill, par1+lvl/par2+Fire Blast/3) and the
//     radius is aurarangecalc (count/4+1). A count below 1 makes the cast fail.
//   - With a count or radius below 2 one missile ("shock field in air", a thrown caltrop, speed 12) flies to the
//     target position.
//   - Otherwise the caster's seed is re-initialised from the target x (DRLG_InitSeed: low = x, high = 0x29a) and
//     count missiles are thrown, each to a random point of the square [tx-R, tx+R) x [ty-R, ty+R) (two
//     RAND_RollSeedModulo(2R) rolls, x then y). A throw is only made when the squared distance between the
//     caster and the point is above 3. The missile explodes into the "shock field on ground" effect.

// shockSeed is the unit seed pair after DRLG_InitSeed.
type shockSeed struct{ lo, hi uint32 }

func newShockSeed(v int) shockSeed { return shockSeed{lo: uint32(v), hi: 0x29a} }

// modulo is RAND_RollSeedModulo 0x457b60: one step lo*0x6ac690c5+hi, the result is the low word modulo n
// (a mask for powers of two, the same value).
func (s *shockSeed) modulo(n int) int {
	if n < 1 {
		return 0
	}

	v := uint64(s.lo)*0x6ac690c5 + uint64(s.hi)
	s.lo, s.hi = uint32(v), uint32(v>>32)

	return int(uint32(v) % uint32(n))
}

// ShockFieldPoints returns the throw destinations of a Shock Web cast: the target alone when count or radius is
// below 2, else up to count random points in the square, those not farther than 3 (squared) from the caster
// left out.
func ShockFieldPoints(tx, ty, count, radius, casterX, casterY int) [][2]int {
	if count < 1 {
		return nil
	}

	if count < 2 || radius < 2 {
		return [][2]int{{tx, ty}}
	}

	s := newShockSeed(tx)

	var out [][2]int

	for i := 0; i < count; i++ {
		x := s.modulo(radius*2) + tx - radius
		y := s.modulo(radius*2) + ty - radius

		dx, dy := casterX-x, casterY-y
		if dx*dx+dy*dy > 3 {
			out = append(out, [2]int{x, y})
		}
	}

	return out
}
