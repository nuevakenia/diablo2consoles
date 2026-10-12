package d2client

import "github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"

// handleJoinSyncPacket receives the host's join sync (0x544140 / 0x536ef0). The
// client owns its quests (they came with AddPlayer), so the packet is logged and
// handed to OnJoinSync.
func (g *GameClient) handleJoinSyncPacket(packet d2netpacket.NetPacket) error {
	p, err := d2netpacket.UnmarshalJoinSync(packet.PacketData)
	if err != nil {
		return err
	}

	g.Infof("JOINSYNC player=%s act=%d reveal=%d active=%d record=%d quest1late=%v merc=%v", p.Player, p.Act,
		p.Sync.RevealLevel, len(p.Sync.Active), len(p.Sync.Record), p.Sync.Quest1Late, p.Merc)

	if g.OnJoinSync != nil {
		g.OnJoinSync(p)
	}

	return nil
}
