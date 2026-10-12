package d2monster

import "testing"

// Wave 38: the classic archetypes were re-read against Game.exe (0x5eee10 Skeleton, 0x5eef40 Zombie,
// 0x5f0360 Goatman, 0x5efb30 Wraith, 0x5f14b0 Swarm, 0x5f0200 QuillRat). These tests pin the flow
// read from the exe with always/never percentages, so no roll prediction is needed.

func runThink(t *testing.T, ai string, inRange bool, dist int, aip ...int) (*fakeWorld, *Brain) {
	t.Helper()

	b := brainAt(profile(ai, aip...))
	w := newFake(dist, inRange)

	if !Tick(w, b) {
		t.Fatalf("%s: think did not run", ai)
	}

	return w, b
}

func TestW38SkeletonGoatmanSwarm(t *testing.T) {
	for _, ai := range []string{"Skeleton", "Goatman", "Swarm"} {
		// out of range, aip1 100: walk to melee reach 7
		if w, _ := runThink(t, ai, false, 12, 100, 9, 0, 0); w.last() != "walk-target/7" {
			t.Errorf("%s approach: %v", ai, w.log)
		}

		// out of range, aip1 0: wait aip2
		if w, b := runThink(t, ai, false, 12, 0, 9, 0, 0); len(w.log) != 0 || b.Wake != 9 {
			t.Errorf("%s stall: %v wake %d", ai, w.log, b.Wake)
		}

		// in range, aip3 0: wait aip2
		if w, b := runThink(t, ai, true, 2, 100, 9, 0, 100); len(w.log) != 0 || b.Wake != 9 {
			t.Errorf("%s in-range stall: %v wake %d", ai, w.log, b.Wake)
		}
	}

	// in range, aip3 100: Skeleton picks A1 with aip4 100 and A2 with aip4 0; Goatman/Swarm always A1
	if w, _ := runThink(t, "Skeleton", true, 2, 0, 9, 100, 100); w.last() != "attack4" {
		t.Errorf("Skeleton A1: %v", w.log)
	}

	if w, _ := runThink(t, "Skeleton", true, 2, 0, 9, 100, 0); w.last() != "attack5" {
		t.Errorf("Skeleton A2: %v", w.log)
	}

	for _, ai := range []string{"Goatman", "Swarm"} {
		if w, _ := runThink(t, ai, true, 2, 0, 9, 100, 0); w.last() != "attack4" {
			t.Errorf("%s A1: %v", ai, w.log)
		}
	}
}

func TestW38Wraith(t *testing.T) {
	if w, _ := runThink(t, "Wraith", true, 2, 0, 9, 100); w.last() != "attack4" {
		t.Errorf("wraith attack: %v", w.log)
	}

	if w, b := runThink(t, "Wraith", true, 2, 0, 9, 0); len(w.log) != 0 || b.Wake != 9 {
		t.Errorf("wraith in-range stall: %v wake %d", w.log, b.Wake)
	}

	if w, b := runThink(t, "Wraith", false, 12, 0, 9, 0); len(w.log) != 0 || b.Wake != 9 {
		t.Errorf("wraith out-of-range stall: %v wake %d", w.log, b.Wake)
	}
}

func TestW38Zombie(t *testing.T) {
	// in range: no aip3 gate, aip4 chooses A1 or A2
	if w, _ := runThink(t, "Zombie", true, 2, 0, 0, 0, 100); w.last() != "attack4" {
		t.Errorf("zombie A1: %v", w.log)
	}

	if w, _ := runThink(t, "Zombie", true, 2, 0, 0, 0, 0); w.last() != "attack5" {
		t.Errorf("zombie A2: %v", w.log)
	}

	// out of range, distance >= aip2: the aip1 roll is skipped and the zombie wanders instead of chasing
	b := brainAt(profile("Zombie", 100, 5, 0, 0))
	w := newFake(12, false)

	Tick(w, b)

	if w.last() == "run-target/0" {
		t.Error("zombie chased although the distance is not below aip2")
	}

	// Graveyard (level 17): always runs at the target
	g := brainAt(profile("Zombie", 0, 5, 0, 0))
	g.LevelID = 17
	w = newFake(12, false)
	Tick(w, g)

	if w.last() != "run-target/0" {
		t.Errorf("graveyard zombie: %v", w.log)
	}
}

func TestW38QuillRat(t *testing.T) {
	// in range: A1
	if w, _ := runThink(t, "QuillRat", true, 2, 10, 0, 0, 5); w.last() != "attack4" {
		t.Errorf("quill rat melee: %v", w.log)
	}

	// aggressive and out of range: spike A2
	b := brainAt(profile("QuillRat", 10, 0, 0, 5))
	b.Aggressive = true
	w := newFake(12, false)
	Tick(w, b)

	if w.last() != "attack5" {
		t.Errorf("aggressive quill rat: %v", w.log)
	}

	// inside the kite distance with aip2 100: spike
	if w, _ := runThink(t, "QuillRat", false, 6, 10, 100, 0, 5); w.last() != "attack5" {
		t.Errorf("kite spike: %v", w.log)
	}
}
