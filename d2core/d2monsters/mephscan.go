package d2monsters

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"

// mephScanRadius2 is the squared distance bound of Mephisto's unit scan
// (callback 0x5f67a0 compares the squared distance with 0x400).
const mephScanRadius2 = 0x400

// MephScan implements d2monster.MephScanner: MONAI_FindBestUnitByModeScan
// 0x5f6850 with its callback 0x5f67a0 (wave 20 act 3-4 notes, completed in
// wave 22). The scan table entry 1 of 0x6e4890 has kind 0, so the walk is
// MONAI_ScanRoomUnitsWithCallbackAlt 0x5dbba0: EVERY unit of every room near
// Mephisto's room, whatever its type. The callback accepts a unit that is not
// Mephisto, not dead or in a death mode, carries unit flag 4 (targetable), is
// hostile to Mephisto (UNIT_AreUnitsHostile) and lies within squared distance
// 1024. That is the heroes, but also their mercenaries and every unit that
// fights on their side (summoned or converted monsters). The accepted unit
// with the lowest current life is the "weakest". A "nearest" slot is only
// filled when the scanning unit itself is a player (type 0), so for a monster
// it stays empty and the second roll of the exe never happens.
//
// UNVERIFIED: that the unit flag 4 is set on every unit the engine models
// (assumed), and the hostility of a pet or merc to a monster (assumed).
func (d *Director) MephScan(b *d2monster.Brain) (weakest, nearest *d2monster.Target) {
	best := int(^uint(0) >> 1)

	for _, id := range d.targetIDs() {
		p := d.targets[id]
		if p == nil || !d.targetable(p) {
			continue
		}

		x, y := playerSubtile(p)
		dx, dy := b.X-x, b.Y-y

		if dx*dx+dy*dy > mephScanRadius2 {
			continue
		}

		if life := p.Stats.Health; life < best {
			best = life
			weakest = &d2monster.Target{ID: id, X: x, Y: y, Size: 1, IsPlayer: true}
		}
	}

	// hostile mercenaries and the units on the heroes' side (the same units the monsters hunt in Nearest)
	for _, mu := range d.sortedUnits() {
		if mu.b == b || !mu.m.Alive() {
			continue
		}

		var id uint32

		switch {
		case mu.merc != nil && (d.opt.IgnoreTown || !mu.merc.owner.IsInTown()):
			id = mercTargetBase + mu.b.ID
		case mu.merc == nil && mu.b.Allied:
			id = unitTargetBase + mu.b.ID
		case mu.merc == nil && mu.ally != nil && mu.ally.opt.DrawsAggro:
			id = unitTargetBase + mu.b.ID
		default:
			continue
		}

		x, y := mu.m.SubtilePos()
		dx, dy := b.X-x, b.Y-y

		if dx*dx+dy*dy > mephScanRadius2 {
			continue
		}

		if life := mu.m.Vitals.HP; life < best {
			best = life
			weakest = &d2monster.Target{ID: id, X: x, Y: y, Size: 1}
		}
	}

	return weakest, nil
}
