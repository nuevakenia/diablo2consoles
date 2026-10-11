package d2cube

import (
	"testing"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/d2drop"
)

// Rules read from Game.exe 1.14b (CUBE_ServerTransmute 0x5642d0,
// CUBE_CheckRecipeInputSlot 0x562e40, CUBE_ServerCreateRecipeOutputs 0x5638a0,
// the CubeMain expression parsers 0x66b210 / 0x66b670), on synthetic tables.

func TestExeUpgInputAcceptsHigherTier(t *testing.T) {
	cat, _, _ := synth()
	in := Input{Code: "ssd", Quality: d2drop.QualityMagic, Upgrade: true}

	for _, code := range []string{"ssd", "crs", "bsd"} {
		it := Item{Code: code, Quality: d2drop.QualityMagic}
		if !cat.Matches(&in, &it) {
			t.Errorf("upg ssd vs %s", code)
		}
	}

	// an exceptional input does not take the basic item
	in2 := Input{Code: "crs", Upgrade: true}
	if cat.Matches(&in2, &Item{Code: "ssd"}) {
		t.Error("exceptional input accepted the basic item")
	}

	// without upg the code must be equal
	in3 := Input{Code: "ssd"}
	if cat.Matches(&in3, &Item{Code: "crs"}) {
		t.Error("code input without upg accepted a higher tier")
	}
}

func TestExeNoRunewordInput(t *testing.T) {
	cat, _, _ := synth()
	in, err := parseInput("ssd,nru", cat)

	if err != nil || !in.NoRuneword {
		t.Fatalf("%+v %v", in, err)
	}

	if cat.Matches(&in, &Item{Code: "ssd", Runeword: "Steel"}) || !cat.Matches(&in, &Item{Code: "ssd"}) {
		t.Error("nru")
	}
}

func TestExeTypeBeforeCode(t *testing.T) {
	cat, _, _ := synth()

	in, err := parseInput("sock", cat)
	if err != nil || in.Type != "sock" || in.Code != "" {
		t.Fatalf("%+v %v", in, err)
	}
}

func TestExeRecipeConditions(t *testing.T) {
	// 2024-05-15 was a Wednesday (weekday 4 with Sunday = 1)
	now := time.Date(2024, 5, 15, 12, 0, 0, 0, time.UTC)
	ctx := &Context{Now: now, Stat: func(k StatKind, id int) (int, uint, bool) {
		if id == 99 {
			return 0, 0, false
		}

		return map[StatKind]int{StatTotal: 50, StatBase: 40, StatBonus: 10}[k], 0, true
	}}

	cases := []struct {
		op, param, value int
		want             bool
	}{
		{1, 10, 20, true}, {1, 16, 20, false}, {1, 1, 14, false},
		{2, 0, 4, true}, {2, 0, 5, false},
		{3, 1, 50, true}, {3, 1, 51, false},
		{4, 1, 50, true}, {4, 1, 49, false},
		{5, 1, 50, false}, {5, 1, 49, true},
		{6, 1, 50, true}, {6, 1, 49, false},
		{7, 1, 40, true}, {10, 1, 39, false},
		{11, 1, 10, true}, {14, 1, 10, true}, {14, 1, 11, false},
		{3, 99, 0, false},
		{28, 0, 0, true},
	}

	for _, c := range cases {
		r := &Recipe{Op: c.op, Param: c.param, Value: c.value}
		if got := ctx.condition(r); got != c.want {
			t.Errorf("op %d param %d value %d: %v", c.op, c.param, c.value, got)
		}
	}
}

func TestExeItemConditionOnFirstInputOnly(t *testing.T) {
	cat, _, _ := synth()
	r := &Recipe{Op: 15, Param: 7, Value: 3}
	in := &Input{Code: "ssd"}
	good := Item{Code: "ssd", Stat: func(StatKind, int) (int, uint, bool) { return 3, 0, true }}
	bad := Item{Code: "ssd", Stat: func(StatKind, int) (int, uint, bool) { return 2, 0, true }}
	ctx := &Context{}

	if !cat.matchInput(ctx, r, 0, in, &good) || cat.matchInput(ctx, r, 0, in, &bad) {
		t.Error("op 15 on input 0")
	}

	if !cat.matchInput(ctx, r, 1, in, &bad) {
		t.Error("item conditions apply to the first input only")
	}

	r27 := &Recipe{Op: 27, Value: 12}
	if cat.matchInput(ctx, r27, 0, in, &Item{Code: "ssd", FileIndex: 12}) ||
		!cat.matchInput(ctx, r27, 0, in, &Item{Code: "ssd", FileIndex: 13}) {
		t.Error("op 27")
	}
}

