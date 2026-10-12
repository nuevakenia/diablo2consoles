package d2mapentity

import (
	"hash/fnv"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2lightmap"
)

// lightTickSeconds is the client light tick (25 Hz, the same rate the light map is rebuilt at).
const lightTickSeconds = 1.0 / 25.0

// maxLightCatchUp bounds the ticks run after a long frame.
const maxLightCatchUp = 5

// advanceLight runs the 25 Hz flicker ticks of the missile light (MISSILE_UpdateLightRadiusRandom
// 0x4c9c10 through the ramping setter). The roll comes from a seed private to the missile, derived
// from its id (the exe's unit seed is not modelled; UNVERIFIED how it is initialised).
func (m *Missile) advanceLight(elapsed float64) {
	// MISSILE_DispatchClientDoFuncAndUpdateLight 0x4cf9c0 (w37) only calls the flicker update when
	// Missiles.txt Flicker is not 0; the flag word (+0x44, UnitAny.dwFlags) bits 0x300 stop it, and
	// what sets them is not traced (UNVERIFIED, treated as 0).
	if m.record == nil || m.record.Light.Diameter <= 0 || m.record.Light.Flicker == 0 {
		return
	}

	if m.flicker.Seed == (d2lightmap.UnitSeed{}) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(m.uuid))
		m.flicker.Seed = d2lightmap.NewUnitSeed(h.Sum64() | 1)
	}

	m.lightAcc += elapsed
	if m.lightAcc > lightTickSeconds*maxLightCatchUp {
		m.lightAcc = lightTickSeconds * maxLightCatchUp
	}

	for m.lightAcc >= lightTickSeconds {
		m.lightAcc -= lightTickSeconds
		// missile flags (+0x44) are not modelled: 0 lets the flicker run
		m.flicker.Tick(0, m.record.Light.Diameter, m.record.Light.Flicker)
	}
}

// LightSource is the light of a missile: Missiles.txt Light (radius, read as subtiles; the OD2
// record names it Diameter, UNVERIFIED which it is) with the flicker of Missiles.txt Flicker
// (MISSILE_UpdateLightRadiusRandom 0x4c9c10) and Red, Green, Blue (MISSILE_CreateClientMissile
// 0x4c9f80). Before the first tick the plain Light radius is returned.
func (m *Missile) LightSource() (LightSource, bool) {
	if m.record == nil {
		return LightSource{}, false
	}

	radius := m.record.Light.Diameter
	if m.flicker.Cur > 0 {
		radius = m.flicker.Subtiles()
	}

	radius, ok := d2lightmap.ClampSourceRadius(radius)
	if !ok {
		return LightSource{}, false
	}

	return LightSource{radius, m.record.Light.Red, m.record.Light.Green, m.record.Light.Blue}, true
}
