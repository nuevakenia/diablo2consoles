package d2monster

import "testing"

func fbnPetNew(ai string, ownerDist int, aip ...int) (*fakePet, *Brain) {
	f, _ := newPet(ai, ownerDist, aip...)
	b := brainAt(withSkills(profile(ai, aip...), 0))
	def, _ := Lookup(ai)
	b.SetAI(def)

	return f, b
}

func TestFBNPetsWithoutLeader(t *testing.T) {
	for _, c := range []struct {
		ai   string
		wake int
	}{{"NecroPet", 10}, {"Totem", 10}, {"Raven", 10}, {"Vines", 25}} {
		f, b := fbnPetNew(c.ai, 5, 1, 1, 1, 1, 1)
		f.noOwner = true

		Tick(f, b)

		if len(f.log) != 0 || b.Wake != c.wake {
			t.Errorf("%s: log=%v wake=%d", c.ai, f.log, b.Wake)
		}
	}

	// a remembered leader id lets the pet teleport back
	f, b := fbnPetNew("NecroPet", 5)
	f.noOwner = true
	b.Scratch[2] = 9

	Tick(f, b)

	if f.teleports != 1 || b.Wake != 5 {
		t.Errorf("remembered leader: teleports=%d wake=%d", f.teleports, b.Wake)
	}
}

func TestFBNNecroPetMelee(t *testing.T) {
	for seed := uint32(1); seed < 40; seed++ {
		f, b := fbnPetNew("NecroPet", 5)
		f.hasEnemy, f.enemyDist = true, 4
		b.Seed.Init(seed)
		sh := shadow(b)
		sh.Roll(100) // leash roll
		skip := sh.Roll(100) > 0x4f

		Tick(f, b)

		switch {
		case skip && (len(f.log) != 0 || b.Wake != 10):
			t.Errorf("seed %d: want a 10 frame wait, log=%v wake=%d", seed, f.log, b.Wake)
		case !skip && f.last() != "attack4":
			t.Errorf("seed %d: want attack, got %v", seed, f.log)
		}

		if b.Scratch[2] != 1 {
			t.Errorf("leader id not remembered: %v", b.Scratch)
		}
	}

	// too far from the owner: teleport without any roll
	f, b := fbnPetNew("NecroPet", 60)
	before := *b.Seed

	Tick(f, b)

	if f.teleports != 1 || *b.Seed != before {
		t.Errorf("far pet: teleports=%d", f.teleports)
	}

	// no enemy: one leash roll. Above 14 (leash 7) SelectPetActionByRange picks mode 2: the 10 percent shuffle roll
	// and then ALWAYS a 15 frame wait (VERIFIED 0x5e2dc0 case 2). At 14 or below (leash 8, stay bit) mode 2
	// returns 0 and the 4-step shuffle runs (step, roll, 2 signs).
	sawStay, sawIdle := false, false

	for seed := uint32(1); seed < 60 && !(sawStay && sawIdle); seed++ {
		f, b = fbnPetNew("NecroPet", 5)
		b.Seed.Init(seed)
		sh := shadow(b)

		if sh.Roll(100) > 14 {
			sawIdle = true

			if sh.Roll(100) < 10 { // the shuffle step is not asserted, only the idle wait of the other branch
				continue
			}

			Tick(f, b)

			if len(f.log) != 0 || b.Wake != 15 || *b.Seed != *sh {
				t.Errorf("seed %d idle pet: log=%v wake=%d", seed, f.log, b.Wake)
			}

			continue
		}

		sawStay = true
		sh.Step()
		sh.Roll(4)
		sh.Step()
		sh.Step()

		Tick(f, b)

		if len(f.log) != 1 || *b.Seed != *sh {
			t.Errorf("seed %d stay pet: %v", seed, f.log)
		}
	}

	if !sawStay || !sawIdle {
		t.Errorf("seeds did not reach both branches: stay=%v idle=%v", sawStay, sawIdle)
	}
}

