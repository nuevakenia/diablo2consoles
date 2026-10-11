package d2monsters

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"

var (
	_ d2monster.StateSetter     = (*Director)(nil)
	_ d2monster.UnitStatAccess  = (*Director)(nil)
	_ d2monster.MummyScanner    = (*Director)(nil)
	_ d2monster.AuraStateSource = (*Director)(nil)
)

// SetUnitState implements d2monster.StateSetter (FUN_0063aef0): the AI-driven
// unit states (inferno 0xc after the Fetish Shaman's and Megademon's Skill1,
// the Raider and sentry flags). They have no stat list or timer here; the AI
// that turns one on turns it off.
func (d *Director) SetUnitState(b *d2monster.Brain, state int, on bool) {
	u := d.unitOf(b)
	if u == nil {
		return
	}

	if !on {
		delete(u.states, state)

		return
	}

	if u.states == nil {
		u.states = map[int]bool{}
	}

	u.states[state] = true
}

// unitHasState answers the AI-set states of a unit.
func (d *Director) unitHasState(b *d2monster.Brain, state int) bool {
	u := d.unitOf(b)

	return u != nil && u.states[state]
}

// UnitStat implements d2monster.UnitStatAccess: a stat the AI scales on the
// monster itself (the Bat Demon's flight boost of stat 0x4a, hpregen). The
// engine has no per-monster regeneration stat yet, so the value is whatever
// the AI stored: 0, which makes the boost a no-op until the stat has a source.
func (d *Director) UnitStat(b *d2monster.Brain, stat int) int {
	if u := d.unitOf(b); u != nil {
		return u.stats[stat]
	}

	return 0
}

// SetUnitStat implements d2monster.UnitStatAccess.
func (d *Director) SetUnitStat(b *d2monster.Brain, stat, v int) {
	u := d.unitOf(b)
	if u == nil {
		return
	}

	if u.stats == nil {
		u.stats = map[int]int{}
	}

	u.stats[stat] = v
}
