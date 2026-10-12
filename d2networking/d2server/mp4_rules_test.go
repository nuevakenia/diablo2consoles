package d2server

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2portal"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

func countType(c *fakeConn, typ d2netpackettype.NetPacketType) int {
	n := 0

	for _, p := range c.got {
		if p.PacketType == typ {
			n++
		}
	}

	return n
}

// A forged kill with negative experience or a pickup of negative gold must not reach anybody.
func TestForgedXPAndGoldRefused(t *testing.T) {
	g, a, b := testServer()
	party(t, g, a, d2netpacket.PartyInvite, "b")
	party(t, g, b, d2netpacket.PartyAccept, "")

	xp, _ := d2netpacket.CreatePartyXPPacket(d2netpacket.PartyXPPacket{XP: -500, Monster: "x"})
	gold, _ := d2netpacket.CreatePartyGoldPacket(d2netpacket.PartyGoldPacket{Amount: -100})

	if err := g.onPartyXP(a, xp); err != nil {
		t.Fatal(err)
	}

	if err := g.onPartyGold(a, gold); err != nil {
		t.Fatal(err)
	}

	for _, c := range []*fakeConn{a, b} {
		if countType(c, d2netpackettype.PartyXP)+countType(c, d2netpackettype.PartyGold) != 0 {
			t.Fatalf("%s received a forged award", c.id)
		}
	}
}

// Party XP with a member that left or died mid fight: the killer alone gets it all, a member in
// another area gets nothing.
func TestPartyXPMembership(t *testing.T) {
	g, a, b := testServer()
	party(t, g, a, d2netpacket.PartyInvite, "b")
	party(t, g, b, d2netpacket.PartyAccept, "")

	award := func(c *fakeConn) int {
		sum := 0

		for _, p := range c.got {
			if p.PacketType == d2netpackettype.PartyXP {
				x, _ := d2netpacket.UnmarshalPartyXP(p.PacketData)
				sum += x.Amount
			}
		}

		return sum
	}

	g.socialArea("a", 2)
	g.socialArea("b", 3) // other level: no share
	pk, _ := d2netpacket.CreatePartyXPPacket(d2netpacket.PartyXPPacket{XP: 90})
	_ = g.onPartyXP(a, pk)

	if award(a) != 90 || award(b) != 0 {
		t.Fatalf("other area: a=%d b=%d", award(a), award(b))
	}

	a.got, b.got = nil, nil
	g.socialArea("b", 2)

	delete(g.connections, "b") // b drops: the roster entry is gone a moment later
	g.socialRemovePlayer("b")
	_ = g.onPartyXP(a, pk)

	if award(a) != 90 {
		t.Fatalf("after b left a gets %d", award(a))
	}
}

// One hero opens a town portal; another can see it, a third joining later is told, the owner leaving
// closes it, and a second pair replaces the first one.
func TestPortalLifecycle(t *testing.T) {
	g, a, b := testServer()

	open := func(c *fakeConn, level int) {
		pk, _ := d2netpacket.CreatePortalOpenPacket(d2netpacket.PortalOpenPacket{
			Pair: d2portal.Pair{Owner: "spoof", Field: d2portal.End{Level: level}, Town: d2portal.End{Level: 1}}})
		if err := g.onPortalOpen(c, pk); err != nil {
			t.Fatal(err)
		}
	}

	open(a, 2)

	if p, ok := g.soc.portals.Get("a"); !ok || p.Field.Level != 2 || p.Owner != "a" {
		t.Fatalf("pair %+v %v", p, ok)
	}

	if countType(b, d2netpackettype.PortalUpdate) == 0 {
		t.Fatal("b was not told")
	}

	open(a, 3)

	if n := len(g.soc.portals.Pairs()); n != 1 {
		t.Fatalf("a second portal of the same owner must replace the first, have %d", n)
	}

	open(b, 4)

	if n := len(g.soc.portals.Pairs()); n != 2 {
		t.Fatalf("two owners: %d pairs", n)
	}

	delete(g.connections, "a")
	g.socialRemovePlayer("a")

	if _, ok := g.soc.portals.Get("a"); ok || len(g.soc.portals.Pairs()) != 1 {
		t.Fatal("a's portal must close when a leaves")
	}
}

// A trade request while one side is in another trade, and a third hero joining the same trade, are refused.
func TestTradeRaceThirdParty(t *testing.T) {
	g, a, b := testServer()
	c := &fakeConn{id: "c", state: hero("Cy", 5, 10)}
	g.connections["c"] = c
	g.socialAddPlayer(c)

	tradeCmd(t, g, a, tradeReq("b"))
	tradeCmd(t, g, c, tradeReq("b")) // b is busy

	if u := lastTrade(t, c); u.State != "cancelled" || u.Reason == "" {
		t.Fatalf("third party: %+v", u)
	}

	tradeCmd(t, g, c, tradeReq("a")) // a is busy too

	if u := lastTrade(t, c); u.State != "cancelled" {
		t.Fatalf("a busy: %+v", u)
	}

	// the stranger cannot answer, offer or accept in somebody else's trade
	for _, op := range []string{d2netpacket.TradeRespond, d2netpacket.TradeOffer, d2netpacket.TradeAccept} {
		tradeCmd(t, g, c, d2netpacket.TradeCommandPacket{Op: op, Accept: true})
	}

	if s := g.soc.trades["a"]; s == nil || s.Other("a") != "b" || g.soc.trades["c"] != nil {
		t.Fatal("the running trade was disturbed")
	}

	_ = b
}

func tradeReq(target string) d2netpacket.TradeCommandPacket {
	return d2netpacket.TradeCommandPacket{Op: d2netpacket.TradeRequest, Target: target}
}

// Leaving a party mid fight (the leaver is also the one who died): the rest still share.
func TestPartyLeaveMidFight(t *testing.T) {
	g, a, b := testServer()
	c := &fakeConn{id: "c", state: hero("Cy", 10, 10)}
	g.connections["c"] = c
	g.socialAddPlayer(c)
	party(t, g, a, d2netpacket.PartyInvite, "b")
	party(t, g, b, d2netpacket.PartyAccept, "")
	party(t, g, a, d2netpacket.PartyInvite, "c")
	party(t, g, c, d2netpacket.PartyAccept, "")

	party(t, g, c, d2netpacket.PartyLeave, "")

	if g.soc.roster.SameParty("a", "c") || !g.soc.roster.SameParty("a", "b") {
		t.Fatal("party after c left")
	}

	// leaving twice, or by a hero that never was in a party, is a refusal, not a crash
	party(t, g, c, d2netpacket.PartyLeave, "")
	party(t, g, b, d2netpacket.PartyAccept, "")
}
