package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Rescue on Mount Arreat, the world side (notes: w22-quest-world.md). The barbarian prisoners are monsters of
// class 534 (monstats "act5pow", AI Wussie). VERIFIED in the exe (quest handler 0x585f30) and in D2MOO (A5Q2.cpp):
//   - every cage object (objects.txt 473) spawns five prisoners (three cages, fifteen barbarians);
//   - when a prison door (class 434) dies, the prisoners alive within 15 subtiles of the door are freed (the count is
//     added to the freed total, their ids are remembered);
//   - a portal (Cain portal object) is placed two subtiles east of the dead door, the freed prisoners walk to it
//     (Wussie AI 0x5ed4d0, already ported) and are removed when they arrive (within 3 tiles); once one of the group
//     has left, the rest of the group is removed at their next think (the group counter is non-zero);
//   - a freed prisoner that dies counts as killed; five dead prisoners fail the quest.
// UNVERIFIED: the portal is a point here (no Cain portal object is drawn); the cage position is the DS1 object's.

// rescueRange is the radius in subtiles around a dead prison door in which prisoners count as freed (< 15).
const rescueRange = 15

// RescueState is the Rescue on Mount Arreat state (quest node 0x20 data of the exe).
type RescueState struct {
	Groups [3]rescueGroup
	// Spawned is the number of prisoners the cages made, Freed those a prison door released, Killed the freed ones
	// that died.
	Spawned, Freed, Killed int

	freedIDs map[uint32]bool // brain ids of the freed prisoners
	nGroups  int
}

type rescueGroup struct {
	ids     []uint32 // brain ids of the five prisoners
	portal  *d2monster.Point
	left    int // prisoners of the group that reached the portal (+0x114)
	spawned bool
}

// Rescue returns the state (created on first use).
func (d *Director) Rescue() *RescueState {
	if d.rescue == nil {
		d.rescue = &RescueState{freedIDs: map[uint32]bool{}}
	}

	return d.rescue
}

// ResetRescue forgets the state (a new level build).
func (d *Director) ResetRescue() { d.rescue = nil }

// SpawnRescueGroup puts the five prisoners of one cage around a subtile position (ACT5Q2_SpawnCagedWussies) and
// returns how many were placed. At most three cages are served.
func (d *Director) SpawnRescueGroup(stat *d2records.MonStatRecord, subX, subY int) int {
	s := d.Rescue()
	if s.nGroups >= len(s.Groups) {
		return 0
	}

	g := &s.Groups[s.nGroups]
	s.nGroups++
	g.spawned = true

	for i := 0; i < 5; i++ {
		m, err := d.SpawnNear(stat, subX, subY, 3)
		if err != nil {
			continue
		}

		if u := d.byEntity[m.ID()]; u != nil {
			g.ids = append(g.ids, u.b.ID)
		}
	}

	s.Spawned += len(g.ids)

	return len(g.ids)
}

func (s *RescueState) groupOf(id uint32) int {
	for i := range s.Groups {
		for _, x := range s.Groups[i].ids {
			if x == id {
				return i
			}
		}
	}

	return -1
}

// RescueDoorDied handles the death of a prison door monster (ev8 of the exe handler): the prisoners alive within
// 15 subtiles are freed, the portal of their group opens two subtiles east of the door. Returns the number freed.
func (d *Director) RescueDoorDied(door *d2mapentity.Monster) int {
	du := d.byEntity[door.ID()]
	if du == nil {
		return 0
	}

	return d.RescueDoorAt(du.b.X, du.b.Y)
}

// RescueDoorAt is RescueDoorDied for a door at a subtile position.
func (d *Director) RescueDoorAt(subX, subY int) int {
	s := d.Rescue()
	freed := 0

	for i := range s.Groups {
		g := &s.Groups[i]
		hit := false

		for _, id := range g.ids {
			u := d.units[id]
			if u == nil || !u.m.Alive() || s.freedIDs[id] {
				continue
			}

			if d2monster.Distance(u.b.X-subX, u.b.Y-subY) < rescueRange {
				s.freedIDs[id] = true
				freed++
				hit = true
			}
		}

		if hit && g.portal == nil {
			g.portal = &d2monster.Point{X: subX + 2, Y: subY}
		}
	}

	s.Freed += freed
	d.emit("quest", "RESCUE door died: %d prisoner(s) freed (total %d of %d)", freed, s.Freed, s.Spawned)

	return freed
}

// RescuePrisonerDied counts a freed prisoner that was killed (five of them fail the quest). Prisoners killed in
// their cage are not counted by the exe (the id list holds only the freed ones). Returns true when it counted.
func (d *Director) RescuePrisonerDied(m *d2mapentity.Monster) bool {
	if u := d.byEntity[m.ID()]; u != nil {
		return d.RescueBrainDied(u.b.ID)
	}

	return false
}

// RescueBrainDied is RescuePrisonerDied for a brain id.
func (d *Director) RescueBrainDied(id uint32) bool {
	s := d.rescue
	if s == nil || !s.freedIDs[id] {
		return false
	}

	s.Killed++
	d.emit("quest", "RESCUE a freed prisoner died (%d killed)", s.Killed)

	return true
}

// ---- d2monster.FBNWussieHost ----

var _ d2monster.FBNWussieHost = (*Director)(nil)

// SavedGroupUnit implements FBNWussieHost: the group is saved once its portal stands; the portal is the target.
func (d *Director) SavedGroupUnit(b *d2monster.Brain) (d2monster.Target, bool, bool) {
	s := d.rescue
	if s == nil {
		return d2monster.Target{}, false, false
	}

	gi := s.groupOf(b.ID)
	if gi < 0 || s.Groups[gi].portal == nil {
		return d2monster.Target{}, false, false
	}

	p := s.Groups[gi].portal

	return d2monster.Target{X: p.X, Y: p.Y}, true, true
}

// GroupCounted implements FBNWussieHost (+0x114 of the group is non-zero).
func (d *Director) GroupCounted(b *d2monster.Brain) bool {
	s := d.rescue
	if s == nil {
		return false
	}

	gi := s.groupOf(b.ID)

	return gi >= 0 && s.Groups[gi].left > 0
}

// CorpseNear implements FBNWussieHost: no unfreed prisoner reacts here (the player-nearby host is not wired).
func (d *Director) CorpseNear(*d2monster.Brain) bool { return false }

// NearGroup implements FBNWussieHost (nothing to book-keep).
func (d *Director) NearGroup(*d2monster.Brain) {}

// Refresh implements FBNWussieHost (the quest log refresh is done by the quest system).
func (d *Director) Refresh(*d2monster.Brain) {}

// WussieShout implements FBNWussieHost (the help packet is a sound only).
func (d *Director) WussieShout(*d2monster.Brain, d2monster.Target) {}

// Rescued implements FBNWussieHost (ACT5Q2_UpdateWussieCounters + removal): the group counter rises, the prisoner
// leaves the level.
func (d *Director) Rescued(b *d2monster.Brain) {
	u := d.units[b.ID]
	if u == nil {
		return
	}

	if s := d.rescue; s != nil {
		if gi := s.groupOf(b.ID); gi >= 0 {
			s.Groups[gi].left++
		}
	}

	d.emit("quest", "RESCUE prisoner %d reached the portal and left", b.ID)
	if d.engine != nil {
		d.engine.RemoveEntity(u.m)
	}

	d.forget(u)
}
