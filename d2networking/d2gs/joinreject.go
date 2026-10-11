package d2gs

import "fmt"

// S2CJoinReject is the packet a server sends before it drops a client whose
// join it refuses: SCMD_SendOpB4 (0x538d80) builds [0xB4][u32 code], 5 bytes
// (table entry 0xb4: 5, verified). The 0x68 join handler (CCMD_ProcessLobbyControlPacket
// 0x53ce40) gets the code from GAME_ValidateJoinNameInGame (0x53cd50) and
// then disconnects the client (NETSRV_HandleClientDisconnectRequest).
const S2CJoinReject byte = 0xb4

// Join refusal codes, the return values of GAME_ValidateJoinNameInGame
// (verified): the game is unknown or the hero name is already in it (6), the
// game already holds 8 players (0xF), the client's version field is not 0xE
// (0x10). The version byte is 14 for 1.14.
const (
	JoinErrNameOrGame uint32 = 6
	JoinErrFull       uint32 = 0xf
	JoinErrVersion    uint32 = 0x10
)

// GameVersion is the version the join must carry (the exe compares the field
// of the 0x68 packet with 0xE; in OUR join layout it is byte 20).
const GameVersion uint8 = 14

// MaxPlayers is the size limit of a game (SERVER_AddClientToGame 0x537470
// refuses a ninth client; verified).
const MaxPlayers = 8

// JoinReject is S2C 0xb4.
type JoinReject struct{ Code uint32 }

// Marshal returns the packet.
func (m JoinReject) Marshal() []byte {
	b := newW(S2CJoinReject, 5)
	b.u32(1, m.Code)

	return b
}

// ParseJoinReject decodes a 0xb4 packet.
func ParseJoinReject(b []byte) (JoinReject, error) {
	if len(b) != 5 || b[0] != S2CJoinReject {
		return JoinReject{}, ErrWrongSize
	}

	return JoinReject{Code: rbuf(b).u32(1)}, nil
}

// Reason describes a refusal code for the player.
func (m JoinReject) Reason() string {
	switch m.Code {
	case JoinErrNameOrGame:
		return "the game does not exist or a hero with that name is already in it"
	case JoinErrFull:
		return "the game is full"
	case JoinErrVersion:
		return "your game version does not match the host's"
	}

	return fmt.Sprintf("the host refused the join (code %d)", m.Code)
}
