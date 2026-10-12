package d2monster

import "testing"

func TestShadowLevels(t *testing.T) {
	// 0x5ea490: ownerBase/2 + shadowSkill/2, clamped 1..24
	for _, tc := range []struct{ owner, sh, want int }{
		{1, 1, 1}, {1, 20, 10}, {20, 20, 20}, {30, 30, 24}, {0, 0, 1}, {3, 3, 2}, {2, 1, 1},
	} {
		if got := ShadowMasterLevel(tc.owner, tc.sh); got != tc.want {
			t.Errorf("master level(%d,%d) = %d, want %d", tc.owner, tc.sh, got, tc.want)
		}
	}

	// 0x5e9f50: shadowSkill/3 + ownerTotal/2, at least 1 (a result of 1 or less is 1)
	for _, tc := range []struct{ sh, owner, want int }{
		{1, 1, 1}, {20, 20, 16}, {21, 20, 17}, {0, 0, 1}, {3, 0, 1}, {6, 3, 3}, {30, 25, 22},
	} {
		if got := ShadowWarriorLevel(tc.sh, tc.owner); got != tc.want {
			t.Errorf("warrior level(%d,%d) = %d, want %d", tc.sh, tc.owner, got, tc.want)
		}
	}
}

func TestShadowManaWhole(t *testing.T) {
	// Shadow Warrior: mana 54, lvlmana 1, manashift 7: level 20 -> (19 + 54) << 7 >> 8 = 36
	if got := ShadowManaWhole(54, 1, 7, 20); got != 36 {
		t.Errorf("shadow warrior cost %d, want 36", got)
	}

	// Dragon Talon: mana 6, shift 8 -> 6
	if got := ShadowManaWhole(6, 0, 8, 12); got != 6 {
		t.Errorf("dragon talon cost %d, want 6", got)
	}

	// a negative result is 0
	if got := ShadowManaWhole(-5, 0, 8, 1); got != 0 {
		t.Errorf("negative cost %d, want 0", got)
	}
}

