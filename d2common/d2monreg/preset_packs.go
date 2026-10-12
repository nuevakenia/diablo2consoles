package d2monreg

// The preset (DS1) monster nodes that make packs: place_unique_pack, place_champion, place_impgroup /
// place_miniongroup (FUN_0054c470 cases 2, 3, 0x18 / 0x1a) and the tentacles (cases 10 / 11). Read from the
// decompilation and the disassembly of Game.exe 1.14b; no emulator golden exists for them, so the numbers in the tests are
// the formulas themselves and the stream order is UNVERIFIED against the original.

// ChampionModID is the monumod.txt id of the "champion" row (16 in the stock table): MONSTER_SetFlagsAndAddUniqueMod
// 0x5a2390 is called with this constant by the place_champion node, which sets the unit's type word bits 1 and 4 and
// appends the id to the unit's modifier list.
const ChampionModID = 16

// PresetUniquePack is case 2 (place_unique_pack): the class comes from the level's umon palette (FUN_005bba00 mode 1), the
// unit from FUN_005a1ec0 with no cell (the position search covers the room, the node's own position is not used) and
// the champion roll switched off (argument 5 = 0). The rest is the natural unique branch: a rare pack of the region's
// unique counter, 3..6 minions.
func (g *Game) PresetUniquePack(w World, room *Room, pop *Population) bool {
	r := g.region(room)
	if r == nil {
		return false
	}

	return g.uniquePackWith(w, room, r, nil, pop, false)
}

// PresetChampion is case 3 (place_champion): a class from the palette (mode 1) is created on the node's spot with the
// exact-spot placement (ring -1, no retry, flags 0 so the party pack of the class is made), then the unit gets the
// champion flags and the modifier ChampionModID without any seed step, then the champion minions of FUN_0054c050
// (1..3 on the ring of 4, each a champion with the leader's xfer modifiers).
func (g *Game) PresetChampion(w World, room *Room, x, y int, pop *Population) *Unit {
	r := g.region(room)
	if r == nil {
		return nil
	}

	class, ok := g.pickType(r, room, 0, true)
	if !ok {
		return nil
	}

	px, py, ok := g.Tables.Place(w, Placement{Room: room, Class: class, X: x, Y: y, Ring: -1})
	if !ok {
		return nil
	}

	u := g.create(pop, room, class, px, py)
	g.partyPack(w, room, nil, u, class, pop)

	u.Unique, u.Champion = true, true
	u.Mods = append(u.Mods, ChampionModID)

	n := int(u.Seed.Roll(3)) + 1

	for ; n > 0; n-- {
		if m := g.follower(w, room, nil, u, class, 4, pop); m != nil {
			m.Champion = true
			g.link(u, m)
			g.partyPack(w, room, nil, m, class, pop)
		}
	}

	return u
}

// cellAtPoint stands in for DRLG_GetLogicCellAtPoint: the region value of the point, as a cell that covers the room
// (nil when the point is in no region, which makes the node create nothing).
func (g *Game) cellAtPoint(w World, room *Room, x, y int) *Cell {
	v := w.CellAt(room, x, y)
	if v == 0 {
		return nil
	}

	return &Cell{X0: room.X / 5, Y0: room.Y / 5, X1: (room.X + room.W) / 5, Y1: (room.Y + room.H) / 5, Flag: v}
}

// PresetGroup is MONSTER_SpawnMonsterWithMinionsAtCell (0x54bf10), the place_impgroup / place_miniongroup nodes of the
// normal difficulty: the monstats MinGrp / MaxGrp of the class are used as they are (no FUN_0054ca80 substitution),
// the leader goes to the node's exact spot (ring -1, flags 0) inside the region found at the point, then
// Roll(leader seed, max-min+1) + min-1 followers on the ring of 3. UNVERIFIED: the first argument of the placement is the region
// value of the point here, taken as "the region the units must stay in".
func (g *Game) PresetGroup(w World, room *Room, class, x, y int, pop *Population) *Unit {
	if class < 0 || class >= len(g.Tables.Mons) {
		return nil
	}

	lo, hi := g.Tables.Mons[class].MinGrp, g.Tables.Mons[class].MaxGrp
	if lo == 0 || hi == 0 || lo > hi {
		return nil
	}

	c := g.cellAtPoint(w, room, x, y)
	if c == nil {
		return nil
	}

	px, py, ok := g.Tables.Place(w, Placement{Room: room, Cell: c, Class: class, X: x, Y: y, Ring: -1, Param4: 1})
	if !ok {
		return nil
	}

	leader := g.create(pop, room, class, px, py)
	g.partyPack(w, room, c, leader, class, pop)

	lo--
	hi--

	n := int(leader.Seed.Roll(int32(hi-lo+1))) + lo

	for ; n > 0; n-- {
		if f := g.follower(w, room, c, leader, class, 3, pop); f != nil {
			g.partyPack(w, room, c, f, class, pop)
		}
	}

	return leader
}

// TentacleHeadClass is the class the tentacle nodes start from (0x105, tentaclehead1 in the stock table).
const TentacleHeadClass = 0x105

// LinkedClass is MONSTER_GetNextLinkedClassAtDepth (0x54b870): depth 0 gives the BaseId of the class, every further
// step follows NextInClass; -1 for a class outside the table, and the walk returns the first invalid link as it is.
func (t *Tables) LinkedClass(class, depth int) int {
	if class < 0 || class >= len(t.Mons) {
		return -1
	}

	cur := t.Mons[class].BaseID
	m := &t.Mons[class]

	for i := 0; i < depth; i++ {
		cur = m.NextInClass
		if cur < 0 || cur >= len(t.Mons) {
			return cur
		}

		m = &t.Mons[cur]
	}

	return cur
}
