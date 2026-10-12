package d2hero

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// TestStarterKitReplacesPlaceholderGear: CreateHeroState hands out placeholder gear (a Sorceress used to get a short
// staff in both hands); the class kit must replace it, so a new Sorceress wears one staff, in the right hand (slot 4,
// CharStats loc "rarm"), and the first export has no "left arm staff" the file cannot hold (realm hero warning).
func TestStarterKitReplacesPlaceholderGear(t *testing.T) {
	rm := &d2records.RecordManager{}
	rm.Item.Weapons = d2records.CommonItems{}
	rm.Item.Armors = d2records.CommonItems{}

	for _, c := range []string{"hax", "wnd", "ssd", "ktr", "sst", "jav", "clb"} {
		rm.Item.Weapons[c] = &d2records.ItemCommonRecord{Code: c, Name: c}
	}

	rm.Item.Armors["buc"] = &d2records.ItemCommonRecord{Code: "buc", Name: "buc"}
	rm.Character.Stats = d2records.CharStats{d2enum.HeroSorceress: &d2records.CharStatRecord{
		StartItem:         [10]string{"sst"},
		StartItemLocation: [10]string{"rarm"},
		StartItemCount:    [10]int{1},
	}}

	am := &d2asset.AssetManager{Records: rm}

	inv, err := d2inventory.NewInventoryItemFactory(am)
	if err != nil {
		t.Fatal(err)
	}

	f := &HeroStateFactory{asset: am, InventoryItemFactory: inv}
	state := &HeroState{Equipment: inv.DefaultHeroItems[d2enum.HeroSorceress]}

	// the placeholder gear of older builds: a second staff in the left hand
	state.Equipment.LeftHand = state.Equipment.RightHand

	f.wearStartingEquipment(state, d2enum.HeroSorceress)

	e := state.Equipment
	if e.RightHand == nil || e.RightHand.ItemCode != "sst" {
		t.Fatalf("right hand = %v, want the short staff", e.RightHand)
	}

	if e.LeftHand != nil || e.Shield != nil {
		t.Fatalf("left arm holds placeholder gear: hand=%v shield=%v", e.LeftHand, e.Shield)
	}

	if codes := engineSlotCodes(state); len(codes) != 1 || codes[d2sSlotRightArm] != "sst" {
		t.Fatalf("engine slots = %v, want only slot 4: sst", codes)
	}
}
