package d2skills

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// ironMaidenBack is the Iron Maiden event (skills.txt auraevent1 domeleedamage, auraeventfunc 4 =
// SKILL_ApplyStateLinkedDamageToTarget 0x5c3ec0, VERIFIED wave 23): when a unit that carries the curse deals
// melee damage (> 0) to a player or monster, the CURSED unit takes pct percent of the damage it dealt (calc1 for
// a monster, ln56 = 200 + 25 per level above the first in the shipped table), as physical damage through the
// normal damage path. The old port reflected hero hits on a cursed monster back at the hero: wrong direction.
// UNVERIFIED: whether the cursed monster's physical resistance applies to the returned damage (the exe goes
// through COMBAT_ApplyDamageToUnit with flag 0x20 of the result flags, meaning not read).
// It returns the damage dealt back (0 when the curse is not on the attacker).
func (e *Engine) ironMaidenBack(attacker *d2mapentity.Monster, p *d2mapentity.Player, melee bool, dmg int) int {
	if attacker == nil || !melee || dmg <= 0 || !attacker.Alive() {
		return 0
	}

	back := d2skill.IronMaidenDamage(dmg, e.setOf(attacker.ID()).ReflectPct(e.frame))
	if back <= 0 {
		return 0
	}

	e.monsters.Damage(attacker, back, p)
	e.emit("damage", "DAMAGE iron_maiden cursed=%s dmg=%d hp=%d/%d", attacker.Label(), back, maxInt(attacker.Vitals.HP, 0),
		attacker.Vitals.MaxHP)

	return back
}

func ironMaidenNote(back int) string {
	if back <= 0 {
		return ""
	}

	return fmt.Sprintf(" iron_maiden=%d", back)
}
