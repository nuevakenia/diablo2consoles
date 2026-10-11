package d2monsters

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"

// mephScanRadius2 is the squared distance bound of Mephisto's unit scan
// (callback 0x5f67a0 compares the squared distance with 0x400).
const mephScanRadius2 = 0x400

// MephScan implements d2monster.MephScanner: MONAI_FindBestUnitByModeScan
// 0x5f6850 with its callback 0x5f67a0 (wave 20 act 3-4 notes). The callback
// accepts a unit that is not Mephisto, not dead or in a death mode, carries
// unit flag 4 (targetable), is hostile to Mephisto (UNIT_AreUnitsHostile) and
// lies within squared distance 1024. The accepted unit with the lowest
// current life is the "weakest". A "nearest" slot is only filled when the
// scanning unit itself is a player (type 0), so for a monster it stays empty
// and the second roll of the exe never happens.
//
// UNVERIFIED: the unit types the walk of list index 1 visits (read here as
// the heroes; hostile mercenaries are not scanned).
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

	return weakest, nil
}
