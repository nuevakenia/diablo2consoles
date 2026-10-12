package d2mapentity

// UseHeroSpeeds gives a monster the walk and run speed of a hero. The Assassin's shadows need it: their monstats rows
// have Velocity 0 and Run 0, and in the exe the summon carries the "shadowwarrior" state, a shapeshift state with
// gfxtype 2 (a player graphic) whose gfxclass is the Assassin: UNIT_GetShapeshiftAdjustedMode 0x646620 turns the unit
// into a player of that class for the velocity lookup (PATH_UpdateUnitVelocityAndAnimRate 0x624150 reads the
// CharStats walk velocity through UNIT_GetBaseVelocityFixed 0x621550). The hero's own constants stand in for the
// Assassin's CharStats value (UNVERIFIED that they match its walk velocity column).
func (m *Monster) UseHeroSpeeds() {
	m.walkSpeed = baseWalkSpeed
	m.runSpeed = baseRunSpeed
}
