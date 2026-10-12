package d2monster

import "testing"

// Second batch re-read in wave 38: PantherWoman 0x5f1300, SandLeaper 0x5f1100, Mummy 0x5f1870,
// DoomKnight 0x5f9c20, SkeletonBow 0x5f5180, CorruptArcher 0x5f4b10, SandRaider 0x5ef800, Idle 0x5ae8a0.

func TestW38PantherWoman(t *testing.T) {
	// aip1 approach%, aip2 attack%, aip3 regroup distance, aip4 stall
	if w, _ := runThink(t, "PantherWoman", true, 2, 0, 100, 0, 7); w.last() != "attack4" {
		t.Errorf("attack: %v", w.log)
	}

	if w, b := runThink(t, "PantherWoman", true, 2, 0, 0, 0, 7); len(w.log) != 0 || b.Wake != 7 {
		t.Errorf("stall: %v %d", w.log, b.Wake)
	}

	if w, _ := runThink(t, "PantherWoman", false, 12, 100, 0, 0, 7); w.last() != "walk-target/7" {
		t.Errorf("approach: %v", w.log)
	}
}

func TestW38SandLeaper(t *testing.T) {
	// aip1 leap%, aip2 bite%, aip3 approach%, aip4 circle%
	if w, _ := runThink(t, "SandLeaper", true, 2, 0, 100, 0, 0); w.last() != "attack5" {
		t.Errorf("bite: %v", w.log)
	}

	if w, b := runThink(t, "SandLeaper", true, 2, 0, 0, 0, 0); len(w.log) != 0 || b.Wake != 10 {
		t.Errorf("stall: %v %d", w.log, b.Wake)
	}

	if w, _ := runThink(t, "SandLeaper", false, 8, 0, 0, 100, 0); w.last() != "walk-target/7" {
		t.Errorf("approach: %v", w.log)
	}

	// the leap needs Skill1 and a distance below 5
	p := profile("SandLeaper", 100, 0, 0, 0)
	p.Skills[0] = SkillSlot{Name: "leap"}
	b := brainAt(p)
	w := newFake(4, false)
	Tick(w, b)

	if w.last() != "cast0" {
		t.Errorf("leap: %v", w.log)
	}
}

func TestW38Mummy(t *testing.T) {
	// aip1 awake distance, aip2 wander%, aip3 attack%, aip4 A1%, aip5 stall
	if w, _ := runThink(t, "Mummy", true, 2, 10, 0, 100, 100, 5); w.last() != "attack4" {
		t.Errorf("A1: %v", w.log)
	}

	if w, _ := runThink(t, "Mummy", true, 2, 10, 0, 100, 0, 5); w.last() != "attack5" {
		t.Errorf("A2: %v", w.log)
	}

	if w, b := runThink(t, "Mummy", false, 20, 10, 0, 0, 0, 5); len(w.log) != 0 || b.Wake != 5 {
		t.Errorf("sleep: %v %d", w.log, b.Wake)
	}

	if w, _ := runThink(t, "Mummy", false, 8, 10, 0, 0, 0, 5); w.last() != "walk-target/7" {
		t.Errorf("walk: %v", w.log)
	}
}

func TestW38DoomKnight(t *testing.T) {
	// aip1 attack%, aip2 stall in range, aip3 approach%, aip4 stall out of range
	if w, _ := runThink(t, "DoomKnight", true, 2, 100, 6, 0, 8); w.last() != "attack4" {
		t.Errorf("attack: %v", w.log)
	}

	if _, b := runThink(t, "DoomKnight", true, 2, 0, 6, 0, 8); b.Wake != 6 {
		t.Errorf("in-range stall wake %d", b.Wake)
	}

	if _, b := runThink(t, "DoomKnight", false, 12, 0, 6, 0, 8); b.Wake != 8 {
		t.Errorf("out-of-range stall wake %d", b.Wake)
	}

	if w, _ := runThink(t, "DoomKnight", false, 12, 0, 6, 100, 8); w.last() != "walk-target/7" {
		t.Errorf("approach: %v", w.log)
	}
}

func TestW38SkeletonBow(t *testing.T) {
	// aip1 shoot%, aip2 stall, aip3 approach%, aip4 step, aip5 desired
	if w, _ := runThink(t, "SkeletonBow", false, 10, 100, 6, 0, 8, 9); w.last() != "attack4" {
		t.Errorf("shoot: %v", w.log)
	}

	if w, b := runThink(t, "SkeletonBow", false, 10, 0, 6, 0, 8, 9); b.Wake != 6 {
		t.Errorf("stall wake %d %v", b.Wake, w.log)
	}

	b := brainAt(profile("SkeletonBow", 100, 6, 0, 8, 9))
	w := newFake(30, false)
	Tick(w, b)

	if b.Wake != 20 {
		t.Errorf("far stall wake %d %v", b.Wake, w.log)
	}

	g := brainAt(profile("SkeletonBow", 0, 6, 0, 8, 9))
	g.Aggressive = true
	w = newFake(10, false)
	Tick(w, g)

	if w.last() != "attack4" {
		t.Errorf("aggressive: %v", w.log)
	}
}

func TestW38CorruptArcher(t *testing.T) {
	// aip1 approach, aip2 shoot%, aip3 stall, aip4 run%, aip5 run dist, aip6/7 skill%, aip8 walk dist
	if w, _ := runThink(t, "CorruptArcher", false, 8, 0, 100, 5, 0, 20, 0, 0, 0); w.last() != "attack4" {
		t.Errorf("A1 without skills: %v", w.log)
	}

	if _, b := runThink(t, "CorruptArcher", false, 8, 0, 0, 5, 0, 20, 0, 0, 0); b.Wake != 5 {
		t.Errorf("stall wake %d", b.Wake)
	}

	if w, _ := runThink(t, "CorruptArcher", false, 30, 0, 100, 5, 0, 20, 0, 0, 0); w.last() != "run-target/20" {
		t.Errorf("run: %v", w.log)
	}

	p := profile("CorruptArcher", 0, 100, 5, 0, 20, 100, 0, 0)
	p.Skills[1] = SkillSlot{Name: "s2"}
	b := brainAt(p)
	w := newFake(8, false)
	Tick(w, b)

	if w.last() != "cast1" {
		t.Errorf("skill2: %v", w.log)
	}
}

func TestW38SandRaiderCharge(t *testing.T) {
	// aip1 hurt%, aip2 strafe%, aip3 melee%, aip4 approach%, aip5 charge ticks, aip6, aip7 A2%
	b := brainAt(profile("SandRaider", 0, 0, 100, 0, 3, 0, 0))
	w := newFake(2, true)

	for i := 0; i < 2; i++ {
		b.Wake = 0
		Tick(w, b)
	}

	if b.Scratch[raiderCount] != 2 || (w.last() != "attack4" && w.last() != "attack5") {
		t.Errorf("count %d log %v", b.Scratch[raiderCount], w.log)
	}

	b.Wake = 0
	n := len(w.log)
	Tick(w, b)

	if len(w.log) != n || b.Scratch[raiderCount] != 3 {
		t.Errorf("charge tick: %v count %d", w.log, b.Scratch[raiderCount])
	}

	b.Wake = 0
	Tick(w, b)

	if b.Scratch[raiderCharged] != 1 {
		t.Error("charged flag not set past aip5")
	}
}

func TestW38Idle(t *testing.T) {
	b := brainAt(profile("Idle"))
	Tick(newFake(5, true), b)

	if b.Wake != 200 {
		t.Errorf("idle wake %d", b.Wake)
	}
}
