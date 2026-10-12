package d2skill

import "testing"

type formUnit struct {
	*testUnit
	form string
}

func (f *formUnit) Shapeshifted() bool         { return f.form != "" }
func (f *formUnit) HasFormState(n string) bool { return f.form != "" && f.form == n }

// TestRestrictOK pins SKILL_CheckStateRestrictions 0x645450.
func TestRestrictOK(t *testing.T) {
	human := &formUnit{}
	wolf := &formUnit{form: "wolf"}
	bear := &formUnit{form: "bear"}

	humanOnly := &Skill{Name: "Firestorm", Restrict: 0}
	always := &Skill{Name: "Raven", Restrict: 1}
	wolfOnly := &Skill{Name: "Fury", Restrict: 2, FormStates: [3]string{"wolf"}}
	either := &Skill{Name: "Hunger", Restrict: 2, FormStates: [3]string{"wolf", "bear"}}
	noState := &Skill{Name: "bad", Restrict: 2}

	cases := []struct {
		u    *formUnit
		sk   *Skill
		want bool
	}{
		{human, humanOnly, true}, {wolf, humanOnly, false}, {bear, humanOnly, false},
		{human, always, true}, {wolf, always, true}, {bear, always, true},
		{human, wolfOnly, false}, {wolf, wolfOnly, true}, {bear, wolfOnly, false},
		{human, either, false}, {wolf, either, true}, {bear, either, true},
		{wolf, noState, false},
	}

	for i, c := range cases {
		if got := RestrictOK(c.u, c.sk); got != c.want {
			t.Errorf("case %d (%s, form %q): got %v want %v", i, c.sk.Name, c.u.form, got, c.want)
		}
	}

	// a unit that cannot say it is shapeshifted is in human shape
	if !RestrictOK(&testUnit{}, humanOnly) || RestrictOK(&testUnit{}, wolfOnly) {
		t.Error("plain unit must count as human")
	}
}
