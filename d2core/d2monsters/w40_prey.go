package d2monsters

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"

// NearestPrey implements d2monster.PreyFinder: MONAI_Vulture_WoundedAllyFilter 0x5f20d0 (w40, VERIFIED filter).
// A candidate is a living player or monster other than the caller, within the squared radius (the radius shrinks
// to the best distance found, so the nearest wins), whose life is at most (max/100)*pct (integer division first).
// The exe does not test hostility. Monsters carry only a life percent here, so their test is HPPercent <= pct
// (UNVERIFIED rounding difference).
func (d *Director) NearestPrey(b *d2monster.Brain, radius, pct int) (d2monster.Target, bool) {
	best := radius * radius

	var (
		res   d2monster.Target
		found bool
	)

	for _, id := range d.targetIDs() {
		p := d.targets[id]
		if p == nil || p.Stats == nil || p.Stats.Health <= 0 {
			continue
		}

		x, y := playerSubtile(p)

		dx, dy := x-b.X, y-b.Y
		if dd := dx*dx + dy*dy; dd <= best && p.Stats.Health <= (p.Stats.MaxHealth/100)*pct {
			best, found = dd, true
			res = d2monster.Target{ID: id, X: x, Y: y, Size: 1, IsPlayer: true}
		}
	}

	for _, u := range d.sortedUnits() {
		if u.b == b || !u.m.Alive() {
			continue
		}

		dx, dy := u.b.X-b.X, u.b.Y-b.Y
		if dd := dx*dx + dy*dy; dd <= best && u.b.HPPercent <= pct {
			best, found = dd, true
			res = targetOf(u)
		}
	}

	return res, found
}
