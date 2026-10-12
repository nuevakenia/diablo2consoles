package d2lightmap

import "testing"

func TestUnitSeedRoll(t *testing.T) {
	s := UnitSeed{Lo: 1, Hi: 0}

	// seed 1: 1*0x6ac690c5 + 0 -> low half 0x6ac690c5; power of two masks (0xc5 & 3 = 1)
	if got := s.Roll(4); got != 1 || s.Lo != 0x6ac690c5 || s.Hi != 0 {
		t.Fatalf("roll 4: got %d seed %+v", got, s)
	}

	// next: 0x6ac690c5^2 + 0 = low 32 bits, high 32 bits go to Hi; modulo 3 for a non power of two
	v := uint64(0x6ac690c5) * 0x6ac690c5
	if got := s.Roll(3); got != int(uint32(v)%3) || s.Hi != uint32(v>>32) {
		t.Fatalf("roll 3: got %d seed %+v", got, s)
	}

	before := s
	if s.Roll(0) != 0 || s.Roll(-2) != 0 || s != before {
		t.Fatal("n <= 0 must give 0 without advancing")
	}
}

func TestFlickerLightGolden(t *testing.T) {
	// Light 3, Flicker 4 (power of two): radius starts at 3 subtiles = 24 units and then stays in
	// [24, 48) units, moving by at most 8 per tick.
	f := FlickerLight{Seed: UnitSeed{Lo: 12345, Hi: 678}}

	var trace []int

	for i := 0; i < 12; i++ {
		f.Tick(0, 3, 4)
		trace = append(trace, f.Cur)
	}

	prev := 24

	for i, c := range trace {
		if c < 24 || c > 48 {
			t.Fatalf("tick %d: radius %d out of range", i, c)
		}

		if d := c - prev; d > RampPerTick || d < -RampPerTick {
			t.Fatalf("tick %d: step %d", i, d)
		}

		prev = c
	}

	// the same seed gives the same trace; a different seed differs somewhere
	g := FlickerLight{Seed: UnitSeed{Lo: 12345, Hi: 678}}
	h := FlickerLight{Seed: UnitSeed{Lo: 999, Hi: 1}}
	differs := false

	for i := 0; i < 12; i++ {
		g.Tick(0, 3, 4)
		h.Tick(0, 3, 4)

		if g.Cur != trace[i] {
			t.Fatalf("not deterministic at tick %d", i)
		}

		differs = differs || h.Cur != g.Cur
	}

	if !differs {
		t.Error("two seeds flicker in step")
	}

	if trace[0] != 24 && trace[0] != 32 {
		t.Errorf("first tick %d", trace[0])
	}
}

func TestFlickerLightNoFlicker(t *testing.T) {
	f := FlickerLight{}
	for i := 0; i < 5; i++ {
		f.Tick(0, 5, 0) // Flicker 0: constant at Light
	}

	if f.Cur != 40 || f.Subtiles() != 5 {
		t.Errorf("got %d / %d", f.Cur, f.Subtiles())
	}

	g := FlickerLight{}
	for i := 0; i < 5; i++ {
		g.Tick(0x100, 5, 8) // flag stops the flicker
	}

	if g.Cur != 40 {
		t.Errorf("flagged missile moved: %d", g.Cur)
	}

	z := FlickerLight{}
	z.Tick(0, 0, 3) // Light 0: no light at all

	if z.Subtiles() != 0 {
		t.Errorf("light 0 made a radius: %d", z.Subtiles())
	}
}
