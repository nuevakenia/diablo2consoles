package d2missile

import "math"

// SrvDoFunc 31: the wake maker of Wake of Fire / Wake of Inferno (missile
// "wake of destruction maker"), read from MISSILE_SrvDoFunc31 at 0x5addd0
// (Game.exe 1.14b). Wave 20 assassin port.
//
// VERIFIED: a maker whose table row has no SubMissile1, or whose owner unit is
// gone, ends (the exe returns 2 after the usual always-explode handling). While
// it flies, each step that entered a new subtile (path flag 8) creates the
// SubMissile1 TWICE at the maker's position: once aimed along the vector held
// in the maker's data fields (0x28 = x part, 0x2c = y part) and once along the
// negated vector, i.e. one flame on each side of the wake line; then the
// standard move runs. The vector is put into the fields by SRVDO_125
// (SRVDO_125_WakeOfDestructionSentry 0x5cf700): direction = target - sentry,
// field 0x28 = -dy, field 0x2c = dx, a quarter turn of the line of fire.
// UNVERIFIED: the sub missiles are aimed at (position + vector), the vector
// used as an offset in subtiles.

// WakeVector is what SRVDO_125 stores in the maker's data fields for a sentry
// at (fromX, fromY) firing at (toX, toY): the quarter turn of the line of fire.
func WakeVector(fromX, fromY, toX, toY int) (d28, d2C uint32) {
	dx, dy := toX-fromX, toY-fromY

	return uint32(int32(-dy)), uint32(int32(dx))
}

func (s *Sim) wakeSpread(m *Missile) bool {
	if m.Spec.SubMissile[0] == "" || m.Owner.ID == "" || m.ownerGone() {
		s.vanish(m)

		return false
	}

	sub := s.lookup(m.Spec.SubMissile[0])
	if sub == nil || !m.entered {
		return true
	}

	vx, vy := float64(int32(m.Data28)), float64(int32(m.Data2C))
	x, y := math.Floor(m.X)+0.5, math.Floor(m.Y)+0.5

	for _, sign := range []float64{1, -1} {
		_, _ = s.Create(CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
			Damage: m.Damage, AreaRadius: m.AreaRadius, X: x, Y: y, DestX: x + sign*vx, DestY: y + sign*vy})
	}

	return true
}
