package d2enum

// Draw mode 6 (PL2 table G, PL2 offset 0x5b500, 64 KiB) is neither an add, a multiply nor a screen.
// Measured on a real Act 1 pl2 (analysis only, nothing of the file is stored here): with the
// table indexed (first << 8 | second), the result is the first palette entry moved toward the
// second by a weight that depends only on the brightness of the second one: about 0.1 for the
// darkest, 0.5 near 45% brightness, 1.0 (the second wins) from 85% up, and an equal pair
// (first == second) maps to itself. So mode 6 is "alpha = brightness of the drawn pixel": dark
// pixels of the sprite almost vanish, bright ones draw fully (a glow without additive
// saturation).
//
// UNVERIFIED: (1) which operand of the table the exe calls the source (the pixel being drawn is
// assumed); (2) the exact weight curve: Mode6Weight is a linear fit w = 1.2 * luma, clamped, of
// the measured bucket means (the fit is within about 0.1 of every bucket).
const (
	Mode6LumaR = 0.30
	Mode6LumaG = 0.59
	Mode6LumaB = 0.11

	// Mode6Gain scales the luma into the blend weight.
	Mode6Gain = 1.2
)

// Mode6Weight is the weight of the drawn pixel (r, g, b in 0..1) when blended over the
// destination in draw mode 6.
func Mode6Weight(r, g, b float64) float64 {
	w := Mode6Gain * (Mode6LumaR*r + Mode6LumaG*g + Mode6LumaB*b)

	switch {
	case w < 0:
		return 0
	case w > 1:
		return 1
	default:
		return w
	}
}
