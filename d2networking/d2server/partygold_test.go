package d2server

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// A gold pickup of a party member is shared by the server (0x53e640): two in the
// party get half each, the picker also learns the overflow; a hero's own drop
// goes to the picker alone.
func TestServerPartyGold(t *testing.T) {
	g, a, b := testServer()
	party(t, g, a, d2netpacket.PartyInvite, "b")
	party(t, g, b, d2netpacket.PartyAccept, "")

	adds := func(c *fakeConn) (add, over, n int) {
		for _, pk := range c.got {
			if pk.PacketType == d2netpackettype.PartyGold {
				x, _ := d2netpacket.UnmarshalPartyGold(pk.PacketData)
				if x.Player != c.id || x.Picker != "a" {
					t.Fatalf("award %+v to %s", x, c.id)
				}

				add, over, n = x.Add, x.Overflow, n+1
			}
		}

		return
	}

	p, _ := d2netpacket.CreatePartyGoldPacket(d2netpacket.PartyGoldPacket{Picker: "x", Amount: 101})
	if err := g.onPartyGold(a, p); err != nil {
		t.Fatal(err)
	}

	if add, over, n := adds(a); add != 51 || over != 0 || n != 1 {
		t.Fatalf("picker add=%d over=%d n=%d, want 51 0 1", add, over, n)
	}

	if add, _, n := adds(b); add != 50 || n != 1 {
		t.Fatalf("member add=%d n=%d, want 50 1", add, n)
	}

	a.got, b.got = nil, nil
	p, _ = d2netpacket.CreatePartyGoldPacket(d2netpacket.PartyGoldPacket{Amount: 100, PlayerDropped: true})
	_ = g.onPartyGold(a, p)

	if add, _, n := adds(a); add != 100 || n != 1 {
		t.Fatalf("dropped pile: picker add=%d n=%d, want 100 1", add, n)
	}

	if _, _, n := adds(b); n != 0 {
		t.Fatal("a hero's own drop must not be shared")
	}
}
