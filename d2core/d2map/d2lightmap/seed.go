package d2lightmap

// UnitSeed is the per-unit random generator the missile light flicker rolls with (the unit seed
// advance read in MISSILE_UpdateLightRadiusRandom 0x4c9c10, VERIFIED in the notes w25-lighting):
// seed = lo * 0x6ac690c5 + hi (64 bit), the new low half is the roll. Every unit has its own seed,
// so two missiles flicker out of step.
type UnitSeed struct{ Lo, Hi uint32 }

// NewUnitSeed makes a seed from a 64 bit value (low half first).
func NewUnitSeed(v uint64) UnitSeed { return UnitSeed{Lo: uint32(v), Hi: uint32(v >> 32)} }

// Roll advances the seed and returns a number in [0, n): the new low half masked for a power of two,
// otherwise modulo n; 0 (without advancing) for n <= 0.
func (s *UnitSeed) Roll(n int) int {
	if n <= 0 {
		return 0
	}

	v := uint64(s.Lo)*0x6ac690c5 + uint64(s.Hi)
	s.Lo, s.Hi = uint32(v), uint32(v>>32)

	if n&(n-1) == 0 {
		return int(s.Lo & uint32(n-1))
	}

	return int(s.Lo % uint32(n))
}

// FlickerLight is the light of one missile: the current radius ramps by RampPerTick toward a target
// that MissileFlickerRadius re-rolls every tick (client tick, 25 Hz).
type FlickerLight struct {
	Seed UnitSeed
	// Cur and Target are radii in 1/8 subtile (the Source units).
	Cur, Target int
	started     bool
}

// Tick runs one 25 Hz step for a missile with Missiles.txt Light and Flicker values: the flicker
// update (only when the radius is at least Light), then the ramp. The first tick starts the radius
// at Light (the light is created fully grown). flags are the missile flags (+0x44).
func (f *FlickerLight) Tick(flags uint32, light, flicker int) {
	if !f.started {
		r, ok := ClampSourceRadius(light)
		if !ok {
			return
		}

		f.Cur, f.Target, f.started = r<<fixedShift, r<<fixedShift, true
	}

	if t, ok := MissileFlickerRadius(flags, f.Cur>>fixedShift, light, flicker, f.Seed.Roll); ok {
		f.Target = t << fixedShift
	}

	switch {
	case f.Cur < f.Target:
		f.Cur += RampPerTick
	case f.Cur > f.Target:
		f.Cur -= RampPerTick
	}
}

// Subtiles is the current radius in whole subtiles (rounded up, so a ramping light never reads 0).
func (f *FlickerLight) Subtiles() int { return (f.Cur + 1<<fixedShift - 1) >> fixedShift }
