package d2monster

import "testing"

// FallenShaman 0x5f04d0: the Skill2 chance is rolled even when the slot is undefined (w38).
func TestW38FallenShamanRollsUndefinedSkill2(t *testing.T) {
	// aip1 resurrect%, aip2 fire%, aip3 melee%, aip4 corpse radius, aip5 fire range
	b := brainAt(profile("FallenShaman", 0, 100, 0, 5, 50))
	w := newFake(5, false)
	exp := shadow(b)

	if !Tick(w, b) {
		t.Fatal("think did not run")
	}

	exp.Roll(100) // the alert broadcast roll (aip1)
	exp.Roll(100) // the Skill2 roll, taken although the slot is empty

	if *b.Seed != *exp {
		t.Errorf("seed after tick differs: expected two rolls")
	}

	if len(w.log) != 0 {
		t.Errorf("an undefined skill must queue nothing: %v", w.log)
	}
}

func TestW38FallenShamanFiresSkill2(t *testing.T) {
	p := profile("FallenShaman", 0, 100, 0, 5, 50)
	p.Skills[1] = SkillSlot{Name: "fire"}
	b := brainAt(p)
	w := newFake(5, false)

	Tick(w, b)

	if w.last() != "cast1" {
		t.Errorf("shaman fire: %v", w.log)
	}
}

func TestW38FallenShamanMelee(t *testing.T) {
	b := brainAt(profile("FallenShaman", 0, 0, 100, 5, 50))
	w := newFake(2, true)

	Tick(w, b)

	if w.last() != "attack4" {
		t.Errorf("melee: %v", w.log)
	}
}
