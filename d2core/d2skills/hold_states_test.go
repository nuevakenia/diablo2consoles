package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// PLRMODE_CanInterruptCurrentMode (0x57ceb0, VERIFIED): state 0x2a = "concentration" with chance stat 0xa4
// (skill_concentration), state 0xf = "concentrate"; the aura only counts for a used skill with the hold bit.
func TestHoldInputsReadConcentrationAndConcentrate(t *testing.T) {
	e := auraEngine(d2state.Defs{})
	p := &d2mapentity.Player{}
	u := &heroUnit{e: e, p: p, seed: d2rand.New(7)}

	in := d2skill.InterruptInput{SkillBits4: d2skill.Bits4Hold}
	e.holdInputs(p, u, &in)

	if in.State2a || in.State0f {
		t.Fatalf("no state set, got %+v", in)
	}

	e.setOf(p.ID()).Apply(0, d2state.Instance{Name: "concentration",
		Mods: []d2state.StatMod{{Stat: "skill_concentration", Value: 20}, {Stat: "damagepercent", Value: 60}}})
	e.setOf(p.ID()).Apply(0, d2state.Instance{Name: "concentrate"})

	in = d2skill.InterruptInput{SkillBits4: d2skill.Bits4Hold}
	e.holdInputs(p, u, &in)

	if !in.State2a || in.State2aChance != 20 || !in.State0f || in.Roll < 0 || in.Roll > 99 {
		t.Fatalf("got %+v", in)
	}

	// Without the hold bit the aura is not consulted (no roll is spent), state 0xf is still reported.
	in = d2skill.InterruptInput{}
	e.holdInputs(p, u, &in)

	if in.State2a || !in.State0f {
		t.Fatalf("got %+v", in)
	}
}

// State 0x36 is the row "uninterruptable" of states.txt (VERIFIED order); it refuses the interruption at once.
func TestHoldInputsUninterruptableRefuses(t *testing.T) {
	e := auraEngine(d2state.Defs{})
	p := &d2mapentity.Player{}
	u := &heroUnit{e: e, p: p, seed: d2rand.New(7)}

	e.setOf(p.ID()).Apply(0, d2state.Instance{Name: "uninterruptable"})

	in := d2skill.InterruptInput{CurrentMode: 1, NewMode: 9, Flag: true, HasUsedSkill: true}
	e.holdInputs(p, u, &in)

	if ok, _ := d2skill.CanInterrupt(in); ok || !in.StateBlocked {
		t.Fatalf("blocked state not honoured: %+v", in)
	}
}
