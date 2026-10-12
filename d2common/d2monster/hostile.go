package d2monster

// Hostility between two units, the model of UNIT_AreUnitsHostile (0x552270) for the hero's own units.
//
// What the notes record of the exe (observations only): the test resolves the owner / leader of each unit first (a
// monster's leader, a missile's owner), answers "not hostile" when both resolve to the same unit, uses the party
// relation flag 8 for two players, and a monster hostility helper otherwise. The missile hit loop (common predicate
// 0x5a6160) and the area scans (filter 0x569100) both call it, so a unit that is not hostile to the caster is never
// hit by a missile that lacks CollideFriend. The monster helper is UNIT_CompareStateStatACRelation 0x652230 (VERIFIED w38): related (not hostile) when the units are the same,
// or both alignments are 0, or both are 2; any pairing involving alignment 1 is hostile, as is 0 against 2. Here every monster that
// follows a hero (summon, golem, mercenary, converted monster) takes its owner's side, every other monster is on the
// monsters' side, and two monsters of the same side never fight.

// Unit is the part of a unit the relation reads.
type Unit struct {
	ID       string
	IsPlayer bool
	// Owner is the unit this one follows (summoner, mercenary master); nil for a free unit.
	Owner *Unit
	// Converted marks a monster that changed sides (Conversion, Revive): it fights for the heroes.
	Converted bool
	// HeroSide is set when the unit is a hero or a monster that fights for the heroes with no recorded owner.
	HeroSide bool
}

// root follows the owner chain (bounded) to the unit that decides the side.
func (u *Unit) root() *Unit {
	cur := u
	for i := 0; i < 8 && cur.Owner != nil; i++ {
		cur = cur.Owner
	}

	return cur
}

// onHeroSide reports whether the unit fights for the heroes.
func (u *Unit) onHeroSide() bool {
	r := u.root()

	return r.IsPlayer || r.HeroSide || u.Converted || u.HeroSide
}

// AreHostile is UNIT_AreUnitsHostile for a and b. PlayersHostile answers the party relation for two players
// (flag 8: nil means "not hostile", the default of a game without a hostility declaration).
func AreHostile(a, b *Unit, playersHostile func(a, b string) bool) bool {
	if a == nil || b == nil || a == b {
		return false
	}

	ra, rb := a.root(), b.root()
	if ra == rb || ra.ID == rb.ID {
		return false // a unit and its own followers (and followers of one owner) are never hostile
	}

	if ra.IsPlayer && rb.IsPlayer {
		return playersHostile != nil && playersHostile(ra.ID, rb.ID)
	}

	return a.onHeroSide() != b.onHeroSide()
}
