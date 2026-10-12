package d2monsters

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"

// Wave 29 hosts of the Rite of Passage fight driven by the game screen (rite_live.go): the portal counter, the reset
// of the Ancients (VERIFIED 0x58ace0: a town portal in level 120 clears the armed flags) and the removal of a monster.

// RiteSetPortals sets the portal counter (+0xc of the quest node) the Ancients and the statues read.
func (d *Director) RiteSetPortals(n int) {
	if d.rite == nil {
		d.rite = &RiteState{}
	}

	d.rite.Portals = n
}

// RiteReset clears the "fight started" byte and the statue modes: the Ancients wait again (VERIFIED 0x58ace0).
func (d *Director) RiteReset() {
	if d.rite == nil {
		d.rite = &RiteState{}
	}

	d.rite.Started = false
	d.rite.Statues = [3]int{}
}

// RiteStarted says whether the fight has been started and not reset.
func (d *Director) RiteStarted() bool { return d.rite != nil && d.rite.Started }

// DismissMonster takes a monster off the map without a death (the reset of the Ancients). False when it is unknown.
func (d *Director) DismissMonster(m *d2mapentity.Monster) bool {
	u := d.byEntity[m.ID()]
	if u == nil {
		return false
	}

	d.emit("boss", "MONSTER dismiss name=%s id=%d", u.m.Label(), u.b.ID)
	d.engine.RemoveEntity(u.m)
	d.forget(u)

	return true
}
