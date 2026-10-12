package d2mapentity

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2lightmap"
)

// highlightBrightness is draw mode 7, the hover highlight: the palette brightened to 170%
// (GFX_DrawStaticCelUnit 0x46dab0). It was 2.0 before the exe was read.
const highlightBrightness = d2enum.Mod2XBrightness

// LightSource is a light a unit carries: a radius in subtiles and a colour. All created
// sources have the peak intensity d2lightmap.SourceIntensity.
type LightSource struct {
	Radius  int
	R, G, B uint8
}

// Highlighted reports whether the entity is hovered this frame (set by Highlight, cleared
// when the entity renders).
func (m *mapEntity) Highlighted() bool { return m.highlight }

// Highlighted reports whether the entity is hovered this frame.
func (ae *AnimatedEntity) Highlighted() bool { return ae.highlight || ae.mapEntity.highlight }

// LightSource is the light of a living monster: MonStats2 Light, LightR, LightG, LightB
// (MONSTER_UpdateClientLightAndVisuals 0x4aa990). The exe also lights up the monsters of level 8
// (Den of Evil) with radius 3 after the quest event and counts the light of equipped
// components; neither is modelled (UNVERIFIED data in OD2).
func (m *Monster) LightSource() (LightSource, bool) {
	if m.StatEx == nil || m.dead {
		return LightSource{}, false
	}

	radius, ok := d2lightmap.ClampSourceRadius(d2lightmap.UnitLightRadius(m.StatEx.Light, 0))
	if !ok {
		return LightSource{}, false
	}

	return LightSource{radius, colorByte(m.StatEx.LightR), colorByte(m.StatEx.LightG), colorByte(m.StatEx.LightB)}, true
}

// LightSource is the light of a missile: Missiles.txt Light (radius, read as subtiles; the OD2
// record names it Diameter, UNVERIFIED which it is) and Red, Green, Blue (MISSILE_CreateClientMissile
// 0x4c9f80).
func (m *Missile) LightSource() (LightSource, bool) {
	if m.record == nil {
		return LightSource{}, false
	}

	radius, ok := d2lightmap.ClampSourceRadius(m.record.Light.Diameter)
	if !ok {
		return LightSource{}, false
	}

	return LightSource{radius, m.record.Light.Red, m.record.Light.Green, m.record.Light.Blue}, true
}

func colorByte(v int) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255: //nolint:gomnd // byte
		return 255
	default:
		return uint8(v)
	}
}
