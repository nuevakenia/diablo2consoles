package d2monster

import "testing"

// MONAI_State16_Pre 0x5e1bc0: slot 2 or 4, zero when the unit holds neither skill.
func TestW37State16Pre(t *testing.T) {
	p := &Profile{}
	b := NewBrain(1, 0, 0, p, 7)
	c := &Ctx{B: b, W: newFake(5, true)}

	state16Pre(c)

	if b.Scratch[1] != 0 {
		t.Errorf("no skills: scratch1 = %d", b.Scratch[1])
	}

	p.Skills[2].Name = "a"
	p.Skills[4].Name = "b"
	seen := map[int]bool{}

	for i := 0; i < 40; i++ {
		state16Pre(c)
		seen[b.Scratch[1]] = true
	}

	if !seen[2] || !seen[4] || len(seen) != 2 {
		t.Errorf("both slots must come up: %v", seen)
	}

	p.Skills[2].Name = ""

	for i := 0; i < 10; i++ {
		state16Pre(c)

		if b.Scratch[1] != 4 {
			t.Fatalf("only slot 4 held: got %d", b.Scratch[1])
		}
	}
}

// The VileMother ring (0x5f8f90) only has axis steps.
func TestW37VileRingAxisOnly(t *testing.T) {
	for i, d := range fbxRing {
		if d.X*d.Y != 0 {
			t.Errorf("entry %d is diagonal: %v", i, d)
		}
	}

	if fbxRing[0] != (Point{1, 0}) || fbxRing[1] != (Point{0, 1}) || fbxRing[5] != (Point{0, -1}) || fbxRing[2] != (Point{-1, 0}) {
		t.Error("ring table differs from {6,4,2,2,2,0,6,6}")
	}
}
