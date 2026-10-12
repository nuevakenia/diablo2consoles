package d2server

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// The killer of the third Ancient reports it; a party member standing in level 120 at level 20 (the Normal gate) is
// paid; one in Harrogath is not (the killer's own bit 0 is set first, so the party spread starts from nobody: the
// exe quirk of 0x58a7b0 / 0x58a600), and neither is one in Act 1: both get the goal mark.
func TestServerQuestShareRite(t *testing.T) {
	outcome := func(c *fakeConn) (d2quest.RiteOutcome, int) {
		n := 0
		o := d2quest.RiteNothing

		for _, pk := range c.got {
			if pk.PacketType == d2netpackettype.QuestShare {
				x, _ := d2netpacket.UnmarshalQuestShare(pk.PacketData)
				o, n = d2quest.RiteOutcome(x.Outcome), n+1
			}
		}

		return o, n
	}

	g, a, b := testServer()
	party(t, g, a, d2netpacket.PartyInvite, "b")
	party(t, g, b, d2netpacket.PartyAccept, "")
	g.socialArea("a", d2quest.LevelSummit)
	g.socialArea("b", d2quest.LevelSummit)

	pkt, _ := d2netpacket.CreateQuestSharePacket(d2netpacket.QuestSharePacket{Quest: d2quest.QuestRite})
	if err := g.onQuestShare(a, pkt); err != nil {
		t.Fatal(err)
	}

	if _, n := outcome(a); n != 0 {
		t.Error("the killer is paid by its own client")
	}

	if o, n := outcome(b); n != 1 || o != d2quest.RiteReward {
		t.Errorf("party member in the summit: outcome %v n=%d", o, n)
	}

	for _, area := range []int{109, 1} {
		b.got = nil
		g.socialArea("b", area)
		_ = g.onQuestShare(a, pkt)

		if o, n := outcome(b); n != 1 || o != d2quest.RiteGoalOnly {
			t.Errorf("party member in level %d: outcome %v n=%d", area, o, n)
		}
	}

	// another quest is not shared
	b.got = nil
	other, _ := d2netpacket.CreateQuestSharePacket(d2netpacket.QuestSharePacket{Quest: d2quest.QuestSiege})
	_ = g.onQuestShare(a, other)

	if _, n := outcome(b); n != 0 {
		t.Error("only Rite is shared")
	}
}
