package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
)

func TestWeaponBlockChance(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	only := func(want string) func(string) bool { return func(s string) bool { return s == want } }

	tests := []struct {
		name      string
		mods      []TruePassiveMod
		leftEmpty bool
		left      func(string) bool
		right     func(string) bool
		want      int
	}{
		{"no mods", nil, false, yes, yes, 0},
		{"other stat", []TruePassiveMod{{Stat: "passive_dodge", Value: 40, Param: "h2h"}}, false, yes, yes, 0},
		{"left matches", []TruePassiveMod{{Stat: "passive_weaponblock", Value: 30, Param: "h2h"}}, false, only("h2h"), no, 30},
		{"right matches only", []TruePassiveMod{{Stat: "passive_weaponblock", Value: 30, Param: "h2h"}}, false, no, only("h2h"), 30},
		{"empty left falls to right", []TruePassiveMod{{Stat: "passive_weaponblock", Value: 25, Param: "h2h"}}, true, yes, only("h2h"), 25},
		{"neither matches", []TruePassiveMod{{Stat: "passive_weaponblock", Value: 30, Param: "h2h"}}, false, no, no, 0},
		{"largest of matching entries", []TruePassiveMod{{Stat: "passive_weaponblock", Value: 30, Param: "h2h"},
			{Stat: "passive_weaponblock", Value: 45, Param: "swor"}, {Stat: "passive_weaponblock", Value: 20, Param: "h2h"}},
			false, yes, yes, 45},
		// an entry without a type replaces the running value even when it is smaller (0x57bd47)
		{"typeless entry overrides", []TruePassiveMod{{Stat: "passive_weaponblock", Value: 50, Param: "h2h"},
			{Stat: "passive_weaponblock", Value: 10}}, false, yes, yes, 10},
	}

	for _, tc := range tests {
		if got := WeaponBlockChance(tc.mods, tc.leftEmpty, tc.left, tc.right); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

// A passing block roll ends the chain before dodge / avoid (outcome 0x10).
func TestWeaponBlockRollEndsChain(t *testing.T) {
	r := &seq{vals: []uint32{10}}
	out := d2combat.RollAvoid(r, d2combat.AvoidInput{AnimBlockChance: 40, AnimIsBlock: true, DodgeChance: 90})

	if out != d2combat.AvoidMonBlock || len(r.vals) != 0 {
		t.Errorf("out %v, rolls left %d", out, len(r.vals))
	}
}
