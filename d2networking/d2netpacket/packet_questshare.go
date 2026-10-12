package d2netpacket

import (
	"encoding/json"

	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// QuestSharePacket carries the shared quest state of Rite of Passage (the third Ancient died: 0x58a7b0 with the
// per-hero callbacks 0x58a600 / 0x58a5a0 and the party callback 0x58a4f0). The killer's client sends it with Quest
// and Difficulty; the server decides every other hero's outcome from the roster (level, area, party) and sends each
// one a packet with Player and Outcome (the d2quest.RiteOutcome value). Quest records are client-owned, so the
// receiving client checks its own bit 0 and level gate before it applies the outcome.
type QuestSharePacket struct {
	Quest      int    `json:"quest"`
	Difficulty int    `json:"difficulty,omitempty"`
	Killer     string `json:"killer,omitempty"`
	Player     string `json:"player,omitempty"`
	Outcome    int    `json:"outcome,omitempty"`
}

// CreateQuestSharePacket builds a QuestShare packet.
func CreateQuestSharePacket(p QuestSharePacket) (NetPacket, error) {
	return social(d2netpackettype.QuestShare, p)
}

// UnmarshalQuestShare decodes a QuestShare packet's data.
func UnmarshalQuestShare(b []byte) (p QuestSharePacket, err error) {
	err = json.Unmarshal(b, &p)

	return
}
