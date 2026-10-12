package d2lightmap

// Source kinds, the second argument of LIGHT_CreateLightSource (0x46fdc0). Read at the call sites:
// the hero and monsters pass 0, missiles and state effects pass 1, objects pass 2.
const (
	KindUnit    = 0 // hero and monsters: line-of-sight painter at light quality 2, else radial
	KindMissile = 1 // missiles and state effects: always the radial painter
	KindObject  = 2 // objects: ray-traced occlusion painter
)

// MaxSourceRadius is the largest radius, in subtiles, a created source keeps: the constructor
// turns any value above 0x11 into 0x12 (then stores radius * 8).
const MaxSourceRadius = 0x12

// SourceIntensity is the peak intensity every created source gets: all call sites pass 0xff.
const SourceIntensity = 0xff

// ClampSourceRadius is the radius LIGHT_CreateLightSource stores for a requested radius in
// subtiles. A radius of 0 or less creates no source at all (the exe returns null); ok reports
// whether a source is made.
func ClampSourceRadius(r int) (radius int, ok bool) {
	if r <= 0 {
		return 0, false
	}

	if r > 0x11 {
		return MaxSourceRadius, true
	}

	return r, true
}

// UnitLightRadius is the radius of a monster's light (MONSTER_UpdateClientLightAndVisuals
// 0x4aa990): the larger of the MonStats2 Light column and the biggest light radius among its
// equipped components (itemRadius; UNVERIFIED that the plain monster data carries any).
func UnitLightRadius(monStats2Light, itemRadius int) int {
	if itemRadius > monStats2Light {
		return itemRadius
	}

	return monStats2Light
}

// HoverIntensity is the light byte of the hovered unit (GFX_DrawStaticCelUnit 0x46dab0 and
// GFX_DrawUnitCompositeLayers 0x46c9a0): doubled, then kept within 0x40..0xff. Together with
// draw mode 7 (the 170% palette) this is the hover highlight.
func HoverIntensity(i uint8) uint8 {
	v := int(i) * 2

	switch {
	case v <= 0x40:
		return 0x40
	case v > 0xfe:
		return 0xff
	default:
		return uint8(v)
	}
}
