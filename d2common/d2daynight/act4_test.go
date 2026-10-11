package d2daynight

import "testing"

func TestAct4Target(t *testing.T) {
	for lvl, want := range map[int]int{0x67: 0x80, 0x68: 0x40, 0x69: 0x38, 0x6a: 0x30, 0x66: 0x10, 0x6b: 0x10, 1: 0x10} {
		if got := Act4Target(lvl); got != want {
			t.Errorf("level %#x: %#x want %#x", lvl, got, want)
		}
	}
}

func TestAct4StepEases(t *testing.T) {
	// from black it climbs one per tick (the "zero means target" check never fires for a target above 0)
	if got := Act4Step(0, 0x67, 1); got != 1 {
		t.Errorf("from zero: %#x", got)
	}

	if got := Act4Step(0x80, 0x68, 10); got != 0x80-10 {
		t.Errorf("down: %#x", got)
	}

	if got := Act4Step(0x10, 0x67, 5); got != 0x15 {
		t.Errorf("up: %#x", got)
	}

	if got := Act4Step(0x41, 0x68, 50); got != 0x40 {
		t.Errorf("settle: %#x", got)
	}

	if got := Act4Step(0x30, 0x6a, 0); got != 0x30 {
		t.Errorf("no ticks: %#x", got)
	}
}
