package d2mapentity

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

// MissileDrawEffect maps the Trans column of Missiles.txt to the draw mode the
// original uses for a missile cel. Ported from GFX_DrawStaticCelUnit (0x46dab0):
// Trans 1 selects mode 3 (additive-like PL2 table), Trans 2 mode 4
// (multiplicative table), anything else mode 5 (normal, opaque pixels). Mode
// numbers equal the d2enum.DrawEffect order. The hover highlight (mode 7,
// DrawEffectMod2X) overrides all of them and is not decided here.
func MissileDrawEffect(trans int) d2enum.DrawEffect {
	switch trans {
	case 1:
		return d2enum.DrawEffectModulate
	case 2:
		return d2enum.DrawEffectBurn
	default:
		return d2enum.DrawEffectNormal
	}
}
