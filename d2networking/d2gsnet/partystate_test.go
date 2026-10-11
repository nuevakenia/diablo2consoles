package d2gsnet

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2party"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2gs"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
)

// The server sends 0x8B to the owner of a relation list only, one per node, and
// the client stores the byte for the other unit.
func TestPartyState8BEmission(t *testing.T) {
	r := d2party.New()
	r.Add(d2party.Member{ID: "a", Name: "Ann", Level: 20})
	r.Add(d2party.Member{ID: "b", Name: "Bob", Level: 20})

	if err := r.Invite("a", "b"); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Accept("b"); err != nil {
		t.Fatal(err)
	}

	np, err := d2netpacket.CreateRosterUpdatePacket(d2netpacket.RosterUpdatePacket{Roster: r.Snapshot()})
	if err != nil {
		t.Fatal(err)
	}

	c, s := newPair()
	s.PlayerID = "a"

	pkts, err := s.Encode(np)
	if err != nil {
		t.Fatal(err)
	}

	var got []d2gs.PartyState

	for _, p := range pkts {
		if p[0] == d2gs.S2CPartyState {
			ps, perr := d2gs.ParsePartyState(p)
			if perr != nil {
				t.Fatal(perr)
			}

			got = append(got, ps)
		}
	}

	bu := s.IDs.Assign("b")
	if len(got) != 1 || got[0].UnitID != bu || got[0].State != 1 {
		t.Fatalf("0x8B packets %+v, want one for b (unit %d) with state 1", got, bu)
	}

	if _, _, err := c.Decode(wire(t, pkts)); err != nil {
		t.Fatal(err)
	}

	if p, ok := c.Peer(bu); !ok || p.PartyState != 1 {
		t.Fatalf("peer %+v %v", p, ok)
	}

	// the wire form is the verified 6 bytes
	b := d2gs.PartyState{UnitID: 0x01020304, State: 1}.Marshal()
	if len(b) != 6 || b[0] != 0x8b || b[5] != 1 {
		t.Fatalf("% x", b)
	}
}
