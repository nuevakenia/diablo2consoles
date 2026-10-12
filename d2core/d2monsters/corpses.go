package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// corpseTargetBase offsets the target ids of corpses a shaman can raise. It is
// above every other id space (players 1..n, mercs 1<<16, monsters 1<<17).
const corpseTargetBase = 1 << 20

// baseKeyOfClass resolves a monstats hcIdx to the lower-case BaseId of its
// family ("fallen1" for every Fallen), or "" when unknown.
func (d *Director) baseKeyOfClass(class int) string {
	for _, st := range d.asset.Records.Monster.Stats {
		if st.ID == class {
			return strings.ToLower(st.BaseKey)
		}
	}

	return ""
}

// NearestCorpse implements d2monster.CorpseFinder (the FUN_005f0420 scan of the
// Fallen Shaman: a dead monster of one of the classes' families, corpse
// present, not already being raised, within maxDist subtiles (squared
// comparison as in the exe)). Classes are matched by family because
// MONSTER_GetBaseClassId maps fallen2..8 to Fallen.
func (d *Director) NearestCorpse(b *d2monster.Brain, classes []int, maxDist int) (d2monster.Target, bool) {
	bases := map[string]bool{}

	for _, c := range classes {
		if k := d.baseKeyOfClass(c); k != "" {
			bases[k] = true
		}
	}

	var (
		best  d2monster.Target
		bestD = maxDist*maxDist + 1
		found bool
	)

	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || u.raising || u.m.Alive() {
			continue
		}

		if u.m.Mode() != d2monster.ModeDead && u.m.CorpseAge() < 1 {
			continue
		}

		if !bases[strings.ToLower(u.m.Stat.BaseKey)] {
			continue
		}

		x, y := u.m.SubtilePos()
		dx, dy := b.X-x, b.Y-y

		if dd := dx*dx + dy*dy; dd < bestD {
			bestD, found = dd, true
			best = d2monster.Target{ID: corpseTargetBase + u.b.ID, X: x, Y: y, Size: 1}
		}
	}

	return best, found
}

// NearestAlly implements d2monster.AllyFinder: the nearest other living
// hostile monster of the same class family (the exe filter is UNVERIFIED).
func (d *Director) NearestAlly(b *d2monster.Brain) (d2monster.Target, int, bool) {
	var (
		best  d2monster.Target
		bestD int
		found bool
	)

	me := d.unitOf(b)
	if me == nil {
		return best, 0, false
	}

	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || !u.m.Alive() ||
			!strings.EqualFold(u.m.Stat.BaseKey, me.m.Stat.BaseKey) {
			continue
		}

		x, y := u.m.SubtilePos()
		dist := d2monster.EdgeDistance(b.X-x, b.Y-y, b.Size)

		if !found || dist < bestD {
			bestD, found = dist, true
			best = d2monster.Target{ID: u.b.ID, X: x, Y: y, Size: 1}
		}
	}

	return best, bestD, found
}

// raiseCorpse lands a resurrection cast: the corpse stands up again with full
// hit points where it lay. skills.txt effects of Resurrect are not simulated
// beyond this.
func (d *Director) raiseCorpse(caster *unit, id uint32) {
	u := d.units[id]
	if u == nil {
		return
	}

	u.raising = false

	if u.m.Alive() {
		return
	}

	u.m.Revive()
	u.m.Vitals.HP = u.m.Vitals.MaxHP
	u.b.HPPercent = 100
	u.b.Wake = d.frame + 1
	x, y := u.m.SubtilePos()
	d.fp.Move(u.b.ID, x, y, d2path.FlagMonster)
	d.Counters.Raised++
	d.emit("raise", "MONSTER raise caster=%s id=%d corpse=%s corpse_id=%d", caster.m.Label(), caster.b.ID,
		u.m.Label(), u.b.ID)
}

// WispFormation implements d2monster.WispFormer: the living Gloam (base class
// 0x76, the only class the exe's filter 0x5f2920 accepts) within 0x20 subtiles
// of the searching wisp, itself included, in the director's stable unit order.
// VERIFIED w37 (MONAI_Think_WillOWisp 0x5f2970): the filter is 0x5f2920 (living, base class 0x76) and the search
// radius field holds 0x20; the think acts only when the search finds 4 or 5 wisps. The radius unit (subtile
// square here) and the result order of the exe's search stay UNVERIFIED.
func (d *Director) WispFormation(b *d2monster.Brain) []d2monster.WispMate {
	var out []d2monster.WispMate

	for _, u := range d.sortedUnits() {
		if u.merc != nil || u.ally != nil || !u.m.Alive() || !strings.EqualFold(u.m.Stat.BaseKey, "Gloam") {
			continue
		}

		x, y := u.m.SubtilePos()
		if dx, dy := b.X-x, b.Y-y; dx > 0x20 || dx < -0x20 || dy > 0x20 || dy < -0x20 {
			continue
		}

		ub := u.b
		out = append(out, d2monster.WispMate{B: ub, Wait: func(n int) { ub.Wake = d.frame + n }})
	}

	return out
}

// ShamanCorpseScan implements d2monster.ShamanCorpseScanner (w20, Fallen Shaman
// scan 0x5f04d0). See the interface for the two exe modes. Not modelled: the
// state masks 0x150 / 0xd4 and the unit flag bit 1 (unit flag word undecoded);
// a present corpse stands for them.
func (d *Director) ShamanCorpseScan(b *d2monster.Brain, radiusSq int) (d2monster.Target, int) {
	me := d.unitOf(b)
	if me == nil {
		return d2monster.Target{}, 0
	}

	const unique, champion = d2mapentity.MonTypeUnique, d2mapentity.MonTypeChampion

	family := me.m.TypeFlags&unique != 0 && me.m.TypeFlags&champion == 0

	var (
		last d2monster.Target
		n    int
	)

	for _, u := range d.sortedUnits() {
		if u.b == b || u.merc != nil || u.ally != nil || u.mirror || u.raising || u.m.Alive() {
			continue
		}

		if u.b.Mode != d2monster.ModeDead && u.m.CorpseAge() < 1 {
			continue
		}

		x, y := u.m.SubtilePos()
		dx, dy := b.X-x, b.Y-y

		if family {
			k := strings.ToLower(u.m.Stat.BaseKey)
			if k != d.baseKeyOfClass(19) && k != d.baseKeyOfClass(58) {
				continue
			}

			if u.m.TypeFlags&(unique|champion) != 0 || dx*dx+dy*dy > radiusSq {
				continue
			}
		} else if u.b.Leader != b || d2monster.EdgeDistance(dx, dy, b.Size) > radiusSq {
			continue
		}

		last, n = d2monster.Target{ID: corpseTargetBase + u.b.ID, X: x, Y: y, Size: 1}, n+1
	}

	return last, n
}
