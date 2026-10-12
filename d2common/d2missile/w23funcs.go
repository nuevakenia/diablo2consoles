package d2missile

import "math"

// Wave 23: movement functions of monster missiles. Read in Game.exe 1.14b
// (V = read in the decompiled body or disassembly).

// monsterBlizzard is SrvDoFunc 8 (0x5ac3e0, monblizcenter; V). Like Blizzard
// (10) it scatters SubMissile1 with the 0x5a7280 helper and the cell mask 5,
// but the numbers come from the missile record, not skill calcs: with
// L = skill level and d = max(Param3, 1), the scatter radius is
// Param1 + max(2, L/d) and the frames between two sub missiles are
// max(3, Param2 - L/d).
func (s *Sim) monsterBlizzard(m *Missile) {
	sp := m.Spec

	d := sp.Param[2]
	if d < 1 {
		d = 1
	}

	q := m.Level / d
	radius := 2
	if q >= 2 {
		radius = q
	}

	every := sp.Param[1] - q
	if every < 3 {
		every = 3
	}

	s.spawnRandomSub(m, spawnMaskBlizzard, every, sp.Param[0]+radius)
}

// subAtOwnPos is SrvDoFunc 9 (0x5ac480, bat lightning bolt; V, without a
// destination) and 12 (0x5ac810, diabwallmaker; V, with the destination set
// to the missile's own cell, create kind 0x21). In a frame in which the last
// step entered a new subtile (path flag 8) the missile creates SubMissile1 on
// its own subtile with its owner, skill and level, then moves on. Function 12
// is destroyed at once (return 2) when it has no SubMissile1 or no owner;
// function 9 does neither check (a missing sub id creates nothing).
func (s *Sim) subAtOwnPos(m *Missile, strict bool) bool {
	name := m.Spec.SubMissile[0]
	if strict && (name == "" || m.Owner.ID == "" || m.ownerGone()) {
		s.vanish(m)

		return false
	}

	sub := s.lookup(name)
	if sub == nil || !m.entered {
		return true
	}

	x, y := math.Floor(m.X)+0.5, math.Floor(m.Y)+0.5
	_, _ = s.Create(CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
		Damage: m.Damage, AreaRadius: m.AreaRadius, X: x, Y: y, DestX: x, DestY: y})

	return true
}

// homingNudge is SrvDoFunc 11 (0x5ac6b0, fingermagespider; V). With a target
// (the path's target unit; the exe falls back to the owner's target, which the
// sim does not track: UNVERIFIED stand-in is Home only) every P1 frames of
// remaining life (Param1, 5 when not positive) and within Param2 of the
// target, the destination becomes the missile's cell moved by max(Param3, 1)
// subtiles on each axis toward the target (0 on an axis where they are equal),
// and the path is recomputed. Without a target it is a plain move.
func (s *Sim) homingNudge(m *Missile) {
	pt, ok := m.Home.(Positioned)
	if !ok || m.Home == nil || !m.Home.Alive() {
		return
	}

	period := m.Spec.Param[0]
	if period < 1 {
		period = 5
	}

	if m.Life%period != 0 {
		return
	}

	tx, ty := pt.SubPos()
	if math.Hypot(tx-m.X, ty-m.Y) > float64(m.Spec.Param[1]) {
		return
	}

	step := m.Spec.Param[2]
	if step < 1 {
		step = 1
	}

	nx, ny := math.Floor(m.X), math.Floor(m.Y)
	if int(tx) > int(m.X) {
		nx += float64(step)
	} else if int(tx) < int(m.X) {
		nx -= float64(step)
	}

	if int(ty) > int(m.Y) {
		ny += float64(step)
	} else if int(ty) < int(m.Y) {
		ny -= float64(step)
	}

	aim(m, nx+0.5, ny+0.5)
}

// castAtPos is SrvDoFunc 21 (0x5ad0e0, "distraction"; V): a missile without a
// record is destroyed. When SubMissile1 is set and the last step entered a new
// subtile it creates SubMissile1 on its subtile for the owner (skill create
// helper, same skill and level), and then it behaves like function 20 (stays
// on the owner, see followOwner).
func (s *Sim) castAtPos(m *Missile) bool {
	if sub := s.lookup(m.Spec.SubMissile[0]); sub != nil && m.entered {
		x, y := math.Floor(m.X)+0.5, math.Floor(m.Y)+0.5
		_, _ = s.Create(CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
			Damage: m.Damage, AreaRadius: m.AreaRadius, X: x, Y: y, DestX: x, DestY: y})
	}

	return s.followOwner(m)
}

// reflectPair is SrvDoFunc 22 (0x5ad170, the lightning trailing javelins; V).
// It needs SubMissile1 and a path. During its first frames (elapsed life < 2)
// it stores the perpendicular of its flight vector in data fields 0x28 / 0x2c:
// (-(destY - y), destX - x). Whenever the last step entered a new subtile it
// creates SubMissile1 twice on its subtile (create kind 0xb: explicit source,
// destination = source + offset, sub-loop count = Param1): once aimed along
// the perpendicular and once along its negation. So the javelin leaves
// lightning bolts to both sides of its track.
func (s *Sim) reflectPair(m *Missile) bool {
	sp := m.Spec

	sub := s.lookup(sp.SubMissile[0])
	if sub == nil {
		s.vanish(m)

		return false
	}

	cx, cy := math.Floor(m.X), math.Floor(m.Y)

	if m.Elapsed() < 2 {
		dx, dy := math.Floor(m.destX), math.Floor(m.destY)
		m.Data28 = uint32(int32(-(dy - cy)))
		m.Data2C = uint32(int32(dx - cx))
	}

	if m.entered {
		ox, oy := float64(int32(m.Data28)), float64(int32(m.Data2C))
		x, y := cx+0.5, cy+0.5

		for _, sign := range []float64{1, -1} {
			_, _ = s.Create(CreateParams{Spec: sub, Parent: m, Owner: m.Owner, SkillID: m.SkillID, Level: m.Level,
				Damage: m.Damage, AreaRadius: m.AreaRadius, X: x, Y: y, DestX: x + sign*ox, DestY: y + sign*oy,
				SubLoops: sp.Param[0]})
		}
	}

	return true
}
