package d2monster

// Batch 8 (notes: ai-faithful-batch8.md in the reverse-engineering notes)
// finishes the handlers batch 7 left open and re-reads the next ones.

// FaithfulBatch8 lists the handlers batch 8 counts as newly faithful: each was
// compared with the exe and either corrected or had its UNVERIFIED simplification
// resolved from the binary.
//
//	GreaterMummy (corpse callback decoded, Skill2 is the wounded-neighbour cast),
//	Vampire (the walk-away speed reset), ZakarumPriest (teleport lands past the
//	target), SandMaggot (rewritten: Skill3 spit, attack-target distance, waits),
//	OblivionKnight (ally distance to the target, sixth slot, scan 0x5f9fb0),
//	Overseer (scan 0x5e1570, alignment test), PutridDefiler (scan 0x5eeb80),
//	ShadowWarrior (aip8 cap reads the Hell column), MaggotEgg (victim and killer
//	of the death call), BloodLord (Sk1mode is A2 for every class) and
//	FrozenHorror (cast gate is distance below the skill level).
func FaithfulBatch8() []string {
	return []string{"GreaterMummy", "Vampire", "ZakarumPriest", "SandMaggot", "OblivionKnight", "Overseer",
		"PutridDefiler", "ShadowWarrior", "MaggotEgg", "BloodLord", "FrozenHorror"}
}

// ReverifiedBatch8 lists handlers read against the exe in batch 8 that already
// matched (not counted again): VileDog, ThornHulk, ZakarumZealot (its fear gate
// needs the Act III quest flag, not hosted), PinHead, QuillMother, Sarcophagus,
// SandMaggotQueen, FrogDemon, ReanimatedHorde, SiegeTower, SiegeBeast,
// Regurgitator, VileMother, TrappedSoul, Spirit, TrapMelee, MaggotLarva,
// FoulCrowNest, MosquitoNest, Catapult, ElementalBeast, FlyingScimitar,
// FingerMage, HellMeteor, InvisoPet, CorruptLancer and Nihlathak (flow; its
// scan result was misread and is fixed).
func ReverifiedBatch8() []string {
	return []string{"VileDog", "ThornHulk", "ZakarumZealot", "PinHead", "QuillMother", "Sarcophagus",
		"SandMaggotQueen", "FrogDemon", "ReanimatedHorde", "SiegeTower", "SiegeBeast", "Regurgitator", "VileMother",
		"TrappedSoul", "Spirit", "Trap-Melee", "MaggotLarva", "FoulCrowNest", "MosquitoNest", "Catapult",
		"ElementalBeast", "FlyingScimitar", "FingerMage", "HellMeteor", "InvisoPet", "CorruptLancer", "Nihlathak"}
}

// radamentClass is monstats row 229 (0xe5), the class the Greater Mummy think
// function widens its scan for.
const radamentClass = 229

// MummyScan is the result of the Greater Mummy neighbour scan (callback
// 0x5f1a00 of MONAI_ForEachUnitByModeCallback): the last dead neighbour in
// reach (Corpse), whether a wounded living neighbour was seen (Wounded), and
// how many neighbours passed the filters and the radius test (InReach).
type MummyScan struct {
	Corpse  *Target
	Wounded bool
	InReach int
}

// MummyScanner is the optional world hook for the scan. radament selects the
// exe's alternative filter (undead monsters only, flag-8 monsters allowed in
// Normal difficulty); radius is the unsquared reach in subtiles.
type MummyScanner interface {
	MummyScan(b *Brain, radius int, radament bool) MummyScan
}
