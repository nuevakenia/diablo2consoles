package d2hero

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"

// Join-time sync of a hero (w37), read from QUEST_SyncPlayerOnGameEnter 0x544140,
// QUEST_SendQuestRecordToClient 0x5423d0, SCMD_SendPacketLen38_B5D0 0x53b5d0,
// SCMD_SendPacketLen103_B3F0 0x53b3f0 and SERVER_PlacePlayerInAct 0x536ef0.
//
// When a hero enters a game the exe (1) marks every quest the hero already
// completed with the complete-on-entry bit, (2) calls the five quest callbacks of
// the entry hook (acts 1..5 node types), and (3) sends the new client:
//   - a 38-byte packet: opcode 0x5e + one "active" byte per quest table entry
//     (the byte at +9 of the quest node; 37 entries here, the table size DAT_0072f378
//     is UNVERIFIED);
//   - a 103-byte packet: 7 header bytes + the 96-byte quest record of the
//     hero's difficulty (the buffer is filled by QUESTREC_CopyToBuffer, 0x60 bytes);
//   - the 97-byte packet 0x29 (QuestInfo): opcode + the same 96-byte record;
//   - the 2-byte packet 0x89 when the node of quest id 1 is active and its state
//     (+0xc) is above 3.
// SERVER_PlacePlayerInAct then reveals the map of the act start level (0x07), sends
// an 11-byte (0x98 family) and a 5-byte packet, recreates a mercenary whose unit
// is missing from the corpse list and re-summons the hero's current pet (skill
// 0x5a) next to it.

// Sizes of the join packets (VERIFIED, the lengths handed to SCMD_QueueServerPacket).
const (
	JoinActiveLen   = 38
	JoinRecordLen   = 103
	QuestInfoLen    = 97
	JoinActiveCount = JoinActiveLen - 1
	joinHeaderLen   = 7
	// QuestInfoOpcode is the 0x29 game quest info packet (VERIFIED 0x5423d0).
	QuestInfoOpcode = 0x29
	// ActiveQuestsOpcode is the 0x5e packet of the 38-byte message (VERIFIED 0x544140).
	ActiveQuestsOpcode = 0x5e
	// Act1QuestOneOpcode is the 2-byte 0x89 packet (VERIFIED 0x544140).
	Act1QuestOneOpcode = 0x89
)

// JoinSync is what the host sends a hero that enters the game.
type JoinSync struct {
	// Active: the 37 active bytes of the 0x5e packet (UNVERIFIED mapping from the
	// record: slot i is active when it has a done, reward or progress bit).
	Active []byte `json:"active"`
	// Record is the 96-byte quest record of the hero's difficulty.
	Record []byte `json:"record"`
	// Quest1Late: the 0x89 packet follows (UNVERIFIED: quest id 1 is Den of Evil, "state
	// above 3" is taken as completed or reward pending).
	Quest1Late bool `json:"quest1Late,omitempty"`
	// RevealLevel is the act start level whose map the host reveals (0x07).
	RevealLevel int `json:"revealLevel,omitempty"`
}

// MarkCompleteOnEntry is the first loop of 0x544140 for a hero that enters a fresh
// game: a quest slot with the done bit (0) or the complete-on-entry bit (15) set gets
// bit 15 (and the node state is reset). Only slots of real quests are touched
// (UNVERIFIED: the exe tests a per-node type byte; prologue slots are skipped here).
// It reports how many quests of each act were marked.
func MarkCompleteOnEntry(rec *d2s.QuestRecord) (perAct [d2s.NumActs]int) {
	if rec == nil {
		return
	}

	for act := 1; act <= d2s.NumActs; act++ {
		for q := 1; q <= d2s.QuestCount(act); q++ {
			slot, ok := d2s.QuestSlot(act, q)
			if !ok {
				continue
			}

			if rec.Get(slot, d2s.QuestBitDone) || rec.Get(slot, d2s.QuestBitCompleteOnEntry) {
				rec.Set(slot, d2s.QuestBitCompleteOnEntry)

				perAct[act-1]++
			}
		}
	}

	return
}

// PlanJoinSync builds the sync of a hero from its quest record. act is the act the
// hero is placed in (1..5); the start level of that act is revealed.
func PlanJoinSync(rec *d2s.QuestRecord, act int) JoinSync {
	j := JoinSync{Active: make([]byte, JoinActiveCount), Record: make([]byte, len(d2s.QuestRecord{}))}

	if rec != nil {
		copy(j.Record, rec[:])

		for i := range j.Active {
			if v := rec.Slot(i); v&(1<<d2s.QuestBitDone|1<<d2s.QuestBitRewardPending|0x0ffc) != 0 {
				j.Active[i] = 1
			}
		}

		if slot, ok := d2s.QuestSlot(1, 1); ok {
			j.Quest1Late = rec.Get(slot, d2s.QuestBitDone) || rec.Get(slot, d2s.QuestBitRewardPending)
		}
	}

	j.RevealLevel = ActStartLevel(act)

	return j
}

// ActStartLevel is the town (start level) of an act: 1, 40, 75, 103, 109 (levels.txt
// order; DRLG_GetActStartLevel is what the exe asks, 0x536ef0). 0 for an invalid act.
func ActStartLevel(act int) int {
	switch act {
	case 1:
		return 1
	case 2:
		return 40
	case 3:
		return 75
	case 4:
		return 103
	case 5:
		return 109
	}

	return 0
}

// EncodeActive is the 38-byte 0x5e packet.
func (j JoinSync) EncodeActive() []byte {
	b := make([]byte, JoinActiveLen)
	b[0] = ActiveQuestsOpcode
	copy(b[1:], j.Active)

	return b
}

// EncodeRecord is the 103-byte packet: 7 header bytes (the opcode and fields are
// UNVERIFIED, kept zero except the difficulty at byte 1) then the 96-byte record.
func (j JoinSync) EncodeRecord(difficulty int) []byte {
	b := make([]byte, JoinRecordLen)
	b[1] = byte(difficulty)
	copy(b[joinHeaderLen:], j.Record)

	return b
}

// EncodeQuestInfo is the 97-byte 0x29 packet.
func (j JoinSync) EncodeQuestInfo() []byte {
	b := make([]byte, QuestInfoLen)
	b[0] = QuestInfoOpcode
	copy(b[1:], j.Record)

	return b
}

// EncodeQuest1 is the 2-byte 0x89 packet (nil when not sent).
func (j JoinSync) EncodeQuest1() []byte {
	if !j.Quest1Late {
		return nil
	}

	return []byte{Act1QuestOneOpcode, 0}
}

// MercNeedsRecreate: SERVER_PlacePlayerInAct walks the hero's corpse list and, for
// an entry whose unit is gone (the mercenary of a hero that left a game), creates
// the mercenary again when it has a class and name id (VERIFIED 0x536ef0, loop on
// INV_GetCorpseListHead). A dead mercenary stays dead (UNVERIFIED: the corpse entry
// of a dead mercenary is the same list).
func MercNeedsRecreate(m *MercState) bool {
	return m != nil && !m.Dead && m.ID != 0
}
