package d2skill

import "testing"

func TestCurseFrames(t *testing.T) {
	for _, tc := range []struct {
		name        string
		do          int
		state       string
		frames, div int
		want        int
	}{
		{"amplify unchanged in hell", 30, "amplifydamage", 1000, 4, 1000},
		{"dim vision nightmare", 30, "dimvision", 1000, 2, 500},
		{"terror hell", 30, "terror", 1000, 4, 250},
		{"terror normal", 30, "terror", 1000, 1, 1000},
		{"attract always divides", 59, "attract", 1000, 4, 250},
		{"confuse always divides", 61, "confuse", 1001, 2, 500},
		{"no divisor loaded", 59, "attract", 1000, 0, 1000},
		{"life tap unchanged", 30, "lifetap", 600, 4, 600},
	} {
		if got := CurseFrames(tc.do, tc.state, tc.frames, tc.div); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestCurseStateRules(t *testing.T) {
	for state, want := range map[string]bool{"dimvision": true, "terror": true, "confuse": false, "attract": false, "weaken": false} {
		if CurseMonstersOnly(state) != want || CurseDivisorState(state) != want {
			t.Errorf("%s: want %v", state, want)
		}
	}

	for state, want := range map[string]int{"dimvision": 10, "taunt": 12, "terror": 11, "weaken": 0, "": 0} {
		if got := CursedAIState(state); got != want {
			t.Errorf("AI state of %q: %d, want %d", state, got, want)
		}
	}
}

func TestSummonResistMods(t *testing.T) {
	none := func(string) int { return 0 }

	if SummonResistMods(0, none) != nil {
		t.Fatal("zero Summon Resist attaches nothing")
	}

	got := SummonResistMods(40, none)
	if len(got) != 4 || got[0] != (StatMod{Stat: "fireresist", Value: 40}) || got[3] != (StatMod{Stat: "poisonresist", Value: 40}) {
		t.Fatalf("plain minion: %+v", got)
	}

	// a Fire Golem absorbs fire: no fire resistance is added, poison always is
	fire := func(n string) int {
		if n == "item_absorbfire_percent" {
			return 20
		}

		return 0
	}

	got = SummonResistMods(40, fire)
	if len(got) != 3 || got[0].Stat != "lightresist" || got[2].Stat != "poisonresist" {
		t.Fatalf("fire golem: %+v", got)
	}
}

func TestAttachesSummonMastery(t *testing.T) {
	for do, want := range map[int]bool{31: true, 56: true, 57: true, 58: false, 119: false, 60: false} {
		if AttachesSummonMastery(do) != want {
			t.Errorf("do %d", do)
		}
	}
}

func TestCurseAlignmentOK(t *testing.T) {
	for _, tc := range []struct {
		do        int
		state     string
		alignment int
		want      bool
	}{
		{59, "attract", 0, true}, {59, "attract", 1, false}, {59, "attract", 2, false},
		{61, "confuse", 0, true}, {61, "confuse", 2, false},
		{30, "terror", 1, false}, {30, "terror", 2, true}, {30, "dimvision", 0, true},
		{30, "weaken", 1, true},
	} {
		if got := CurseAlignmentOK(tc.do, tc.state, tc.alignment); got != tc.want {
			t.Errorf("%+v: %v", tc, got)
		}
	}
}

func TestRaiseSkeletonLifeCost(t *testing.T) {
	if RaiseSkeletonLifeCost(2, 800) != 0 {
		t.Error("a Necromancer pays nothing")
	}

	if RaiseSkeletonLifeCost(3, 800) != 100 {
		t.Error("class index 3 pays an eighth")
	}
}
