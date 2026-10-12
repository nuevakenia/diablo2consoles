package d2maprenderer

// Animated floor tiles (lava; water is not animated this way), DRLG_MarkPresetTileRecordsFlagged
// 0x670090 / DRLGANIM_CreateTileRecordsFromLayer 0x670120 (w37). Every animated tile keeps a position in
// 8.8 fixed point over its n frames (the DT1 variants ordered by their rarity value); once per client world
// tick (GAME_ClientWorldTick 0x447c00) the position advances by the speed and wraps at n*256; the frame
// shown is position>>8. The speed is the layer value of the room's preset info, and 0x50 (80, i.e. 0.3125
// frames per tick) when that is 0. UNVERIFIED: that the client world tick runs at 25 Hz, and where the layer
// speed comes from (the three layer speeds sit at +0x14/+0x1c/+0x24 of the room's preset info).
const (
	// tileAnimDefaultSpeed is the exe default speed (8.8 frames per tick).
	tileAnimDefaultSpeed = 0x50
	// tileAnimTickSeconds is the client world tick (25 Hz, UNVERIFIED).
	tileAnimTickSeconds = 1.0 / 25.0
)

// tileAnimFrame is the frame shown after ticks world ticks for a tile of frames frames.
func tileAnimFrame(ticks, speed, frames int) int {
	if frames <= 0 {
		return 0
	}

	if speed == 0 {
		speed = tileAnimDefaultSpeed
	}

	return ((ticks * speed) % (frames << 8)) >> 8
}
