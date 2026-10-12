package d2monsters

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"

// HPOf is the current life of a monster of the director (false when it is unknown), for scripted fights that must notice a
// target they cannot hurt (a spell that keeps ending on a wall).
func (d *Director) HPOf(m *d2mapentity.Monster) (int, bool) {
	if d == nil || m == nil {
		return 0, false
	}

	u := d.byEntity[m.ID()]
	if u == nil {
		return 0, false
	}

	return u.m.Vitals.HP, true
}
