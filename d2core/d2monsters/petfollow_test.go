package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

func TestDruidPetsUsePetAI(t *testing.T) {
	for _, tc := range []struct {
		ai, kind string
		want     bool
	}{
		{"DruidWolf", "minion", true},
		{"druidbear", "minion", true},
		{"DruidWolf", "totem", false},
		{"Vines", "minion", false}, // the vines are skill-engine traps
		{"CycleOfLife", "minion", false},
	} {
		if got := usesPetAI(petUnit(1, 0, 0, tc.ai, tc.kind)); got != tc.want {
			t.Errorf("%s/%s: got %v want %v", tc.ai, tc.kind, got, tc.want)
		}
	}
}

func TestRavenPecks(t *testing.T) {
	r := petUnit(1, 0, 0, "Raven", "minion")
	d := testDirector(r)

	if n := d.FBNRavenPecks(r.b); n != 3 {
		t.Errorf("default pecks = %d", n)
	}

	r.ally.opt.Pecks = 12 + 9 // Param5 12, Param6 1, level 10

	if n := d.FBNRavenPecks(r.b); n != 21 {
		t.Errorf("pecks = %d", n)
	}
}

func TestPetCountAndAdjacent(t *testing.T) {
	owner := &d2mapentity.Player{}
	other := &d2mapentity.Player{}
	a := petUnit(1, 10, 10, "DruidWolf", "minion")
	b := petUnit(2, 11, 10, "DruidWolf", "minion")
	c := petUnit(3, 40, 10, "DruidBear", "minion")
	e := petUnit(4, 10, 11, "NecroPet", "minion")
	a.ally.owner, b.ally.owner, c.ally.owner, e.ally.owner = owner, owner, owner, other

	d := testDirector(a, b, c, e)

	if n := d.FBNPetCount(a.b); n != 3 {
		t.Errorf("pet count = %d, want 3 (only the owner's minions)", n)
	}

	if n := d.FBNPetsAdjacent(a.b); n != 1 {
		t.Errorf("adjacent = %d, want 1 (b; c is far and e belongs to somebody else)", n)
	}
}
