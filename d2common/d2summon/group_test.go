package d2summon

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// Wave 23: pet types that share a PetType.txt group release each other
// (PETS_ReleaseSharedLimitTypePets 0x573520), and a one-at-a-time summon at
// its limit releases the oldest pet of its own type (0x573a30).
func TestSharedGroupReleasesOtherTypes(t *testing.T) {
	groups := map[string]int{"spiritwolf": 3, "fenris": 3, "grizzly": 3, "raven": 0, "totem": 0}
	r := &Roster{Group: func(p string) int { return groups[p] }}

	wolf := &d2skill.SummonOrder{Key: "spiritwolf", PetType: "spiritwolf", Max: 3, Count: 1}
	for id := uint32(1); id <= 3; id++ {
		r.Apply(wolf, r.Plan(wolf), []uint32{id}, 0)
	}

	// a fourth wolf at the limit releases the oldest wolf only
	if p := r.Plan(wolf); len(p.Evict) != 1 || p.Evict[0] != 1 {
		t.Fatalf("limit eviction %+v", p)
	}

	// a Dire Wolf releases the three spirit wolves
	fen := &d2skill.SummonOrder{Key: "fenris", PetType: "fenris", Max: 1, Count: 1}
	p := r.Plan(fen)

	if len(p.Evict) != 3 {
		t.Fatalf("group release %+v", p)
	}

	r.Apply(fen, p, []uint32{10}, 0)

	if r.Count("spiritwolf") != 0 || r.Count("fenris") != 1 {
		t.Errorf("roster after: wolves %d dire %d", r.Count("spiritwolf"), r.Count("fenris"))
	}

	// a type outside any group is left alone
	raven := &d2skill.SummonOrder{Key: "raven", PetType: "raven", Max: 5, Count: 1}
	if p := r.Plan(raven); len(p.Evict) != 0 {
		t.Errorf("raven released %v", p.Evict)
	}

	// without group data nothing is shared
	r2 := &Roster{}
	r2.Apply(wolf, r2.Plan(wolf), []uint32{1}, 0)

	if p := r2.Plan(fen); len(p.Evict) != 0 {
		t.Errorf("no groups yet released %v", p.Evict)
	}
}
