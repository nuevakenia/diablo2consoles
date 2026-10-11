package d2monster

import (
	"strings"
	"testing"
)

// capWorld records the target of every cast.
type capWorld struct {
	*fbWorld
	targets []Target
}

func (w *capWorld) Cast(b *Brain, slot int, t Target) bool {
	w.targets = append(w.targets, t)

	return w.fbWorld.Cast(b, slot, t)
}

func capRun(name string, aip []int, dist int, inRange bool, setup func(b *Brain, w *capWorld)) (*Brain, *capWorld) {
	w := &capWorld{fbWorld: &fbWorld{fakeWorld: newFake(dist, inRange), cellFree: true}}
	p := withSkills(profile(name, aip...), 0, 1, 2, 3, 4, 5)
	b := brainAt(p)
	b.Def, _ = Lookup(name)

	if setup != nil {
		setup(b, w)
	}

	Tick(w, b)

	return b, w
}

func TestPriestTeleportsPastTheTarget(t *testing.T) {
	// the fixture target stands at (100+dist, 100); the priest at (100, 100)
	for _, tc := range []struct {
		inRange bool
		wantX   int
	}{{false, 110 + 10}, {true, 110 + 10*4}} {
		_, w := capRun("ZakarumPriest", []int{0, 0, 0, 0, 5, 0}, 10, tc.inRange, func(b *Brain, w *capWorld) {
			b.HPPercent = 20
			w.frame = 1
			b.Aggressive = true
		})

		if len(w.targets) == 0 || w.targets[0].X != tc.wantX || w.targets[0].Y != 100 {
			t.Fatalf("inRange=%v: cast targets %v, want x=%d", tc.inRange, w.targets, tc.wantX)
		}
	}
}

func TestOblivionKnightAllyDistanceAndSlot(t *testing.T) {
	// the ally is farther from the target than the knight is: walk to it
	_, w := capRun("OblivionKnight", []int{20, 30, 0, 0, 0, 0, 0, 0}, 5, false, func(b *Brain, w *capWorld) {
		w.scan = FBXScanResult{Found: true, T: Target{ID: 9, X: 100, Y: 140, Size: 1}}
	})

	if got := strings.Join(w.log, ","); got != "walk-target/4" {
		t.Fatalf("log %q, want a walk to the ally", got)
	}

	// the ally is nearer to the target than the knight: no walk to it
	_, w = capRun("OblivionKnight", []int{20, 30, 0, 0, 0, 0, 0, 0}, 5, false, func(b *Brain, w *capWorld) {
		w.scan = FBXScanResult{Found: true, T: Target{ID: 9, X: 104, Y: 100, Size: 1}}
	})

	if got := strings.Join(w.log, ","); got == "walk-target/4" {
		t.Fatalf("log %q: the ally is closer to the target, no walk to it", got)
	}

	// the sixth slot (index 5) is the cooled-down curse in reach
	_, w = capRun("OblivionKnight", []int{0, 30, 50, 100, 0, 0, 0, 0}, 5, true, func(b *Brain, w *capWorld) { w.frame = 1 })

	if got := strings.Join(w.log, ","); got != "cast5" {
		t.Fatalf("log %q, want cast5", got)
	}
}

func TestNihlathakHelperCastUsesTheSecondScanResult(t *testing.T) {
	_, w := capRun("Nihlathak", []int{0, 0, 0, 100, 0}, 20, false, func(b *Brain, w *capWorld) {
		w.scan = FBXScanResult{Found2: true, T2: Target{ID: 7, X: 120, Y: 100, Size: 1}}
	})

	found := false

	for i, l := range w.log {
		if l == "cast1" && w.targets[i].ID == 7 {
			found = true
		}
	}

	if !found {
		t.Fatalf("log %v targets %v: slot 2 should be cast at the scan's second result", w.log, w.targets)
	}
}

func TestSandMaggotSpitIsSkillThree(t *testing.T) {
	// phase 2 with a spit budget: the third skill (index 2) is cast, phase -> 1
	b, w := capRun("SandMaggot", []int{100, 0, 5, 0, 10}, 8, false, func(b *Brain, w *capWorld) {
		b.Scratch[maggotPhase] = 2
	})

	if got := strings.Join(w.log, ","); got != "cast2" {
		t.Fatalf("log %q, want cast2", got)
	}

	if b.Scratch[maggotPhase] != 1 || b.Scratch[maggotCount] != 1 {
		t.Fatalf("scratch %v", b.Scratch)
	}
}

func TestFrozenHorrorCastGateIsDistanceBelowSkillLevel(t *testing.T) {
	for _, tc := range []struct {
		dist int
		cast bool
	}{{7, true}, {8, false}, {20, false}} {
		_, w := capRun("FrozenHorror", []int{0, 0, 100, 5}, tc.dist, false, func(b *Brain, w *capWorld) {
			b.Profile.Skills[0].Level = 8
		})

		if got := len(w.targets) > 0; got != tc.cast {
			t.Errorf("dist %d: cast=%v log %v", tc.dist, got, w.log)
		}
	}
}

func TestFaithfulBatch8Registered(t *testing.T) {
	if len(FaithfulBatch8()) != 11 {
		t.Fatalf("batch 8 counts %d handlers", len(FaithfulBatch8()))
	}

	for _, list := range [][]string{FaithfulBatch8(), ReverifiedBatch8()} {
		for _, n := range list {
			d, ok := Lookup(n)
			if !ok || !d.Implemented {
				t.Errorf("%s not registered", n)
			}
		}
	}
}

func TestShadowWarriorCapReadsHellColumn(t *testing.T) {
	p := profile("ShadowWarrior")
	p.AIP[8] = 2
	p.AIPRaw[8] = [3]int{2, 2, 200}

	b := NewBrain(1, 1, Normal, p, testSeed)

	if got := shadowWord(b, 8, 2); got != 200 {
		t.Fatalf("hell column %d, want 200", got)
	}
}
