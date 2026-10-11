package d2gsnet

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2gs"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// The client turns a 0xB4 refusal into the ServerFull packet carrying the code.
func TestClientDecodesJoinReject(t *testing.T) {
	for _, code := range []uint32{d2gs.JoinErrNameOrGame, d2gs.JoinErrFull, d2gs.JoinErrVersion} {
		c, _ := newPair()

		got, _, err := c.Decode(wire(t, [][]byte{d2gs.JoinReject{Code: code}.Marshal()}))
		if err != nil || len(got) != 1 || got[0].PacketType != d2netpackettype.ServerFull {
			t.Fatalf("code %d: %v %+v", code, err, got)
		}

		sf, _ := d2netpacket.UnmarshalServerFull(got[0].PacketData)
		if sf.Code != code || sf.Reason == "" {
			t.Fatalf("%+v", sf)
		}
	}
}

// The client's join carries the version the server checks.
func TestJoinCarriesVersion(t *testing.T) {
	c, s := newPair()

	st := &d2hero.HeroState{HeroName: "Ann", HeroType: d2enum.HeroSorceress, Stats: &d2hero.HeroStatsState{Level: 5}}
	req, _ := d2netpacket.CreatePlayerConnectionRequestPacket("u", st)

	pkts, err := c.Encode(req)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Decode(wire(t, pkts)); err != nil {
		t.Fatal(err)
	}

	if v := s.Joined().Version; v != d2gs.GameVersion {
		t.Fatalf("version %d", v)
	}
}
