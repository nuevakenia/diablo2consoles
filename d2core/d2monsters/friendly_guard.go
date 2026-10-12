package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Friendly fire guard (wave 36). The exe never lets a hero's attack, missile or area scan reach one of the hero's own
// units: the missile hit loop (common predicate 0x5a6160) and the scan filter 0x569100 call UNIT_AreUnitsHostile
// (0x552270), which answers "not hostile" for a unit and its owner's followers. The skills engine already lists only
// Monsters() (hostile) as area targets; this guard is the single place that backs every other route (a direct
// Damage / DamageOverTime / HeroStrike, a click or shift-click attack, a status) with the same relation.

// relationUnit builds the d2monster.Unit view of a director unit for the relation test.
func (d *Director) relationUnit(u *unit) *d2monster.Unit {
	r := &d2monster.Unit{ID: u.m.ID()}

	switch {
	case u.ally != nil && u.ally.owner != nil:
		r.Owner = &d2monster.Unit{ID: u.ally.owner.ID(), IsPlayer: true}
	case u.merc != nil:
		r.HeroSide = true // a mercenary follows the hero it was hired by
	case u.ally != nil:
		r.HeroSide = true // an ownerless summon still fights for the heroes
	}

	r.Converted = u.b != nil && u.b.Allied

	return r
}

// HostileToHeroes is UNIT_AreUnitsHostile(hero, m) for any hero: false for the hero's summons, golems, mercenary and
// converted monsters, true for every other monster. A monster the director does not know is treated as hostile.
func (d *Director) HostileToHeroes(m *d2mapentity.Monster) bool {
	u := d.byEntity[m.ID()]
	if u == nil {
		return true
	}

	hero := &d2monster.Unit{ID: "hero", IsPlayer: true}

	return d2monster.AreHostile(hero, d.relationUnit(u), nil)
}

// refuseFriendly reports (and counts) an attempt of a hero to hurt one of its own units.
func (d *Director) refuseFriendly(m *d2mapentity.Monster, what string) bool {
	if d.HostileToHeroes(m) {
		return false
	}

	d.Counters.FriendlyRefused++
	d.emit("friendly", "FRIENDLY refused via=%s target=%s id=%s", what, m.Label(), m.ID())

	return true
}