func TestExeOutputLevelRules(t *testing.T) {
	cases := []struct {
		o                   Output
		player, first, want int
	}{
		{Output{Level: 30, PLevel: 100, ILevel: 100}, 80, 70, 30},
		{Output{PLevel: 50, ILevel: 50}, 81, 41, 60},
		{Output{}, 80, 70, 1},
		{Output{PLevel: 100, ILevel: 100}, 90, 90, 99},
	}
	for i, c := range cases {
		if got := OutputLevel(&c.o, c.player, c.first); got != c.want {
			t.Errorf("case %d: %d want %d", i, got, c.want)
		}
	}
}

func TestExeMainItemIsFirstInput(t *testing.T) {
	cat, tab, _ := synth()
	// recipe "sock": input 1 swor,mag, input 2 gfv; the main item is input 1
	// even when the gem comes first in the cube.
	items := []Item{{Code: "gfv"}, {Code: "crs", Quality: d2drop.QualityMagic, ILvl: 30}}

	m := cat.Find(tab, &Context{PlayerLevel: 50, Expansion: true}, items)
	if m == nil {
		t.Fatal("no match")
	}

	if got := cat.MainItem(m, items); got != 1 {
		t.Errorf("main %d", got)
	}
}

func TestExeModChance(t *testing.T) {
	// chance c applies when roll(100) <= c; 0 means always
	cat, _, _ := synth()
	o := &Output{Mods: []Mod{{Code: "x", Chance: 10, Min: 1, Max: 1}}}
	n := 0

	rng := d2rand.New(3)
	for i := 0; i < 20000; i++ {
		var it Item

		cat.applyMods(o, &it, rng)
		n += len(it.Mods)
	}

	if f := float64(n) / 20000; f < 0.09 || f > 0.13 {
		t.Errorf("rate %.3f, want about 0.11", f)
	}

	var it Item

	cat.applyMods(&Output{Mods: []Mod{{Code: "x", Chance: 0, Min: 1, Max: 1}}}, &it, rng)

	if len(it.Mods) != 1 {
		t.Error("chance 0 must always apply")
	}
}

func TestExeRecipeSockets(t *testing.T) {
	cases := []struct {
		q    d2drop.Quality
		n    int
		want int
	}{
		{d2drop.QualityMagic, 3, 3}, {d2drop.QualityMagic, 4, 3}, {d2drop.QualityMagic, 6, 3},
		{QualityTempered, 5, 3}, {d2drop.QualityRare, 2, 1}, {d2drop.QualityUnique, 6, 1},
		{d2drop.QualityCrafted, 1, 1}, {d2drop.QualityNormal, 6, 6}, {d2drop.QualitySet, 0, 0},
	}
	for _, c := range cases {
		if got := RecipeSockets(c.q, c.n); got != c.want {
			t.Errorf("quality %d n %d: %d want %d", c.q, c.n, got, c.want)
		}
	}
}

func TestExeRemReturnsGems(t *testing.T) {
	cat, _, _ := synth()
	src := Item{Code: "ssd", Sockets: 2, Socketed: []string{"gcv", "r01"}, Quality: d2drop.QualityNormal}
	items := []Item{src}
	m := &Match{Recipe: &Recipe{Outputs: []Output{{UseItem: true, Remove: true}}}, Assigned: [][]int{{0}}}

	res, err := cat.Execute(m, &Context{PlayerLevel: 10}, items, d2rand.New(1))
	if err != nil {
		t.Fatal(err)
	}

	p := res.Products[0]
	if len(p.Returned) != 2 || p.Item.Socketed != nil {
		t.Errorf("%+v", p)
	}
}
