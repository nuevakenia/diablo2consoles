package d2skills

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"

// sparesOwnUnit is the engine side of the friendly fire rule (wave 36): UNIT_AreUnitsHostile (0x552270) is false
// between a hero and its summons, golems, mercenary and converted monsters, so no damage, burn, poison or hostile
// state (curse, slow, stun, fear) from a hero skill reaches them. The target lists (Monsters, monstersNear, the
// missile world Targets / IsEnemy) already leave them out; this catches a route that names the unit directly
// (a pick, a chain, a reflected hit, a stale target of a unit that changed sides). It logs and counts the leak.
func (e *Engine) sparesOwnUnit(m *d2mapentity.Monster, what string) bool {
	if m == nil || e.monsters == nil || !e.monsters.IsAlly(m) {
		return false
	}

	e.Counters.FriendlySpared++
	e.emit("friendly", "FRIENDLY spared via=%q target=%s", what, m.Label())

	return true
}
