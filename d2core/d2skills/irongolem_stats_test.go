package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2stats"
)

type fakeVal struct {
	d2stats.StatValue
	n int
	t d2stats.StatNumberType
}

func (f fakeVal) NumberType() d2stats.StatNumberType { return f.t }
func (f fakeVal) Int() int                           { return f.n }

type fakeStat struct {
	d2stats.Stat
	name string
	vals []d2stats.StatValue
}

func (f fakeStat) Name() string                { return f.name }
func (f fakeStat) Values() []d2stats.StatValue { return f.vals }

type fakeList struct {
	d2stats.StatList
	s []d2stats.Stat
}

func (f fakeList) Stats() []d2stats.Stat { return f.s }

func TestGolemItemMods(t *testing.T) {
	one := func(n int) []d2stats.StatValue { return []d2stats.StatValue{fakeVal{n: n}} }
	l := fakeList{s: []d2stats.Stat{
		fakeStat{name: "FireResist", vals: one(30)},
		fakeStat{name: "maxhp", vals: one(20)},
		fakeStat{name: "firemindam", vals: []d2stats.StatValue{fakeVal{n: 1}, fakeVal{n: 5}}},
		fakeStat{name: "item_armor_percent", vals: one(50)},
	}}

	got := golemItemMods(l)
	if len(got) != 3 || got[0].Stat != "fireresist" || got[0].Value != 30 || got[1].Value != 20<<8 || got[2].Stat != "item_armor_percent" {
		t.Errorf("mods %+v", got)
	}

	if golemItemMods(nil) != nil {
		t.Error("nil list")
	}
}
