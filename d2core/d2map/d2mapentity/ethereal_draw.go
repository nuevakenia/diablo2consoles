package d2mapentity

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

// Ethereal equipment draw mode, the equipment rule read in the draw mode chooser at 0x4d8090 (notes
// w22-render): a worn ethereal (item flag 0x400000) right-hand or left-hand weapon draws at mode 1
// (50 percent, d2enum.DrawEffectPctTransparency50) and an ethereal item in the shield slot at mode 2
// (d2enum.DrawEffectPctTransparency75). Mode numbers follow the d2enum.DrawEffect order, as for the
// missile modes. The Items.txt transparent rule and the
// unit stat 0xb5 rule are in transparency_draw.go (the Items.txt rule is not wired to items yet).

// EtherealLayerEffect returns the draw effect override of an equipment layer.
func EtherealLayerEffect(layer d2enum.CompositeType, ethereal bool) (d2enum.DrawEffect, bool) {
	if !ethereal {
		return 0, false
	}

	switch layer {
	case d2enum.CompositeTypeRightHand, d2enum.CompositeTypeLeftHand:
		return d2enum.DrawEffectPctTransparency50, true
	case d2enum.CompositeTypeShield:
		return d2enum.DrawEffectPctTransparency75, true
	}

	return 0, false
}

// equipmentEffects collects the draw effect overrides of the worn equipment.
func equipmentEffects(eq *d2inventory.CharacterEquipment) map[d2enum.CompositeType]d2enum.DrawEffect {
	if eq == nil {
		return nil
	}

	out := map[d2enum.CompositeType]d2enum.DrawEffect{}

	add := func(layer d2enum.CompositeType, ethereal bool) {
		if eff, ok := EtherealLayerEffect(layer, ethereal); ok {
			out[layer] = eff
		}
	}

	if eq.RightHand != nil {
		add(d2enum.CompositeTypeRightHand, eq.RightHand.Ethereal)
	}

	if eq.LeftHand != nil {
		add(d2enum.CompositeTypeLeftHand, eq.LeftHand.Ethereal)
	}

	if eq.Shield != nil {
		add(d2enum.CompositeTypeShield, eq.Shield.Ethereal)
	}

	return out
}
