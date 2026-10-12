package d2daynight

import "math"

// Screen flash and scene darkening (ENVIRON_GetSpecialLevelLight 0x467770, ENVIRON_GetFlashLightColor
// 0x467700, ENVIRON_UpdateAmbientLight 0x466690, ENVIRON_InitScreenEffectState 0x4667e0). All three
// replace the base light of the light map while they run. Verified in the decompile and the x87
// disassembly; the triggers (which quest scene event or packet starts them) are only partly traced
// and nothing here is wired to a trigger yet (UNVERIFIED).

// Cos512 is MATH_CosLookup512 (0x406b40): the exe's 512-entry sine table read at index
// (x + 0x80) & 0x1ff, i.e. cos(2 pi x / 512).
func Cos512(x int) float64 {
	return math.Sin(2 * math.Pi * float64((x+0x80)&0x1ff) / 512)
}

// Level 8 flash: a red flash in level 8 while the scene event counter runs 0..29 (then the
// event spawns its missiles). The colour is (255, 64, 48) and the intensity 80 * cos512((n*128)/30)
// truncated, so it decays from 80 to 0 in a quarter wave. A counter of -1 (not started) gives the
// constant 80.
const (
	FlashLevel8Ticks = 30
	FlashR           = 0xff
	FlashG           = 0x40
	FlashB           = 0x30
)

// FlashLevel8Intensity is the flash intensity for the scene counter n (-1 = not started).
func FlashLevel8Intensity(n int) int {
	if n == -1 {
		return 80
	}

	return int(80 * Cos512(n*128/FlashLevel8Ticks))
}

// FlashLevel107Intensity is the flash of levels 0x6b and 0x6c while their flag is set and the
// counter is -1: a constant 0xa0 in the same colour (otherwise the level has no flash and the
// normal base light applies).
func FlashLevel107Intensity() int { return 0xa0 }

// ScreenEffect is the darkening envelope of ENVIRON_UpdateAmbientLight: the base intensity is
// multiplied by cos512 over a fade-in, held at 0 and brought back over a fade-out.
type ScreenEffect struct {
	fadeIn, total, fadeOut int
	tick, phase            int
}

// NewScreenEffect starts the effect (ENVIRON_InitScreenEffectState): fadeIn ticks down, hold ticks
// black, fadeOut ticks back up; a fadeOut of 0 becomes 0x19. fadeIn must be above 0 (the exe aborts
// otherwise).
func NewScreenEffect(fadeIn, hold, fadeOut int) *ScreenEffect {
	if fadeOut == 0 {
		fadeOut = 0x19
	}

	return &ScreenEffect{fadeIn: fadeIn, total: fadeIn + hold + fadeOut, fadeOut: fadeOut}
}

// Step runs one game tick and returns the intensity for base. The phase used is the one computed
// on the previous tick (the exe reads it before updating it). done reports that the effect is over
// (the exe clears its state when the counter passes the total).
func (e *ScreenEffect) Step(base int) (intensity int, done bool) {
	switch {
	case e.tick < e.fadeIn:
		intensity = int(float64(base) * Cos512(e.phase))
		e.phase = e.tick * 128 / e.fadeIn
	case e.tick < e.total-e.fadeOut:
		intensity = 0
	default:
		intensity = int(float64(base) * Cos512(e.phase))
		e.phase = 0x80 - (e.fadeOut-e.total+e.tick)*0x80/e.fadeOut
	}

	e.tick++

	return intensity, e.tick > e.total
}
