package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

// three cages of five prisoners, the cages 100 subtiles apart
func rescueDirector() (*Director, [3][]*unit) {
	var (
		all    []*unit
		groups [3][]*unit
	)

	id := uint32(1)

	for g := 0; g < 3; g++ {
		for i := 0; i < 5; i++ {
			u := petUnit(id, g*100+i, 0, "Wussie", "")
			id++
			groups[g] = append(groups[g], u)
			all = append(all, u)
		}
	}

	d := testDirector(all...)
	d.Logger = d2util.NewLogger()
	d.fp = newFootprints(nil)
	s := d.Rescue()

	for g := range groups {
		s.Groups[g].spawned = true
		s.nGroups++

		for _, u := range groups[g] {
			s.Groups[g].ids = append(s.Groups[g].ids, u.b.ID)
		}
	}

	s.Spawned = 15

	return d, groups
}

func TestRescueDoorFreesTheNearGroupOnly(t *testing.T) {
	d, _ := rescueDirector()

	if n := d.RescueDoorAt(2, 0); n != 5 {
		t.Fatalf("first door freed %d, want the five of its cage", n)
	}

	if n := d.RescueDoorAt(102, 0); n != 5 {
		t.Fatalf("second door freed %d", n)
	}

	if n := d.RescueDoorAt(2, 0); n != 0 {
		t.Fatalf("the same door freed %d again", n)
	}

	s := d.Rescue()
	if s.Freed != 10 || s.Groups[0].portal == nil || s.Groups[2].portal != nil {
		t.Fatalf("state: freed=%d portals %v/%v", s.Freed, s.Groups[0].portal, s.Groups[2].portal)
	}

	if p := s.Groups[0].portal; p.X != 4 || p.Y != 0 {
		t.Fatalf("portal at %v, want two subtiles east of the door", *p)
	}
}

func TestRescuePrisonerDeadInTheCageIsNotFreed(t *testing.T) {
	d, groups := rescueDirector()
	delete(d.units, groups[1][0].b.ID) // gone before the door fell

	if n := d.RescueDoorAt(102, 0); n != 4 {
		t.Fatalf("freed %d, want 4", n)
	}

	if d.RescueBrainDied(groups[1][0].b.ID) {
		t.Error("a prisoner lost in its cage is not counted as a freed one that died")
	}

	if !d.RescueBrainDied(groups[1][1].b.ID) || d.Rescue().Killed != 1 {
		t.Error("a freed prisoner's death is counted")
	}
}

func TestRescueWussieWalksToThePortalAndLeaves(t *testing.T) {
	d, groups := rescueDirector()
	u := groups[0][0]

	if _, _, saved := d.SavedGroupUnit(u.b); saved {
		t.Fatal("saved before any door fell")
	}

	d.RescueDoorAt(2, 0)

	tg, ok, saved := d.SavedGroupUnit(u.b)
	if !ok || !saved || tg.X != 4 {
		t.Fatalf("saved group: %v %v %v", tg, ok, saved)
	}

	if d.GroupCounted(u.b) {
		t.Error("nobody has left yet")
	}

	d.Rescued(u.b)

	if d.units[u.b.ID] != nil || !d.GroupCounted(groups[0][1].b) {
		t.Error("the rescued one must be gone and the group counted (the rest leave at their next think)")
	}

	var _ d2monster.FBNWussieHost = d
}
