package d2monster

import "testing"

func TestAreHostile(t *testing.T) {
	hero := &Unit{ID: "hero", IsPlayer: true}
	rival := &Unit{ID: "rival", IsPlayer: true}
	raven := &Unit{ID: "raven", Owner: hero}
	golem := &Unit{ID: "golem", Owner: hero}
	spawn := &Unit{ID: "spawn", Owner: golem} // a follower of a follower
	merc := &Unit{ID: "merc", Owner: hero}
	conv := &Unit{ID: "conv", Converted: true}
	foe := &Unit{ID: "foe"}
	foeSummon := &Unit{ID: "foesummon", Owner: foe}
	rivalPet := &Unit{ID: "rivalpet", Owner: rival}

	declared := func(a, b string) bool { return true }

	tests := []struct {
		name string
		a, b *Unit
		ph   func(a, b string) bool
		want bool
	}{
		{"hero vs own raven", hero, raven, nil, false},
		{"raven vs hero", raven, hero, nil, false},
		{"hero vs own golem", hero, golem, nil, false},
		{"hero vs golem's spawn", hero, spawn, nil, false},
		{"hero vs mercenary", hero, merc, nil, false},
		{"raven vs golem", raven, golem, nil, false},
		{"hero vs converted monster", hero, conv, nil, false},
		{"hero vs foe", hero, foe, nil, true},
		{"foe vs hero", foe, hero, nil, true},
		{"raven vs foe", raven, foe, nil, true},
		{"golem vs foe's summon", golem, foeSummon, nil, true},
		{"foe vs its summon", foe, foeSummon, nil, false},
		{"foe vs foe's summon sibling", foeSummon, foe, nil, false},
		{"converted vs foe", conv, foe, nil, true},
		{"same unit", foe, foe, nil, false},
		{"nil", nil, foe, nil, false},
		{"two heroes, no declaration", hero, rival, nil, false},
		{"two heroes, declared", hero, rival, declared, true},
		{"hero vs rival's pet, declared", hero, rivalPet, declared, true},
		{"hero vs rival's pet, not declared", hero, rivalPet, nil, false},
	}

	for _, tc := range tests {
		if got := AreHostile(tc.a, tc.b, tc.ph); got != tc.want {
			t.Errorf("%s: AreHostile=%v want %v", tc.name, got, tc.want)
		}
	}
}
