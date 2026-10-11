package d2monster

// Batch 8 (notes: ai-faithful-batch8.md in the reverse-engineering notes)
// finishes the handlers batch 7 left open and re-reads the next ones.

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