func TestShadowCopyAllowed(t *testing.T) {
	for _, tc := range []struct {
		name           string
		summons, own   bool
		pet, shadowPet int
		owner, want    bool
	}{
		{"no summon", false, false, 3, 3, true, true},
		{"summons the shadow's own class", true, true, -1, -1, true, false},
		{"same pet type as the shadow", true, false, 4, 4, true, false},
		{"other pet type (sentry)", true, false, 5, 4, true, true},
		{"no pet type", true, false, -1, 4, true, true},
		{"no owner lets the pet type through", true, false, 4, 4, false, true},
	} {
		if got := ShadowCopyAllowed(tc.summons, tc.own, tc.pet, tc.shadowPet, tc.owner); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestShadowValidateCopy(t *testing.T) {
	melee := ShadowSkill{ID: 254, AIType: 4, Bits4: 4, AuraState: 3, AuraStat1: 9}
	buff := ShadowSkill{ID: 258, AIType: 1, AuraState: 5}
	curse := ShadowSkill{ID: 99, AIType: 2, AuraState: 6, AuraTargetState: 7}
	missile := ShadowSkill{ID: 261, AIType: 5}

	ok := func(in ShadowValidateIn) bool { in.Allowed = true; in.HasTarget = true; return ShadowValidateCopy(in) }

	// melee kinds only in reach, the others only out of it
	if !ok(ShadowValidateIn{Skill: melee, InRange: true}) || ok(ShadowValidateIn{Skill: melee}) {
		t.Error("melee kinds are for reach only")
	}

	if ok(ShadowValidateIn{Skill: missile, InRange: true}) || !ok(ShadowValidateIn{Skill: missile}) {
		t.Error("ranged kinds are for out of reach only")
	}

	// a buff the shadow carries
	if ok(ShadowValidateIn{Skill: buff, SelfState: true}) || !ok(ShadowValidateIn{Skill: buff}) {
		t.Error("buff already on")
	}

	// a curse on either side
	if ok(ShadowValidateIn{Skill: curse, TargetHas: true}) || ok(ShadowValidateIn{Skill: curse, SelfState: true}) ||
		!ok(ShadowValidateIn{Skill: curse}) {
		t.Error("curse states")
	}

	// progressive: three charges stop the skill
	if ok(ShadowValidateIn{Skill: melee, InRange: true, SelfState: true, StateStat: 3, StateStatOK: true}) ||
		!ok(ShadowValidateIn{Skill: melee, InRange: true, SelfState: true, StateStat: 2, StateStatOK: true}) {
		t.Error("progressive charges")
	}

	// no target: the target kinds are out
	if ShadowValidateCopy(ShadowValidateIn{Skill: missile, Allowed: true}) {
		t.Error("a missile needs a target")
	}

	if !ShadowValidateCopy(ShadowValidateIn{Skill: buff, Allowed: true}) {
		t.Error("a buff needs no target")
	}

	// not allowed
	if ShadowValidateCopy(ShadowValidateIn{Skill: buff, HasTarget: true}) {
		t.Error("not allowed")
	}
}

func shadowWarriorBrain() *Brain {
	b := fb1Brain("ShadowWarrior", 20, 20, 100)
	b.Profile.AIPRaw[8] = [3]int{268, 5, 64}
	b.Scratch = [3]int{0, 0, 20}

	return b
}

func TestShadowCopyCast(t *testing.T) {
	in := ShadowCopyIn{Frame: 100, OwnerClass: 6, SkillClass: 6, SkillID: 254, Valid: true, Cost: 2}

	// class mismatch (the plain attack has class 7) and an invalid skill fail before any roll
	b := shadowWarriorBrain()
	before := *b.Seed

	bad := in
	bad.SkillClass = 7

	if ShadowCopyCast(b, bad) || *b.Seed != before {
		t.Error("a foreign class must fail without a roll")
	}

	bad = in
	bad.Valid = false

	if ShadowCopyCast(b, bad) || *b.Seed != before {
		t.Error("an invalid skill must fail without a roll")
	}

	// the plain attack passes after validation, without a roll
	plain := in
	plain.SkillID, plain.SkillClass = 0, 6

	if !ShadowCopyCast(b, plain) || *b.Seed != before {
		t.Error("skill 0 passes at once")
	}

	// the cooldown gate: the mana roll is drawn first, then nothing more
	cd := shadowWarriorBrain()
	cd.Scratch[0] = 500
	s0 := *cd.Seed

	r := cd.Roll(100)
	*cd.Seed = s0

	if r <= 100-in.Cost*0xa0/100 {
		if ShadowCopyCast(cd, in) {
			t.Error("cooldown must refuse")
		}

		probe := shadowWarriorBrain()
		probe.Roll(100)

		if *cd.Seed != *probe.Seed {
			t.Error("cooldown refusal draws exactly the mana roll")
		}
	}

	// over many tries the fatigue counter grows by (0x140 - level) * cost / (level + 100) per success, starts at its
	// floor, and the success rate falls as it grows
	succ, fat := 0, 0
	fb := shadowWarriorBrain()

	for i := 0; i < 400; i++ {
		fb.Scratch[0] = 0

		prev := fb.Scratch[1]
		if ShadowCopyCast(fb, in) {
			succ++

			if fb.Scratch[1] < prev {
				t.Fatal("fatigue must not fall on a success")
			}

			if i == 0 {
				fat = fb.Scratch[1]
			}
		}
	}

	if succ == 0 || succ == 400 {
		t.Errorf("fatigue should gate some casts, got %d of 400", succ)
	}

	// first success: floor 5 plus (320-20)*2/(20+100) = 5
	if fat != 5+(0x140-20)*2/(20+100) {
		t.Errorf("first fatigue %d", fat)
	}

	// the counter is reset to the floor when it is over the cap (cap 64 * 32)
	of := shadowWarriorBrain()
	of.Scratch[1] = 64*32 + 1
	ShadowCopyCast(of, in)

	if of.Scratch[1] > 64*32 {
		t.Errorf("fatigue above the cap not reset: %d", of.Scratch[1])
	}
}

func TestShadowCooldown(t *testing.T) {
	if got := ShadowCooldown(30, 1000); got != 1028 {
		t.Errorf("cooldown %d, want 1028", got)
	}
}

type upkeepEnv struct {
	has, other map[int]bool
	left       bool
	selected   []int
}

func (e *upkeepEnv) SelfHasState(_ *Brain, s int) bool      { return e.has[s] }
func (e *upkeepEnv) OtherInStateGroup(_ *Brain, s int) bool { return e.other[s] }
func (e *upkeepEnv) LeftSkillSet(*Brain) bool               { return e.left }
func (e *upkeepEnv) SelectActiveSkill(_ *Brain, id int) {
	e.selected = append(e.selected, id)
	e.left = true
}

func TestShadowUpkeep(t *testing.T) {
	skills := []ShadowSkill{
		{ID: 0},
		{ID: 258, AIType: 1, AuraState: 5}, // Burst of Speed
		{ID: 267, AIType: 1, AuraState: 6}, // Fade
		{ID: 70, AIType: 6},
	}

	// a buff the shadow lacks is cast on a 60 percent roll; count over many seeds
	cast := 0

	for id := uint32(1); id <= 300; id++ {
		b := NewBrain(id, 1, Normal, &Profile{}, testSeed)
		env := &upkeepEnv{has: map[int]bool{}, other: map[int]bool{}}

		var got []int

		ShadowUpkeep(b, env, skills, func(s int) bool { got = append(got, s); return true })

		if len(got) > 1 {
			t.Fatal("the loop must end at the first cast")
		}

		if len(got) == 1 {
			if got[0] != 258 && got[0] != 267 {
				t.Fatalf("cast %d", got[0])
			}

			cast++
		}
	}

	// two buffs each 60 percent: 1 - 0.4*0.4 = 84 percent expected
	if cast < 200 || cast > 290 {
		t.Errorf("upkeep cast %d of 300, expected near 252", cast)
	}

	// a state the shadow has is not recast; the aitype 6 skill sets the left skill with 20 percent while none is set
	b := NewBrain(7, 1, Normal, &Profile{}, testSeed)
	env := &upkeepEnv{has: map[int]bool{5: true, 6: true}, other: map[int]bool{}}

	if ShadowUpkeep(b, env, skills, func(int) bool { t.Fatal("no recast"); return true }) {
		t.Error("nothing to cast")
	}

	picked := 0

	for id := uint32(1); id <= 500; id++ {
		b := NewBrain(id, 1, Normal, &Profile{}, testSeed)
		e := &upkeepEnv{has: map[int]bool{5: true, 6: true}}
		ShadowUpkeep(b, e, skills, func(int) bool { return false })

		if len(e.selected) == 1 && e.selected[0] == 70 {
			picked++
		}
	}

	if picked < 70 || picked > 130 {
		t.Errorf("left skill chosen %d of 500, expected near 100", picked)
	}

	// another state of the same group makes the cast need an extra 4 percent roll: it nearly never casts
	rare := 0

	for id := uint32(1); id <= 500; id++ {
		b := NewBrain(id, 1, Normal, &Profile{}, testSeed)
		e := &upkeepEnv{has: map[int]bool{}, other: map[int]bool{5: true, 6: true}, left: true}

		if ShadowUpkeep(b, e, skills, func(int) bool { return true }) {
			rare++
		}
	}

	if rare > 50 {
		t.Errorf("group-blocked buffs cast %d of 500, expected under 10 percent", rare)
	}
}

func TestShadowKindOf(t *testing.T) {
	for name, want := range map[string]ShadowKind{
		"ShadowWarrior": ShadowKindWarrior, "ShadowMaster": ShadowKindMaster,
		"ShadowMasterNoInit": ShadowKindMasterNoInit, "Melee": ShadowKindNone,
	} {
		b := &Brain{Def: &AIDef{Name: name}}
		if got := ShadowKindOf(b); got != want {
			t.Errorf("%s: %d, want %d", name, got, want)
		}
	}

	if ShadowKindOf(nil) != ShadowKindNone || ShadowKindOf(&Brain{}) != ShadowKindNone {
		t.Error("no brain or definition")
	}
}

type preWorld struct {
	*fb1World
	calls []ShadowKind
}

func (p *preWorld) ShadowInit(b *Brain, k ShadowKind) { p.calls = append(p.calls, k); b.Scratch[2] = 9 }

func TestShadowPreRunsOnce(t *testing.T) {
	w := &preWorld{fb1World: newFB1(10, false)}
	b := fb1Brain("ShadowWarrior", 20, 20, 100)

	for i := 0; i < 3; i++ {
		Tick(w, b)
		b.WakeNow(0)
	}

	if len(w.calls) != 1 || w.calls[0] != ShadowKindWarrior {
		t.Errorf("Pre-hook calls %v, want one warrior init", w.calls)
	}

	if b.Scratch[2] != 9 {
		t.Error("the hook's level was not kept")
	}
}
