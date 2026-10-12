package d2state

import "testing"

// 0x56c740: a monster class with the npc column set, or without isAtt, takes no timed state.
func TestTimedStateRefusedForMonster(t *testing.T) {
	cases := []struct{ npc, att, want bool }{{false, true, false}, {true, true, true}, {false, false, true}, {true, false, true}}
	for _, c := range cases {
		if got := TimedStateRefusedForMonster(c.npc, c.att); got != c.want {
			t.Errorf("npc=%v attackable=%v: %v", c.npc, c.att, got)
		}
	}
}
