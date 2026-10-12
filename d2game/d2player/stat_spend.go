package d2player

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

// Spending attribute points on the character screen. VERIFIED in Game.exe (wave 37):
// UI_HandleCharacterPanelMouseDown 0x4a3f80 (the mouse-down handler of the panel, which also owns the four +
// buttons; 0x4a2db0 and 0x4a2ef0 are the bottom-bar "New Stats" button handlers, not these).
//
//   - Only the Shift key is looked at (GetKeyState(VK_SHIFT) < 0). A plain click spends ONE point. Shift spends ALL
//     the remaining points. Control has no effect. There is no hold-to-repeat: the handler runs once per mouse-down.
//   - The points go out in packets of at most 32 (loop: n = min(left, 32); left -= n). Each packet is opcode 0x3a
//     followed by one little-endian word: low byte = attribute id, high byte = n-1.
//   - Attribute ids in the 0x723c90 button table order (row stride 14 bytes): the button rows top to bottom are
//     strength 0, dexterity 2, vitality 3, energy 1.
//   - Server side (SERVER_HandleC2S3A_AllocateStatPoints 0x549b40): the packet is 3 bytes, attribute id must be <= 15,
//     n-1 must be <= 99, then the server adds one point per count and stops at the first refusal.
const (
	statSpendPacketMax = 32   // most points one 0x3a packet carries (VERIFIED 0x4a4100)
	statSpendOpcode    = 0x3a // client packet: allocate stat points
)

// Attribute ids of the 0x3a packet (VERIFIED, table 0x723c90 low words).
const (
	StatAllocStrength  = 0
	StatAllocEnergy    = 1
	StatAllocDexterity = 2
	StatAllocVitality  = 3
)

// StatSpendCount returns how many points one click on an add-stat button spends: 1, or every remaining point
// while Shift is held. Control is ignored (VERIFIED). Never more than remain.
func StatSpendCount(mod d2enum.KeyMod, remaining int) int {
	if remaining <= 0 {
		return 0
	}

	if mod&d2enum.KeyModShift != 0 {
		return remaining
	}

	return 1
}

// StatSpendPackets splits n points into the per-packet counts the client sends (32 at most each).
func StatSpendPackets(n int) []int {
	var out []int

	for n > 0 {
		c := n
		if c > statSpendPacketMax {
			c = statSpendPacketMax
		}

		out = append(out, c)
		n -= c
	}

	return out
}

// StatAllocPacket encodes one 0x3a packet (opcode and word) for count points on attribute id.
func StatAllocPacket(id, count int) [3]byte {
	w := uint16(id&0xff) | uint16((count-1)&0xff)<<8

	return [3]byte{statSpendOpcode, byte(w), byte(w >> 8)}
}

// StatAllocServerAccepts mirrors the server's range checks (0x549b40): attribute id <= 15 and 1..100 points.
func StatAllocServerAccepts(id, count int) bool {
	return id >= 0 && id <= 15 && count >= 1 && count <= 100
}
