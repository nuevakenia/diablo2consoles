package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// The friendly Paladin auras (filter 73731) reach allied monsters (minions); Meditation (73729) is players only;
// Holy Fire's 42883 has no allied bit (VERIFIED filter bits, 0x569100).
func TestPaladinAuraFilterReach(t *testing.T) {
	ally := d2skill.FilterFacts{Kind: d2skill.KindMonster, Allied: true}

	for _, c := range []struct {
		filter int
		want   bool
	}{{73731, true}, {73729, false}, {42883, false}} {
		if got := d2skill.FilterMatch(c.filter, ally); got != c.want {
			t.Errorf("filter %d reaches an allied minion = %v, want %v", c.filter, got, c.want)
		}
	}
}

func TestAllyAuraPulseNoPets(t *testing.T) {
	e := &Engine{Logger: d2util.NewLogger(), sets: map[string]*d2state.Set{}, heroes: map[string]*heroUnit{},
		monsters: &d2monsters.Director{}, pets: map[string][]*d2mapentity.Monster{}}
	p := &d2mapentity.Player{}
	ef := &d2skill.Effect{State: "might", Filter: 73731, Radius: 10}

	e.allyAuraPulse(p, 0, 0, ef, []d2state.StatMod{{Stat: "damagepercent", Value: 40}}, 10, true)
	e.allyAuraPulse(p, 0, 0, ef, nil, 0, false)
}
