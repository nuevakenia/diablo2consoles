package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// STATE_UnitHasStateInMask150 (0x63b870) is the states.txt udead column (VERIFIED, 0x617c50): a corpse holding
// such a state is no raise / explode target.
func TestCorpseFactsReadUDeadStates(t *testing.T) {
	e := auraEngine(d2state.Defs{"corpseexp": {UDead: true}, "poison": {}})
	m := &d2mapentity.Monster{}

	if f := e.corpseFacts(m); f.InStateMask150 {
		t.Fatal("no state, flagged")
	}

	e.setOf(m.ID()).Apply(0, d2state.Instance{Name: "poison"})

	if f := e.corpseFacts(m); f.InStateMask150 {
		t.Fatal("a state without udead flagged the corpse")
	}

	e.setOf(m.ID()).Apply(0, d2state.Instance{Name: "corpseexp"})

	f := e.corpseFacts(m)
	if !f.InStateMask150 {
		t.Fatal("udead state not seen")
	}
}
