package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

func tgt0() d2monster.Target { return d2monster.Target{ID: 7, X: 3, Y: 3, Size: 1} }

func TestShadowsUsePetAI(t *testing.T) {
	for _, tc := range []struct {
		ai   string
		kind string
		want bool
	}{
		{"ShadowWarrior", "minion", true},
		{"ShadowMaster", "minion", true},
		{"shadowmasternoinit", "minion", true},
		{"ShadowMaster", "", false}, // a hostile copy keeps the monster driver
		{"ShadowMaster", "trap", false},
		{"ShadowDancer", "minion", false},
	} {
		if got := usesPetAI(petUnit(1, 0, 0, tc.ai, tc.kind)); got != tc.want {
			t.Errorf("%s/%s: got %v", tc.ai, tc.kind, got)
		}
	}
}

func TestShadowBook(t *testing.T) {
	sh := petUnit(1, 0, 0, "ShadowWarrior", "minion")
	other := petUnit(2, 0, 0, "NecroPet", "minion")
	d := testDirector(sh, other)

	if !d.HasSkills(sh.b) || d.HasSkills(other.b) {
		t.Error("only a shadow counts as having skills")
	}

	if d.CastQueued(sh.b, 1, tgt0(), true) || d.Upkeep(sh.b, nil, true) {
		t.Error("no queued cast or upkeep is hosted")
	}

	if id, ok := d.Choose(sh.b, tgt0(), nil, true); ok || id != 0 {
		t.Error("nothing is scored")
	}

	// out of reach the copy declines (the think then runs to the target)
	if _, ok := d.Copy(sh.b, tgt0(), tgt0(), false, false, false); ok {
		t.Error("a copy out of reach must decline")
	}
}
