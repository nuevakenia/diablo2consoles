package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// A monster the director does not list as an ally is a normal target; nil and a missing director are safe.
// (The ally case runs in the 97 / 86 scenarios with real summons: sparing needs a spawned minion.)
func TestSparesOwnUnitOnlyAllies(t *testing.T) {
	e, _ := defenseEngine()
	m := &d2mapentity.Monster{}

	for name, mon := range map[string]*d2monsters.Director{"empty director": {}, "no director": nil} {
		e.monsters = mon

		if e.sparesOwnUnit(m, "test") {
			t.Errorf("%s: an unlisted monster was spared", name)
		}
	}

	if e.sparesOwnUnit(nil, "test") {
		t.Error("nil monster spared")
	}
}
