package d2player

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
)

// WeaponRangeAdder is UNIT_GetMeleeRange (0x622a30) for the hero: the rangeadder column of the wielded weapon
// (ITEM_GetRangeAdder 0x628ab0), 0 for bare hands and for most weapons (polearms and spears have 5-6 in the
// 1.14 tables). It feeds the Fend / Zeal / Frenzy reach (reach = this + 4).
func (g *GameControls) WeaponRangeAdder() int {
	if g == nil || g.inventory == nil {
		return 0
	}

	w, ok := g.inventory.WornAt(d2equip.LocRightHand).(*diablo2item.Item)
	if !ok || w == nil {
		return 0
	}

	rec := w.CommonRecord()
	if rec == nil || rec.ExtraRange < 0 {
		return 0
	}

	return rec.ExtraRange
}
