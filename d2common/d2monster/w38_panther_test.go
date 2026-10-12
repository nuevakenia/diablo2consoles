package d2monster

import "testing"

type sameClassWorld struct {
	*fakeWorld
	ally Target
	sq   int
	ok   bool
}

func (s *sameClassWorld) NearestSameClass(*Brain) (Target, int, bool) { return s.ally, s.sq, s.ok }

func TestW38PantherJavelinRegroupUsesRawAip3(t *testing.T) {
	// aip1 approach%, aip2 throw%, aip3 group distance, aip4 walk-away%, aip5 stall, aip6 throw distance
	b := brainAt(profile("PantherJavelin", 0, 0, 10, 0, 7, 3))
	w := &sameClassWorld{fakeWorld: newFake(5, false), ally: Target{ID: 9, X: 130, Y: 100, Size: 1}, sq: 11, ok: true}
	w.attackOK = false

	Tick(w, b)

	// aip3 (10) < squared distance (11): walk to the ally; not squared like the PantherWoman
	if w.last() != "walk-target/7" {
		t.Errorf("regroup: %v", w.log)
	}

	b = brainAt(profile("PantherJavelin", 0, 0, 10, 0, 7, 3))
	w = &sameClassWorld{fakeWorld: newFake(5, false), ally: Target{ID: 9, X: 110, Y: 100, Size: 1}, sq: 10, ok: true}
	w.attackOK = false

	Tick(w, b)

	if len(w.log) != 0 || b.Wake != 7 {
		t.Errorf("no regroup at sq == aip3: %v wake %d", w.log, b.Wake)
	}
}

func TestW38PantherJavelinThrowAndRetreat(t *testing.T) {
	// close target and aip4 100: walk away, nothing else
	w, _ := runThink(t, "PantherJavelin", true, 4, 0, 100, 10, 100, 7, 20)
	if len(w.log) != 1 {
		t.Errorf("retreat: %v", w.log)
	}

	// within the throw distance and aip2 100: throw
	if w, _ := runThink(t, "PantherJavelin", false, 12, 0, 100, 10, 0, 7, 20); w.last() != "attack4" {
		t.Errorf("throw: %v", w.log)
	}

	// within the throw distance, aip2 0: stall aip5
	if _, b := runThink(t, "PantherJavelin", false, 12, 0, 0, 10, 0, 7, 20); b.Wake != 7 {
		t.Errorf("stall wake %d", b.Wake)
	}
}

func TestW38PantherWomanRegroupSquaresAip3(t *testing.T) {
	b := brainAt(profile("PantherWoman", 0, 0, 5, 7))
	w := &sameClassWorld{fakeWorld: newFake(12, false), ally: Target{ID: 9, X: 130, Y: 100, Size: 1}, sq: 26, ok: true}

	Tick(w, b)

	if w.last() != "walk-target/7" {
		t.Errorf("regroup: %v", w.log)
	}
}

func TestW38Griswold(t *testing.T) {
	// out of range: 50% walk (roll below 50), else wait 10; in range: 80% A1
	b := brainAt(profile("Griswold"))
	exp := shadow(b)
	w := newFake(12, false)

	Tick(w, b)

	if exp.Roll(100) < 50 {
		if w.last() != "walk-target/7" {
			t.Errorf("walk: %v", w.log)
		}
	} else if b.Wake != 10 {
		t.Errorf("wait: %d", b.Wake)
	}

	b = brainAt(profile("Griswold"))
	exp = shadow(b)
	w = newFake(2, true)

	Tick(w, b)

	if exp.Roll(100) < 80 {
		if w.last() != "attack4" {
			t.Errorf("attack: %v", w.log)
		}
	} else if b.Wake != 10 {
		t.Errorf("wait in range: %d", b.Wake)
	}
}
