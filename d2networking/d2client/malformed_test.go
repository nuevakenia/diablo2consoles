package d2client

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// A packet the client does not know, or one that is not valid JSON for its type, is an error and
// never ends the game (it used to be a Fatalf for unknown types).
func TestMalformedServerPackets(t *testing.T) {
	g := &GameClient{Logger: d2util.NewLogger(), Players: nil}

	if err := g.handlePacket(d2netpacket.NetPacket{PacketType: 400}); err == nil {
		t.Error("unknown packet type accepted")
	}

	for _, typ := range []d2netpackettype.NetPacketType{d2netpackettype.Chat, d2netpackettype.UpdateServerInfo,
		d2netpackettype.MovePlayer, d2netpackettype.CastSkill, d2netpackettype.PlayerDisconnectionNotification} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panic %v", typ, r)
				}
			}()

			if err := g.handlePacket(d2netpacket.NetPacket{PacketType: typ, PacketData: []byte("{")}); err == nil {
				t.Errorf("%s: garbage accepted", typ)
			}
		}()
	}

	// valid JSON about players this client does not know
	for _, typ := range []d2netpackettype.NetPacketType{d2netpackettype.MovePlayer, d2netpackettype.CastSkill,
		d2netpackettype.PlayerDisconnectionNotification} {
		if err := g.handlePacket(d2netpacket.NetPacket{PacketType: typ, PacketData: []byte(`{"playerId":"nobody","id":"nobody"}`)}); err != nil {
			t.Errorf("%s about an unknown player: %v", typ, err)
		}
	}
}
