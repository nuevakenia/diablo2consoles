package d2inventory

import "testing"

func TestCanStack(t *testing.T) {
	base := StackProbe{Code: "jav", Quality: 2, Stackable: true}

	cases := []struct {
		name string
		mod  func(p *StackProbe)
		want bool
	}{
		{"same", func(p *StackProbe) {}, true},
		{"other base", func(p *StackProbe) { p.Code = "tkf" }, false},
		{"other quality", func(p *StackProbe) { p.Quality = 3 }, false},
		{"ethereal", func(p *StackProbe) { p.Ethereal = true }, false},
		{"socketed", func(p *StackProbe) { p.Sockets = 1 }, false},
		{"other damage", func(p *StackProbe) { p.Damage[0] = 2 }, false},
		{"other file index", func(p *StackProbe) { p.FileIndex = 4 }, false},
	}

	for _, c := range cases {
		o := base
		c.mod(&o)

		if got := CanStack(base, o); got != c.want {
			t.Errorf("%s: %v want %v", c.name, got, c.want)
		}
	}

	magic := StackProbe{Code: "jav", Quality: 4, Stackable: true}
	if CanStack(magic, magic) {
		t.Error("magic items do not stack")
	}

	if CanStack(StackProbe{Code: "x", Quality: 2}, StackProbe{Code: "x", Quality: 2}) {
		t.Error("a base that is not stackable does not stack")
	}
}

func TestNonMagicalQuality(t *testing.T) {
	for q, want := range map[int]bool{0: false, 1: true, 2: true, 3: true, 4: false, 7: false, 9: false, 10: true} {
		if NonMagicalQuality(q) != want {
			t.Errorf("quality %d", q)
		}
	}
}

func TestHeldDrop(t *testing.T) {
	cube := HeldUnder{IsCube: true}
	plain := HeldUnder{}

	cases := []struct {
		name string
		in   HeldInput
		want HeldAction
	}{
		{"empty cells", HeldInput{}, HeldPlace},
		{"cube page ignores a cube", HeldInput{Page: 3, HeldIsCube: true}, HeldNothing},
		{"one item swaps", HeldInput{Under: []HeldUnder{plain}}, HeldSwap},
		{"one item on the trade page", HeldInput{Page: 2, Under: []HeldUnder{plain}}, HeldNothing},
		{"stack", HeldInput{Under: []HeldUnder{{Stack: true}}}, HeldStack},
		{"stack wins over cube", HeldInput{Under: []HeldUnder{{Stack: true, IsCube: true}}}, HeldStack},
		{"socket", HeldInput{Under: []HeldUnder{{Socket: true}}}, HeldSocket},
		{"scroll onto tome", HeldInput{Under: []HeldUnder{{Tome: true}}}, HeldScrollToTome},
		{"full tome", HeldInput{Under: []HeldUnder{{Tome: true, TomeFull: true}}}, HeldNothing},
		{"into the cube", HeldInput{CubeFits: true, Under: []HeldUnder{cube}}, HeldIntoCube},
		{"cube without room", HeldInput{Under: []HeldUnder{cube}}, HeldImpossible},
		{"two items refused", HeldInput{Under: []HeldUnder{plain, plain}}, HeldPlace},
		{"two items with a cube go into it", HeldInput{CubeFits: true, Under: []HeldUnder{plain, cube}}, HeldIntoCube},
		{"two items with a full cube", HeldInput{Under: []HeldUnder{cube, plain}}, HeldImpossible},
	}

	for _, c := range cases {
		if got := HeldDrop(c.in); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}
