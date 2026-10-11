package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2rand"
)

// The Valkyrie's passive monster skills (sumskill1..4): dodge only against melee, avoid only against missiles,
// evade only while moving (d2combat.RollAvoid rules).
func TestSummonAvoids(t *testing.T) {
	seed := func() *d2rand.Seed { s := &d2rand.Seed{}; s.Init(1); return s }

	tests := []struct {
		name           string
		o              MinionOptions
		moving, ranged bool
		want           bool
	}{
		{"none", MinionOptions{}, false, false, false},
		{"dodge melee", MinionOptions{Dodge: 100}, false, false, true},
		{"dodge ignores missiles", MinionOptions{Dodge: 100}, false, true, false},
		{"avoid missile", MinionOptions{Avoid: 100}, false, true, true},
		{"avoid ignores melee", MinionOptions{Avoid: 100}, false, false, false},
		{"evade moving", MinionOptions{Evade: 100}, true, false, true},
		{"evade standing", MinionOptions{Evade: 100}, false, false, false},
		{"moving rolls only evade", MinionOptions{Dodge: 100}, true, false, false},
	}

	for _, tc := range tests {
		if got := summonAvoids(seed(), tc.o, tc.moving, tc.ranged); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// The Dopplezon decoy (monstats AI "Idle") never acts; the Valkyrie (NecroPet) does.
func TestIsIdleAI(t *testing.T) {
	if !isIdleAI(petUnit(1, 0, 0, "Idle", "minion")) || isIdleAI(petUnit(2, 0, 0, "NecroPet", "minion")) {
		t.Error("isIdleAI")
	}
}
