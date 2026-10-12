package d2lightmap

// Hero and monster light changes from stat updates, ported from the 1.14b exe (w25).
//
// The stat-change callback of the client player (installed in PLAYER_InitClientPlayer 0x45c3d0, body
// at 0x45c1d0) dispatches on stat id - 0x59 through a jump table: item_lightradius (89) calls
// UNIT_SetLightRadiusFromSkill 0x45c100 and item_lightcolor (90) calls UNIT_SetLightColorFromPacked
// 0x45c170. Both are VERIFIED in the disassembly. The stat value handed to the callback is taken to be
// the new total (UNVERIFIED, but the radius is set absolute, base + value, so a delta would drift).

const (
	// StatLightRadius and StatLightColor are the ItemStatCost ids the callback reacts to.
	StatLightRadius = 89
	StatLightColor  = 90

	// HeroBaseRadius is the hero radius the stat adds to (the constant 0xd in 0x45c100 and in
	// PLAYER_InitClientPlayer).
	HeroBaseRadius = 13

	// MissileFlagsNoFlicker is the missile flag mask (unit +0x44) that stops the flicker update.
	MissileFlagsNoFlicker = 0x300
)

// UnitRadiusFromStat is UNIT_SetLightRadiusFromSkill: the new radius of a unit light is its base
// (13 for the hero; for a monster the larger of MonStats2 Light and the best equipped component light)
// plus the item_lightradius value. A total of 0 or less changes nothing (apply is false: the exe's
// setter ignores radii <= 0, so the old radius stays), above 0x11 it becomes 0x12. Unlike the
// ramping missile setter, this one sets current and target radius at once.
func UnitRadiusFromStat(base, statValue int) (radius int, apply bool) {
	return ClampSourceRadius(base + statValue)
}

// DecodeLightColor is UNIT_SetLightColorFromPacked: a packed stat value of 0 means white; otherwise
// red is bits 16..23 (arithmetic shift, as in the exe), green bits 8..15 and blue bits 0..7.
func DecodeLightColor(packed int32) (r, g, b uint8) {
	if packed == 0 {
		return 0xff, 0xff, 0xff
	}

	return uint8(packed >> 16), uint8(packed >> 8), uint8(packed)
}

// MissileFlickerRadius is MISSILE_UpdateLightRadiusRandom (0x4c9c10), run each client tick for a missile
// with a light: when the missile flags & 0x300 are clear and the source's current radius (in subtiles,
// radius field >> 3) is at least Missiles.txt Light, the new target radius is Light + roll(Flicker)
// subtiles, clamped by the ramping setter (values <= 0 ignored, above 0x11 -> 0x12). roll(n) is the
// unit seed roll (value in [0,n), 0 when n <= 0); it is drawn only when the update runs.
func MissileFlickerRadius(flags uint32, currentSubtiles, light, flicker int, roll func(n int) int) (target int, apply bool) {
	if flags&MissileFlagsNoFlicker != 0 || currentSubtiles < light {
		return 0, false
	}

	r := 0
	if flicker > 0 {
		r = roll(flicker)
	}

	return ClampSourceRadius(light + r)
}

// SetRadiusNow sets the current and the target radius at once (LIGHT_SetLightRadius 0x46ff10).
func (s *Source) SetRadiusNow(subtiles int) {
	r, ok := ClampSourceRadius(subtiles)
	if !ok {
		return
	}

	s.Radius = r << fixedShift
	s.Target = s.Radius
}
