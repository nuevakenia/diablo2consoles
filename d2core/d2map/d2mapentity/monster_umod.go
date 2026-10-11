package d2mapentity

// UModElem is an elemental damage a modifier adds to every hit of the monster (stats 0x30..0x3b of the exe's stat
// list). Type is "fire", "ltng", "cold" or "pois"; for poison Min and Max are already the total over the length.
type UModElem struct {
	Type     string
	Min, Max int
}

// UModSkill is one skill a modifier granted.
type UModSkill struct {
	Skill, Level int
	Active       bool
}

// UModBonus is what the monster modifiers (monumod.txt) did to the unit beyond its life, level and experience:
// resists, extra elemental damage, mana drain and speed (d2monreg.FxCtx, VERIFIED init callbacks of the exe).
type UModBonus struct {
	// Resist is added to the monstats resists, in the order physical, magic, fire, lightning, cold, poison.
	Resist [6]int
	// Elems are added to each hit that lands.
	Elems []UModElem
	// DrainMin and DrainMax are the mana a hit takes from the hero (whole points; Mana Burn).
	DrainMin, DrainMax int
	// SpeedPct is the velocity percent (stat 0x43) of Fast / Fanatic / the champion kit.
	SpeedPct int
	// Skills are the skills granted by Teleportation (184) and Aura Enchanted; AIFlag20 is the AI data flag
	// Teleportation sets and TimerEvent the event of modifier 41. AIFlag20 opens the wounded teleport (Brain.CanTeleport) and the aura stats of
	// Aura Enchanted are applied to the monster (wave 21); the timer event is still only recorded.
	Skills     []UModSkill
	AIFlag20   bool
	TimerEvent bool
	// Applied is set once the init callbacks ran, so they never run twice on one unit.
	Applied bool
}

// SetUModSpeed sets the speed bonus the modifiers give (percent of the base speed).
func (m *Monster) SetUModSpeed(pct int) {
	if m.umodSpeedPct == pct {
		return
	}

	m.umodSpeedPct = pct
	m.UMod.SpeedPct = pct
	m.refreshSpeed()
}
