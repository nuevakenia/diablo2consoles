package d2itemdesc

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

// w36 golden tooltips: well known items with typical rolls, described with the
// real tables (D2_TABLES). The stat order inside the magic block comes from
// ItemStatCost descpriority (higher first), the rest from the build order of
// INV_BuildItemDescription (names, base stats, requirements, properties,
// footnotes). Set W36_DUMP=1 to print the current output.

type gp struct {
	id  int
	par uint32
	val int64
}

type goldenCase struct {
	name  string // unique/set index name
	set   bool
	code  string
	def   int
	dur   int
	props []gp
	want  []string
}

func (g goldenCase) item(t *testing.T, tb *Tables) d2s.Item {
	it := d2s.Item{Code: g.code, Identified: true, Defense: g.def, MaxDurability: uint16(g.dur), Durability: uint16(g.dur)}

	if g.set {
		it.Quality = d2s.QualitySet

		found := false

		for i, s := range tb.SetItems {
			if s.Index == g.name {
				it.SetID, it.Code, found = uint16(i), s.Code, true
			}
		}

		if !found {
			t.Fatalf("set item %q not in the tables", g.name)
		}
	} else {
		it.Quality = d2s.QualityUnique

		found := false

		for i, u := range tb.Uniques {
			if u.Index == g.name && !found {
				it.UniqueID, it.Code, found = uint16(i), u.Code, true
			}
		}

		if !found {
			t.Fatalf("unique %q not in the tables", g.name)
		}
	}

	for _, p := range g.props {
		it.Properties = append(it.Properties, d2s.Property{ID: p.id, Param: p.par, Value: p.val})
	}

	return it
}

var w36Cases = []goldenCase{
	{name: "Harlequin Crest", code: "uap", def: 120, dur: 12, props: []gp{{127, 0, 2}, {80, 0, 50}, {36, 0, 10}, {0, 0, 2}, {7, 0, 2}, {9, 0, 2}}},
	{name: "The Stone of Jordan", code: "rin", props: []gp{{127, 0, 1}, {9, 0, 25}}},
	{name: "Arachnid Mesh", code: "ulc", def: 100, dur: 12, props: []gp{{127, 0, 1}, {105, 0, 20}, {93, 0, 20}}},
	{name: "Windforce", code: "6lw", dur: 100, props: []gp{{17, 0, 250}, {18, 0, 250}, {93, 0, 20}, {136, 0, 5}}},
	{name: "Skin of the Vipermagi", code: "utp", def: 400, dur: 12, props: []gp{{127, 0, 1}, {105, 0, 30}, {39, 0, 30}, {41, 0, 30}, {43, 0, 30}, {45, 0, 30}, {36, 0, 10}}},
	{name: "Tyrael's Might", code: "xfp", def: 600, dur: 12, props: []gp{{0, 0, 30}}},
	{name: "Raven Frost", code: "rin", props: []gp{{2, 0, 20}, {19, 0, 200}, {43, 0, 30}}},
	{name: "The Cat's Eye", code: "amu", props: []gp{{2, 0, 25}, {96, 0, 25}}},
	{name: "Nagelring", code: "rin", props: []gp{{80, 0, 20}, {19, 0, 100}}},
	{name: "Highlord's Wrath", code: "amu", props: []gp{{127, 0, 1}, {93, 0, 20}}},
	{name: "Skullder's Ire", code: "uui", def: 400, dur: 12, props: []gp{{80, 0, 80}, {39, 0, 30}}},
	{name: "Gheed's Fortune", code: "cm3", props: []gp{{79, 0, 100}, {80, 0, 30}}},
	{name: "Deaths's Web", code: "7gw", dur: 60, props: []gp{{127, 0, 1}}},
	{name: "Griffon's Eye", code: "xsk", def: 100, dur: 12, props: []gp{{127, 0, 1}, {105, 0, 25}}},
	{name: "Bul Katho's Wedding Band", code: "rin", props: []gp{{7, 0, 50}, {60, 0, 3}}},
	{name: "Crown of Ages", code: "ci3", def: 100, dur: 12, props: []gp{{127, 0, 1}, {0, 0, 15}}},
	{name: "Thunderstroke", code: "am5", dur: 40, props: []gp{{19, 0, 100}}},
	{name: "Tal Rasha's Howling Wind", set: true, code: "xea", def: 300, dur: 12, props: []gp{{7, 0, 50}}},
	{name: "Tal Rasha's Horadric Crest", set: true, code: "xsk", def: 100, dur: 12, props: []gp{{7, 0, 50}}},
	{name: "Sigon's Shelter", set: true, code: "gth", def: 90, dur: 12, props: []gp{{7, 0, 10}}},
	{name: "Civerb's Ward", set: true, code: "lrg", def: 20, dur: 24, props: []gp{{7, 0, 10}}},
	{name: "Natalya's Totem", set: true, code: "xh9", def: 100, dur: 12, props: []gp{{7, 0, 10}}},
	{name: "Goldwrap", code: "xtb", def: 80, dur: 12, props: []gp{{80, 0, 20}}},
	{name: "Dwarf Star", code: "rin", props: []gp{{39, 0, 20}}},
	{name: "Spiritkeeper", code: "dr4", def: 80, dur: 12, props: []gp{{127, 0, 1}}},
}

// The charstats.txt "Expansion" separator must not be a class: class ids 5 and 6
// are Druid and Assassin.
func TestW36ClassOrder(t *testing.T) {
	tb := realTables(t)
	want := []string{"Amazon", "Sorceress", "Necromancer", "Paladin", "Barbarian", "Druid", "Assassin"}

	if len(tb.Classes) != len(want) {
		t.Fatalf("%d classes, want %d", len(tb.Classes), len(want))
	}

	for i, w := range want {
		if tb.Classes[i].Name != w || tb.Classes[i].ClassOnly == "" {
			t.Errorf("class %d = %q (only=%q), want %q", i, tb.Classes[i].Name, tb.Classes[i].ClassOnly, w)
		}
	}
}

func TestW36GoldenTooltips(t *testing.T) {
	tb := realTables(t)

	for _, g := range w36Cases {
		it := g.item(t, tb)
		got := texts(tb.Describe(&it, nil))

		if os.Getenv("W36_DUMP") != "" {
			s := ""
			for _, l := range got {
				s += ", " + fmt.Sprintf("%q", l)
			}

			t.Logf("GOLD\t%q: {%s},", g.name, strings.TrimPrefix(s, ", "))

			continue
		}

		want, ok := w36Golden[g.name]
		if !ok {
			t.Errorf("%s: no golden lines", g.name)

			continue
		}

		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s:\n got %q\nwant %q", g.name, got, want)
		}
	}
}
