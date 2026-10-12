package d2lightmap

// Light of a state overlay (cold, fire, shocked...): GFX_CreateStateEffect 0x46be50 (w37). When the
// Overlay.txt row has Radius != 0 the effect creates a light with the row's InitRadius, intensity 0xff and
// the row's Red/Green/Blue (LIGHT_CreateLightSource 0x46fdc0: radius in subtiles, > 0x11 becomes 0x12, a
// radius <= 0 creates nothing, current and target start at radius*8), and when Radius != InitRadius it
// sets the target to Radius through LIGHT_SetRadiusClamped 0x46fed0 (same clamp). The ordinary light tick
// (LIGHT_UpdateLightSource 0x471280) then moves the current radius toward the target by 8 per tick, i.e.
// one subtile per 25 Hz tick, so the light grows or shrinks from InitRadius to Radius.
// UNVERIFIED: the light kind the effect passes (the decompile hides the argument), and the Overlay.txt
// states each skill or monster effect uses (not wired to entity states yet).

// NewStateLight builds the light source of a state overlay; ok is false when the overlay has no light.
func NewStateLight(radius, initRadius int, r, g, b uint8) (*Source, bool) {
	if radius == 0 || initRadius <= 0 {
		return nil, false
	}

	const maxCreateRadius = 0x12

	initRadius = min(initRadius, maxCreateRadius)
	s := NewSource(initRadius, SourceIntensity, r, g, b)

	if radius != initRadius && radius > 0 {
		s.SetTargetRadius(min(radius, maxCreateRadius))
	}

	return s, true
}
