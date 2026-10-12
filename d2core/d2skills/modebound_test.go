package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// UNIT_SetUnitMode -> UNIT_RemoveModeBoundStatLists (VERIFIED): the Concentrate state has no timer and ends when the
// hero's mode changes.
func TestModeBoundStateEndsOnModeChange(t *testing.T) {
	e := auraEngine(d2state.Defs{})
	p := &d2mapentity.Player{}
	mode := d2enum.PlayerAnimationModeAttack1

	e.setOf(p.ID()).Apply(0, d2state.Instance{Name: "concentrate"}) // Until 0: no expiry
	e.modeBound = append(e.modeBound, &modeBoundState{p: p, state: "concentrate", read: func() d2enum.PlayerAnimationMode { return mode }})

	e.tickModeBound() // first look records the mode
	e.tickModeBound()

	if !e.setOf(p.ID()).Active(100, "concentrate") {
		t.Fatal("state removed without a mode change")
	}

	mode = d2enum.PlayerAnimationModeTownNeutral // back to neutral

	e.tickModeBound()

	if e.setOf(p.ID()).Active(100, "concentrate") || len(e.modeBound) != 0 {
		t.Fatal("state kept after the mode change")
	}
}
