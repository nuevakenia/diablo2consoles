package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

var _ d2monster.WispBlesser = (*Director)(nil)

// timedStat is one stat a will-o-wisp blessing put on a hero: its value and
// the Director frame it stops counting.
type timedStat struct {
	value, until int
}

// wispStatNames maps the stat ids the monster AI hands out to the
// ItemStatCost names the hero stat merge uses. The wisp dance gives stat 0x50
// (item_magicbonus).
var wispStatNames = map[int]string{0x50: "item_magicbonus"}

// WispBless implements d2monster.WispBlesser: the finished dance puts a timed
// stat list on the player (stat 0x50, (difficulty+1)*50, 0x1a5e00 frames). The
// hero's SkillStats merge reads it through TimedHeroStats, so RecalcStats adds
// it to the totals. A second blessing of the same stat replaces the first
// (UNVERIFIED: the exe may stack a separate list).
func (d *Director) WispBless(_ *d2monster.Brain, t d2monster.Target, stat, value, frames int) {
	p := d.playerFor(t.ID)
	if p == nil || p.Stats == nil || wispStatNames[stat] == "" {
		return
	}

	if d.blessings == nil {
		d.blessings = map[*d2mapentity.Player]map[int]timedStat{}
	}

	if d.blessings[p] == nil {
		d.blessings[p] = map[int]timedStat{}
	}

	d.blessings[p][stat] = timedStat{value: value, until: d.frame + frames}

	if p.Stats.Recalc != nil {
		p.Stats.Recalc()
	}
}

// TimedHeroStats returns the unexpired wisp stats of a hero, keyed by
// ItemStatCost name, for the hero's SkillStats merge.
func (d *Director) TimedHeroStats(p *d2mapentity.Player) map[string]int {
	var out map[string]int

	for stat, ts := range d.blessings[p] {
		if ts.until <= d.frame {
			continue
		}

		if out == nil {
			out = map[string]int{}
		}

		out[wispStatNames[stat]] += ts.value
	}

	for k, v := range d.cursedStats(p) { // Cursed monsters (umodhooks.go)
		if out == nil {
			out = map[string]int{}
		}

		out[k] += v
	}

	return out
}
