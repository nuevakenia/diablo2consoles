package d2monsters

import "testing"

// The hero's own summons are friendly (a click on them must not start an attack); an ordinary monster is not.
func TestIsFriendlyForSummonsOnly(t *testing.T) {
	pet := golemUnit(1, golemOwner(500), 0, 0, 300, 300)
	foe := petUnit(2, 5, 5, "NecroPet", "minion")
	foe.ally = nil
	foe.b.Allied = false
	// the test monsters share an empty entity id, so each gets its own director
	dp, df := testDirector(pet), testDirector(foe)
	dp.byEntity[pet.m.ID()] = pet
	df.byEntity[foe.m.ID()] = foe

	d := dp

	if !d.IsFriendly(pet.m) {
		t.Error("a summon of the hero must be friendly")
	}

	if df.IsFriendly(foe.m) {
		t.Error("an ordinary monster is not friendly")
	}
}
