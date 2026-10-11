package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestBoostOf(t *testing.T) {
	got := boostOf([]d2state.StatMod{
		{Stat: "damagepercent", Value: 40}, {Stat: "item_tohit_percent", Value: 75}, {Stat: "item_maxhp_percent", Value: 90},
		{Stat: "thorns_percent", Value: 200},
	})

	want := d2monsters.AuraBoost{DamagePct: 40, ToHitPct: 75, MaxHPPct: 90}
	if got != want {
		t.Errorf("boost %+v, want %+v", got, want)
	}
}

func TestCorpseFacts(t *testing.T) {
	e := &Engine{}

	still := &d2mapentity.Monster{Stat: &d2records.MonStatRecord{SpeedBase: 0}}
	if f := e.corpseFacts(still); f.WalkSpeed != 0 || !f.Dead || !f.Selectable || d2skill.CorpseRaisable(*f, false) {
		t.Errorf("stationary corpse %+v", f)
	}

	walker := &d2mapentity.Monster{Stat: &d2records.MonStatRecord{SpeedBase: 9}}
	if f := e.corpseFacts(walker); !d2skill.CorpseRaisable(*f, false) || !d2skill.CorpseExplodable(*f) {
		t.Errorf("walking corpse %+v", f)
	}

	// no stat record at all: usable, like the pipeline's own default
	if f := e.corpseFacts(&d2mapentity.Monster{}); !d2skill.CorpseRaisable(*f, false) {
		t.Errorf("recordless corpse %+v", f)
	}
}

// A totem pulse puts the aura state on the hero inside the circle and takes it off outside, through the filter
// 0x10103 (batch 5).
func TestTotemPulseRange(t *testing.T) {
	e := &Engine{Logger: d2util.NewLogger(), sets: map[string]*d2state.Set{}, heroes: map[string]*heroUnit{},
		monsters: &d2monsters.Director{}, pets: map[string][]*d2mapentity.Monster{}}

	p := &d2mapentity.Player{}
	u := e.hero(p)
	totem := &d2mapentity.Monster{}
	totem.Position.Set(10, 10)

	sk := &d2skill.Skill{ID: 226}
	ef := &d2skill.Effect{Level: 5}
	mods := []d2state.StatMod{{Stat: "item_maxhp_percent", Value: 50}}

	p.Position.Set(12, 11) // inside radius 5
	e.frame = 1
	e.totemPulse(p, u, sk, ef, 65795, "oaksagecontrol", 5, mods, totem)

	if in := e.setOf(p.ID()).Get(e.frame, "oaksagecontrol"); in == nil || len(in.Mods) != 1 {
		t.Fatalf("hero inside: %+v", in)
	}

	p.Position.Set(30, 30)
	e.totemPulse(p, u, sk, ef, 65795, "oaksagecontrol", 5, mods, totem)

	if in := e.setOf(p.ID()).Get(e.frame, "oaksagecontrol"); in != nil {
		t.Errorf("hero outside keeps the state: %+v", in)
	}

	// a filter without the allied bit (and monsters only) does not reach the hero
	p.Position.Set(12, 11)
	e.totemPulse(p, u, sk, ef, d2skill.FilterMonsters, "oaksagecontrol", 5, mods, totem)

	if in := e.setOf(p.ID()).Get(e.frame, "oaksagecontrol"); in != nil {
		t.Errorf("monster-only filter reached the hero: %+v", in)
	}

	e.totemClear(p, "oaksagecontrol")
}
