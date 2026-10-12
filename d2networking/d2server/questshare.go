package d2server

import (
	"fmt"
	"sort"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
)

// onQuestShare answers the killer's report that the third Ancient died (QUEST_A5_RiteOfPassage_OnMonsterKilled
// 0x58a7b0): every other hero of the game gets its outcome from d2quest.RiteDeathOutcomes (the reward in level 120
// and for the party members in Act 5, else the goal mark). The server does not hold the quest records (they are
// client-owned in OpenDiablo2), so every hero is taken as not done here and the receiving client checks its own bit 0
// and level gate; an already finished hero does not spread to its party in the exe, which this approximation
// ignores (UNVERIFIED as harmless). The killer is paid by its own client.
func (g *GameServer) onQuestShare(client ClientConnection, packet d2netpacket.NetPacket) error {
	p, err := d2netpacket.UnmarshalQuestShare(packet.PacketData)
	if err != nil {
		return err
	}

	if p.Quest != d2quest.QuestRite {
		return nil
	}

	p.Killer = client.GetUniqueID()

	g.soc.mu.Lock()
	ids := make([]string, 0, len(g.connections))

	for id := range g.connections {
		ids = append(ids, id)
	}

	sort.Strings(ids)

	heroes := make([]d2quest.RiteHero, 0, len(ids))

	for _, id := range ids {
		m, ok := g.soc.roster.Member(id)
		if !ok {
			continue
		}

		party := ""
		if pid := g.soc.roster.PartyID(id); pid != 0 {
			party = fmt.Sprint(pid)
		}

		heroes = append(heroes, d2quest.RiteHero{ID: id, Level: m.Level, Area: m.Area, Party: party})
	}

	out := d2quest.RiteDeathOutcomes(p.Killer, p.Difficulty, heroes)
	g.soc.mu.Unlock()

	for _, id := range ids {
		o, ok := out[id]
		if !ok || id == p.Killer {
			continue
		}

		c := g.connByID(id)
		if c == nil {
			continue
		}

		res := p
		res.Player, res.Outcome = id, int(o)

		pkt, err := d2netpacket.CreateQuestSharePacket(res)
		if err != nil {
			return err
		}

		g.Infof("QUESTSHARE rite player=%q outcome=%d", g.nameOf(id), int(o))

		if err := c.SendPacketToClient(pkt); err != nil {
			g.Errorf("QuestShare to %s: %v", id, err)
		}
	}

	return nil
}
