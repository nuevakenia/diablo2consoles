package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// PlayerScaling: the fifth word of MONSTER_CalcPlayerCountScaling is the player count, or 1 for a class whose
// monstats Align byte (+0x4c) is non-zero.
func TestPlayerScalingAlign(t *testing.T) {
	for _, tc := range []struct {
		name    string
		align   int
		players int
		want    int
	}{
		{"hostile, 4 players", 0, 4, 4},
		{"hostile, solo", 0, 1, 1},
		{"friendly class keeps 1", 1, 4, 1},
		{"neutral class keeps 1", 2, 8, 1},
	} {
		u := petUnit(1, 0, 0, "Diablo", "")
		u.m.Stat = &d2records.MonStatRecord{}
		u.m.Stat.Alignment = d2enum.MonsterAlignmentType(tc.align)

		d := testDirector(u)
		d.opt.Players = tc.players

		if got := d.PlayerScaling(u.b); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}
