package d2monreg

import (
	"math"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

func presetRoom(level int, seed uint32) *Room {
	r := &Room{Level: level, X: 500, Y: 500, W: 40, H: 40}
	r.Seed = *d2rand.New(seed)

	return r
}

func TestLinkedClassChain(t *testing.T) {
	tb := realTables(t)

	// tentaclehead1..3 (the stock table: base 261, next 262, 263, none)
	for depth, want := range []int{TentacleHeadClass, TentacleHeadClass + 1, TentacleHeadClass + 2} {
		if got := tb.LinkedClass(TentacleHeadClass, depth); got != want {
			t.Errorf("depth %d: class %d, want %d", depth, got, want)
		}
	}

	if got := tb.LinkedClass(-1, 0); got != -1 {
		t.Errorf("invalid class: %d", got)
	}
}

func TestPresetGroupSize(t *testing.T) {
	tb := realTables(t)
	class := tb.MonByKey("fallen1")
	m := tb.Mons[class]

	counts := map[int]int{}

	for seed := uint32(1); seed <= 300; seed++ {
		g := NewGame(tb, seed, 0, true)
		room := presetRoom(2, seed)
		room.Cells = []Cell{{X0: 100, Y0: 100, X1: 108, Y1: 108, Flag: 1}}

		var pop Population

		if g.PresetGroup(openWorld{}, room, class, 510, 510, &pop) == nil {
			t.Fatalf("seed %d: no group", seed)
		}

		counts[len(pop.Units)]++

		if n := len(pop.Units); n < m.MinGrp || n > m.MaxGrp {
			t.Fatalf("seed %d: %d units, MinGrp %d MaxGrp %d", seed, n, m.MinGrp, m.MaxGrp)
		}
	}

	if len(counts) < 2 && m.MaxGrp > m.MinGrp {
		t.Errorf("the size never varies: %v", counts)
	}
}

func TestPresetGroupNeedsRegionAndSize(t *testing.T) {
	tb := realTables(t)
	class := tb.MonByKey("fallen1")
	g := NewGame(tb, 1, 0, true)
	room := presetRoom(2, 1)

	var pop Population

	if g.PresetGroup(noRegion{}, room, class, 510, 510, &pop) != nil {
		t.Error("a point in no region must create nothing")
	}

	if g.PresetGroup(openWorld{}, room, -1, 510, 510, &pop) != nil || len(pop.Units) != 0 {
		t.Error("invalid class created something")
	}
}

type noRegion struct{ openWorld }

func (noRegion) CellAt(*Room, int, int) int { return 0 }

func TestPresetChampion(t *testing.T) {
	tb := realUModTables(t)
	sizes := map[int]int{}

	for seed := uint32(1); seed <= 300; seed++ {
		g := NewGame(tb, seed, 0, true)
		room := presetRoom(3, seed) // Cold Plains: a plain palette

		var pop Population

		u := g.PresetChampion(openWorld{}, room, 520, 520, &pop)
		if u == nil {
			continue
		}

		if !u.Champion || !u.Unique || len(u.Mods) != 1 || u.Mods[0] != ChampionModID {
			t.Fatalf("seed %d: leader %+v", seed, u)
		}

		if tb.UMods.Rows[ChampionModID].Name != "champion" {
			t.Fatalf("mod %d is %q", ChampionModID, tb.UMods.Rows[ChampionModID].Name)
		}

		minions := len(pop.Units) - 1
		if minions < 1 || minions > 3 {
			t.Fatalf("seed %d: %d champion minions", seed, minions)
		}

		for _, m := range pop.Units[1:] {
			if !m.Champion || !m.Minion || m.Leader != u {
				t.Fatalf("seed %d: minion %+v", seed, m)
			}
		}

		sizes[minions]++
	}

	if len(sizes) != 3 {
		t.Errorf("1..3 champion minions expected over 300 seeds, got %v", sizes)
	}
}

// The preset unique pack never rolls the champion decision: argument 5 of FUN_005a1ec0 is 0.
func TestPresetUniquePackNoChampion(t *testing.T) {
	tb := realUModTables(t)
	made := 0

	for seed := uint32(1); seed <= 300; seed++ {
		g := NewGame(tb, seed, 0, true)
		room := presetRoom(3, seed)

		var pop Population

		if !g.PresetUniquePack(openWorld{}, room, &pop) {
			continue
		}

		made++

		u := pop.Units[0]
		if !u.Unique || u.Champion {
			t.Fatalf("seed %d: %+v", seed, u)
		}

		if len(u.Mods) < 1 {
			t.Fatalf("seed %d: a rare without modifiers", seed)
		}

		if g.Regions[3].Uniques != 1 {
			t.Fatalf("seed %d: region unique counter %d", seed, g.Regions[3].Uniques)
		}
	}

	if made < 250 {
		t.Errorf("only %d of 300 preset unique packs were made", made)
	}
}

// The room-wide position search of the preset unique pack stays inside the room (x, y + 1, size - 1).
func TestPickPosWithoutCell(t *testing.T) {
	tb := realTables(t)
	g := NewGame(tb, 1, 0, true)
	room := presetRoom(3, 1)

	for i := 0; i < 400; i++ {
		x, y, ok := g.pickPos(openWorld{}, room, nil, tb.MonByKey("fallen1"), false)
		if !ok {
			t.Fatal("no spot in an open room")
		}

		if x < room.X+1 || x > room.X+room.W-1 || y < room.Y+1 || y > room.Y+room.H-1 {
			t.Fatalf("spot %d,%d outside the room", x, y)
		}
	}
}

// Lava levels 107 and 108: the estimate "rolls * MonDen / 100000" counts every density hit as a group, but a hit needs
// one of 20 random spots of the room (FUN_0054ba70, the 0x13 limit of its loop) to stand on ground the monster fits on.
// With the walkable share s of a room the share of hits that make a group is 1 - (1-s)^20, so a mostly lava level
// (walkable share around 11 percent, w20 measurement of level 107) keeps about 90 percent of the hits in rooms that have
// ground: the loss comes from the rooms, not from the 20 tries. These are the numbers the notes quote.
func TestLavaPlacementEstimate(t *testing.T) {
	for _, c := range []struct {
		share float64
		want  float64
	}{{0.112, 0.9}, {0.5, 1}, {0.03, 0.46}, {0.01, 0.18}} {
		got := 1 - math.Pow(1-c.share, 20)
		if math.Abs(got-c.want) > 0.02 {
			t.Errorf("share %.3f: %.3f, want about %.2f", c.share, got, c.want)
		}
	}

	// level 107 of the w20 run: 410.5 predicted groups, 393 hits, 104 groups made
	predicted, hits, groups := 51315*800/100000.0, 393, 104
	if math.Abs(float64(hits)/predicted-0.957) > 0.01 {
		t.Errorf("hits/predicted = %.3f", float64(hits)/predicted)
	}

	if float64(groups)/float64(hits) > 0.3 {
		t.Errorf("groups/hits = %.2f", float64(groups)/float64(hits))
	}
}
