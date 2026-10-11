package d2monsters

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"

// MummyScan implements d2monster.MummyScanner, the neighbour scan of the Greater
// Mummy (callback 0x5f1a00, read in batch 8). A neighbour counts when it is a
// monster other than the mummy, on the same side (the exe's alignment test:
// converted monsters and the hero's summons are out), and a low-undead monster
// (monstats flag word bit 0x800; Radament's variant takes high undead too, bit
// 0x1000). Inside radius squared it is counted; a dead one (mode DD, not
// already being raised) becomes the corpse, the last one visited wins; a living
// one below its maximum life that is not dying marks "wounded". The exe also
// skips monsters whose unit-data word has bit 8 set unless this is Radament in
// Normal; what that bit is, is not decoded and it is not modelled.
func (d *Director) MummyScan(b *d2monster.Brain, radius int, radament bool) d2monster.MummyScan {
	var out d2monster.MummyScan

	r2 := radius * radius

	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || u.mirror {
			continue
		}

		low, high := u.m.Stat.IsUndeadLow, u.m.Stat.IsUndeadHigh
		if !low && !(radament && high) {
			continue
		}

		x, y := u.m.SubtilePos()
		dx, dy := b.X-x, b.Y-y

		if dx*dx+dy*dy > r2 {
			continue
		}

		out.InReach++

		mode := u.m.Mode()

		switch {
		case !u.m.Alive() && (mode == d2monster.ModeDead || u.m.CorpseAge() >= 1):
			if !u.raising {
				out.Corpse = &d2monster.Target{ID: corpseTargetBase + u.b.ID, X: x, Y: y, Size: 1}
			}
		case u.b.Mode != d2monster.ModeDying && u.m.Alive() && u.m.Vitals.HP != u.m.Vitals.MaxHP:
			out.Wounded = true
		}
	}

	return out
}
