package d2trade

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Wiki cross-check (wave 30) of the vendor price formulas, with the real
// npc.txt / base item tables from D2_TABLES (skipped when unset). The wiki
// numbers are the author's recall of the community pages (the sites were not
// reachable), so they are WIKI-RECALL; the engine side is the exe-verified
// ItemPrice / GamblePrice.

type w30Table struct {
	col  map[string]int
	rows [][]string
}

func w30Read(t *testing.T, rel ...string) *w30Table {
	t.Helper()

	root := os.Getenv("D2_TABLES")
	if root == "" {
		t.Skip("D2_TABLES not set")
	}

	raw, err := os.ReadFile(filepath.Join(append([]string{root}, rel...)...))
	if err != nil {
		t.Fatal(err)
	}

	tab := &w30Table{col: map[string]int{}}

	for i, l := range strings.Split(string(raw), "\n") {
		f := strings.Split(strings.TrimRight(l, "\r"), "\t")
		if i == 0 {
			for k, h := range f {
				if _, dup := tab.col[h]; !dup {
					tab.col[h] = k
				}
			}

			continue
		}

		if len(f) > 1 {
			tab.rows = append(tab.rows, f)
		}
	}

	return tab
}

func (w *w30Table) get(r []string, c string) string {
	if i, ok := w.col[c]; ok && i < len(r) {
		return r[i]
	}

	return ""
}

func (w *w30Table) n(r []string, c string) int {
	v, _ := strconv.Atoi(w.get(r, c))

	return v
}

func (w *w30Table) find(key, val string) []string {
	for _, r := range w.rows {
		if w.get(r, key) == val {
			return r
		}
	}

	return nil
}

func w30NPC(t *testing.T, name string) NPC {
	tab := w30Read(t, "d2exp", "npc.txt")

	r := tab.find("npc", name)
	if r == nil {
		t.Fatalf("npc %s missing", name)
	}

	// The loader keeps "sell mult" (what the player pays) first.
	return NPC{PlayerPays: tab.n(r, "sell mult"), VendorPays: tab.n(r, "buy mult"), Repair: tab.n(r, "rep mult")}
}

func TestW30WikiVendorPrices(t *testing.T) {
	gheed, charsi, akara := w30NPC(t, "gheed"), w30NPC(t, "charsi"), w30NPC(t, "akara")

	// Wiki: Gheed charges 6.25% more, Charsi 6.25% less, others the base price.
	if gheed.PlayerPays != 1088 || charsi.PlayerPays != 960 || akara.PlayerPays != 1024 {
		t.Errorf("npc multipliers gheed %d charsi %d akara %d", gheed.PlayerPays, charsi.PlayerPays, akara.PlayerPays)
	}

	item := Item{BaseCost: 1000, Identified: true, Quality: QualityNormal}

	for _, k := range []struct {
		name string
		npc  NPC
		want int
	}{{"gheed", gheed, 1062}, {"charsi", charsi, 937}, {"akara", akara, 1000}} {
		if got := ItemPrice(&item, Params{Mode: ModeBuy, NPC: k.npc}); got != k.want {
			t.Errorf("buy at %s: engine %d, expected %d", k.name, got, k.want)
		}
	}

	// Reduced vendor prices (Gheed's Fortune 10-15%) take a straight percentage off.
	if got := ItemPrice(&item, Params{Mode: ModeBuy, NPC: akara, ReducedPrices: 15}); got != 850 {
		t.Errorf("15%% reduced prices: %d", got)
	}

	// Selling: the vendor pays the "buy mult" share of the item value.
	sell := ItemPrice(&item, Params{Mode: ModeSell, NPC: akara})
	if sell != 500 {
		t.Errorf("sell value of a 1000 gold normal item at Akara: %d", sell)
	}

	// Ethereal items sell for a quarter.
	eth := item
	eth.Ethereal = true

	if got := ItemPrice(&eth, Params{Mode: ModeSell, NPC: akara}); got != 125 {
		t.Errorf("ethereal sell: %d", got)
	}

	// Repair: cost scales with the lost fraction of durability and the 1/8 repair multiplier.
	armor := Item{BaseCost: 1000, Identified: true, Quality: QualityNormal, HasDurability: true, Repairable: true,
		MaxDur: 40, CurDur: 20}

	for _, k := range []struct {
		cur  int
		want int
	}{{20, 1000 * 20 / 40 * 128 / 1024}, {0, 1000 * 128 / 1024}, {39, 1000 * 1 / 40 * 128 / 1024}} {
		armor.CurDur = k.cur

		got := ItemPrice(&armor, Params{Mode: ModeRepair, NPC: charsi})
		// Charsi's repair multiplier is 128/1024 and her base for repairs is
		// the plain cost (the 960 factor is the buy multiplier only).
		want := k.want
		if want < 1 {
			want = 1
		}

		if got != want {
			t.Errorf("repair %d/40: engine %d, expected %d", k.cur, got, want)
		}
	}

	// Armor prices scale with the rolled defense (cost * def / maxac).
	arm := Item{BaseCost: 1000, Identified: true, Quality: QualityNormal, IsArmour: true, MinAC: 10, MaxAC: 20, Defense: 15}
	if got := ItemPrice(&arm, Params{Mode: ModeBuy, NPC: akara}); got != 750 {
		t.Errorf("armor defense scaling: %d", got)
	}

	// Cain: 100 gold per unidentified item, free after the quest.
	if IdentifyCost(5, false) != 500 || IdentifyCost(5, true) != 0 {
		t.Error("identify cost")
	}
}

func TestW30WikiGamble(t *testing.T) {
	misc := w30Read(t, "misc.txt")
	weap := w30Read(t, "weapons.txt")

	// Rings and amulets cost their gamble column whatever the level.
	for _, code := range []string{"rin", "amu"} {
		r := misc.find("code", code)
		g := Gamble{IsRingOrAmulet: true, GambleCost: misc.n(r, "gamble cost")}

		for _, l := range []int{1, 50, 99} {
			if got := GamblePrice(l, g); got != g.GambleCost {
				t.Errorf("%s at level %d: %d want %d", code, l, got, g.GambleCost)
			}
		}

		t.Logf("%s gamble cost column %d", code, g.GambleCost)
	}

	// A weapon family: the price grows with the character level and the
	// exceptional / elite versions come into the mix as the level passes
	// their requirement.
	row := func(code string) []string { return weap.find("code", code) }

	norm, exc, elite := row("wax"), row("9wa"), row("7wa")
	g := Gamble{
		ReqLevel: weap.n(norm, "levelreq"), Cost: weap.n(norm, "cost"), MinStack: 1, MaxStack: 1,
		HasExc: true, ExcReq: weap.n(exc, "levelreq"), ExcCost: weap.n(exc, "cost"),
		HasElite: true, EliteReq: weap.n(elite, "levelreq"), EliteCost: weap.n(elite, "cost"),
	}

	prev := 0

	for _, l := range []int{1, 10, 20, 30, 40, 50, 60, 70, 80, 90, 99} {
		p := GamblePrice(l, g)
		if p < prev {
			t.Errorf("gamble price falls from %d to %d at level %d", prev, p, l)
		}

		prev = p
		t.Logf("War Axe family gamble price at clvl %d: %d", l, p)
	}
}
