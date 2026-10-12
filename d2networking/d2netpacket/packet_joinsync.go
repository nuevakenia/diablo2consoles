package d2netpacket

import (
	"encoding/json"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// JoinSyncPacket is sent by the host to a hero that just entered the game: the
// quest sync of QUEST_SyncPlayerOnGameEnter (0x544140) and the act placement of
// SERVER_PlacePlayerInAct (0x536ef0). OD2 already carries the waypoints, quests,
// equipment and mercenary in AddPlayer, so the packet is the host's confirmation:
// the client compares it with what it loaded.
type JoinSyncPacket struct {
	Player string          `json:"player"`
	Sync   d2hero.JoinSync `json:"sync"`
	Act    int             `json:"act"`
	Merc   bool            `json:"merc,omitempty"` // the host recreates the mercenary (0x536ef0)
}

// CreateJoinSyncPacket builds a JoinSync packet.
func CreateJoinSyncPacket(p JoinSyncPacket) (NetPacket, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return NetPacket{PacketType: d2netpackettype.JoinSync}, err
	}

	return NetPacket{PacketType: d2netpackettype.JoinSync, PacketData: b}, nil
}

// UnmarshalJoinSync decodes a JoinSync packet's data.
func UnmarshalJoinSync(b []byte) (p JoinSyncPacket, err error) {
	err = json.Unmarshal(b, &p)

	return p, err
}