func TestFBNNecroPetCaster(t *testing.T) {
	for seed := uint32(1); seed < 40; seed++ {
		f, b := fbnPetNew("NecroPet", 5)
		b.Scratch[0] = 1
		f.hasTarget = true
		b.Seed.Init(seed)
		sh := shadow(b)
		sh.Roll(100)
		r := sh.Roll(100)

		Tick(f, b)

		switch {
		case r <= 0x4f && f.last() != "cast0":
			t.Errorf("seed %d: want cast, got %v", seed, f.log)
		case r > 0x4f:
			r2 := sh.Roll(100)
			if r2 > 0x4a && len(f.log) != 1 || r2 <= 0x4a && b.Wake != 10 {
				t.Errorf("seed %d: strafe/wait mismatch log=%v wake=%d", seed, f.log, b.Wake)
			}
		}
	}
}

func TestFBNRaven(t *testing.T) {
	// a peck: A1, one fewer peck, next peck aip3*10 frames away
	f, b := fbnPetNew("Raven", 10, 30, 5, 4, 100, 20)
	f.hasTarget = true
	b.Scratch[0], b.Scratch[1] = 2, -1

	Tick(f, b)

	if f.last() != "attack4" || b.Scratch[0] != 1 || b.Scratch[1] != 40 {
		t.Errorf("raven peck: %v scratch=%v", f.log, b.Scratch)
	}

	// out of pecks: it dies (mode 0)
	f, b = fbnPetNew("Raven", 10, 30, 5, 4, 100, 20)
	b.Scratch[0], b.Scratch[1] = 0, 7

	Tick(f, b)

	if f.last() != "attack0" {
		t.Errorf("raven spent: %v", f.log)
	}

	// a fresh raven gets the default 3 pecks
	f, b = fbnPetNew("Raven", 10, 30, 5, 4, 100, 20)
	f.hasTarget = true
	f.frame = 10

	Tick(f, b)

	if b.Scratch[0] != 2 {
		t.Errorf("fresh raven pecks left = %d", b.Scratch[0])
	}

	// far from the Druid: teleport (above 50)
	f, b = fbnPetNew("Raven", 60, 30, 5, 4, 100, 20)
	b.Scratch[0], b.Scratch[1] = 2, 1

	Tick(f, b)

	if f.teleports != 1 {
		t.Errorf("far raven: %d", f.teleports)
	}
}

func TestFBNVinesAndTotem(t *testing.T) {
	// Vines: in reach and off cooldown it casts Skill1 and stamps the frame
	f, b := fbnPetNew("Vines", 3, 10, 20, 15, 4, 40)
	f.hasTarget, f.attackOK, f.inRange = true, true, true
	b.Scratch[1] = -100
	f.frame = 50

	Tick(f, b)

	if f.last() != "cast0" || b.Scratch[1] != 50 {
		t.Errorf("vines: %v scratch=%v", f.log, b.Scratch)
	}

	// Vines with nothing to do sleep aip3
	f, b = fbnPetNew("Vines", 3, 10, 20, 15, 4, 40)
	f.hasTarget = false

	Tick(f, b)

	if b.Wake != 15 {
		t.Errorf("idle vines wake=%d", b.Wake)
	}

	// Totem: nothing near, the leash roll is not drawn without a target;
	// two rolls happen (forget-target) -> one, then the final wait of 25
	f, b = fbnPetNew("Totem", 3, 0, 0, 30, 20)
	f.hasEnemy = false
	sh := shadow(b)
	sh.Roll(100) // aip2 forget roll
	sh.Roll(100) // mode 2 of SelectPetActionByRange: the 10 percent shuffle roll, then a 15 frame wait

	Tick(f, b)

	if b.Wake != 15 || *b.Seed != *sh {
		t.Errorf("totem: log=%v wake=%d", f.log, b.Wake)
	}
}
