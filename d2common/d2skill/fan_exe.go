package d2skill

// Fan geometry of SRVDO_008_MultipleShot (0x5d9fe0, V wave 22): Multiple Shot, Teeth and Shock Wave. The exe does
// not turn the aim by an angle. It builds the vector v from the caster to the aim point, scales it up when it
// is short (SKILL_ScaleSmallDirectionVector 0x56b0e0), turns it a quarter and shrinks it back
// (SKILL_GetShrunkPerpendicularVector 0x56b120) into a spacing s whose components are -1, 0 or 1, moves the aim
// point back by s * n / 2 (C division) and aims missile i at that start + i * s: a row of n destinations through
// the aim point, perpendicular to the line of fire, one subtile (two on a diagonal) apart.

// FanScale is SKILL_ScaleSmallDirectionVector: a vector with squared length below 4 is multiplied by 4, and one
// that is then still below 16 (squared) by 2, so that the next step starts from a squared length of at least 16
// (a zero vector stays zero).
func FanScale(vx, vy int) (int, int) {
	if vx*vx+vy*vy < 4 {
		vx, vy = vx*4, vy*4
	}

	if vx*vx+vy*vy < 16 {
		vx, vy = vx*2, vy*2
	}

	return vx, vy
}

// FanPerpendicular is SKILL_GetShrunkPerpendicularVector: the vector (y, -x) halved (truncating towards zero)
// until its squared length is 3 or less.
func FanPerpendicular(vx, vy int) (int, int) {
	sx, sy := vy, -vx

	for sx*sx+sy*sy > 3 {
		sx, sy = sx/2, sy/2
	}

	return sx, sy
}

// FanSpacing is the spacing vector of a fan for the caster-to-aim vector (vx, vy).
func FanSpacing(vx, vy int) (int, int) {
	return FanPerpendicular(FanScale(vx, vy))
}

// FanDestinations lists the n destinations of a fan around the aim point (tx, ty) for the spacing (sx, sy).
func FanDestinations(tx, ty, n, sx, sy int) [][2]int {
	if n < 1 {
		return nil
	}

	x, y := tx-sx*n/2, ty-sy*n/2
	out := make([][2]int, n)

	for i := range out {
		out[i] = [2]int{x, y}
		x, y = x+sx, y+sy
	}

	return out
}
