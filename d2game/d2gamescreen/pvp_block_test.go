package d2gamescreen

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
)

func TestPvPHitRollsBlock(t *testing.T) {
	cases := []struct {
		name string
		p    d2netpacket.PvPHitPacket
		want bool
	}{
		{"melee swing", d2netpacket.PvPHitPacket{}, true},
		{"fire ball (no physical damage)", d2netpacket.PvPHitPacket{Skill: "Fire Ball", Rate: 1024}, false},
		{"meteor area", d2netpacket.PvPHitPacket{Skill: "Meteor area"}, false},
		{"blizzard splash", d2netpacket.PvPHitPacket{Skill: "Blizzard splash"}, false},
		{"arrow with physical damage", d2netpacket.PvPHitPacket{Skill: "Multiple Shot", Blockable: true}, true},
	}

	for _, c := range cases {
		if got := pvpHitRollsBlock(c.p); got != c.want {
			t.Errorf("%s: rolls block = %v, want %v", c.name, got, c.want)
		}
	}
}
