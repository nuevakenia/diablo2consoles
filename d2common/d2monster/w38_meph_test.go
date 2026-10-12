package d2monster

import "testing"

// Mephisto selector 0x5f6950 (w38 re-read): farther than 20 and out of reach selects phase 4 (approach),
// whose body walks near the target and stores phase 0; in reach with aip1 100 the roll can never exceed
// aip1, so the burst phase (2) is chosen and its burst counter (+0x14) is rolled 3..5.
func TestW38MephistoSelector(t *testing.T) {
	b := brainAt(profile("Mephisto", 100))
	w := newFake(30, false)

	if !Tick(w, b) {
		t.Fatal("no think")
	}

	if b.Scratch[2] != mephIdle {
		t.Errorf("approach stores phase 0, got %d", b.Scratch[2])
	}

	if len(w.log) == 0 {
		t.Error("approach queued nothing")
	}

	g := brainAt(profile("Mephisto", 100))
	w = newFake(3, true)
	Tick(w, g)

	if g.Scratch[0] < mephBurstMin-1 || g.Scratch[0] > mephBurstMin+2 {
		t.Errorf("burst counter %d", g.Scratch[0])
	}
}

// With aip1 0 every in-reach roll above 0 picks the melee phase, which stores phase 0.
func TestW38MephistoMelee(t *testing.T) {
	b := brainAt(profile("Mephisto", -1))
	w := newFake(3, true)

	Tick(w, b)

	if b.Scratch[2] != mephIdle {
		t.Errorf("melee phase stores 0, got %d", b.Scratch[2])
	}

	if len(w.log) == 0 {
		t.Error("melee queued nothing")
	}
}
