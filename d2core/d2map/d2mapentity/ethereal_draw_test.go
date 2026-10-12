package d2mapentity

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

func TestEtherealLayerEffect(t *testing.T) {
	tests := []struct {
		layer d2enum.CompositeType
		eth   bool
		want  d2enum.DrawEffect
		ok    bool
	}{
		{d2enum.CompositeTypeRightHand, true, d2enum.DrawEffectPctTransparency50, true},
		{d2enum.CompositeTypeLeftHand, true, d2enum.DrawEffectPctTransparency50, true},
		{d2enum.CompositeTypeShield, true, d2enum.DrawEffectPctTransparency75, true},
		{d2enum.CompositeTypeRightHand, false, 0, false},
		{d2enum.CompositeTypeTorso, true, 0, false},
		{d2enum.CompositeTypeHead, true, 0, false},
	}

	for _, tc := range tests {
		got, ok := EtherealLayerEffect(tc.layer, tc.eth)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("%v eth=%v: got (%v,%v) want (%v,%v)", tc.layer, tc.eth, got, ok, tc.want, tc.ok)
		}
	}

	// the mode numbers of the exe: weapon mode 1, shield mode 2
	if int(d2enum.DrawEffectPctTransparency50) != 1 || int(d2enum.DrawEffectPctTransparency75) != 2 {
		t.Error("draw effect order changed")
	}
}

func TestEquipmentEffects(t *testing.T) {
	if equipmentEffects(nil) != nil {
		t.Error("nil equipment")
	}

	eq := &d2inventory.CharacterEquipment{
		RightHand: &d2inventory.InventoryItemWeapon{Ethereal: true},
		LeftHand:  &d2inventory.InventoryItemWeapon{},
		Shield:    &d2inventory.InventoryItemArmor{Ethereal: true},
	}

	got := equipmentEffects(eq)
	if len(got) != 2 || got[d2enum.CompositeTypeRightHand] != d2enum.DrawEffectPctTransparency50 ||
		got[d2enum.CompositeTypeShield] != d2enum.DrawEffectPctTransparency75 {
		t.Errorf("got %v", got)
	}
}
