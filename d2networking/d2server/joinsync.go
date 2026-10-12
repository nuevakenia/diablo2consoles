package d2server

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
)

// sendJoinSync sends a hero that just entered the game the join-time sync of
// QUEST_SyncPlayerOnGameEnter 0x544140 (quest active bytes, quest record, the
// quest 1 late packet) and SERVER_PlacePlayerInAct 0x536ef0 (map reveal of the
// act start level, mercenary recreate). Other players' equipment (0x9c/0x9d) already
// travels in their AddPlayer packet, which handleClientConnection sends.
func (g *GameServer) sendJoinSync(client ClientConnection) {
	st := client.GetPlayerState()
	if st == nil {
		return
	}

	act := st.Act
	if act < 1 {
		act = 1
	}

	var plan d2hero.JoinSync

	if st.Progress != nil {
		rec := st.Progress.QuestRecord(int(st.Difficulty))
		if rec != nil {
			cp := *rec
			d2hero.MarkCompleteOnEntry(&cp) // the sent copy; the hero keeps its own record
			plan = d2hero.PlanJoinSync(&cp, act)
		}
	}

	if plan.Record == nil {
		plan = d2hero.PlanJoinSync(nil, act)
	}

	pkt, err := d2netpacket.CreateJoinSyncPacket(d2netpacket.JoinSyncPacket{
		Player: client.GetUniqueID(), Sync: plan, Act: act, Merc: d2hero.MercNeedsRecreate(st.Merc),
	})
	if err != nil {
		g.Errorf("JoinSync: %v", err)
		return
	}

	if err := client.SendPacketToClient(pkt); err != nil {
		g.Errorf("GameServer: error sending JoinSync to client %s: %s", client.GetUniqueID(), err)
	}
}
