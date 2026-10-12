package d2skills

import "testing"

func TestUnitRoll100PerUnitStreams(t *testing.T) {
	e := &Engine{}
	e.opt.Seed = 1234

	a1, a2 := e.unitRoll100("a"), e.unitRoll100("a")
	b := e.unitRoll100("b")

	// the same unit shares one generator: two closures advance the same stream
	x, y := a1(), a2()
	for _, v := range []int{x, y, b()} {
		if v < 0 || v > 99 {
			t.Fatalf("roll out of range: %d", v)
		}
	}

	// reproducible from a fresh engine with the same seed
	f := &Engine{}
	f.opt.Seed = 1234

	fa := f.unitRoll100("a")
	if fa() != x || fa() != y {
		t.Fatal("not reproducible")
	}
}
