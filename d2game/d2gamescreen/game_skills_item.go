package d2gamescreen

import (
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/d2drop"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// castTestItemCode is the base item the scenario drops for a skill that targets a ground item (Iron Golem): a
// hand axe, a weapon, so the engine's golem-item test (weapon or armor, UNVERIFIED in the exe) passes.
const castTestItemCode = "hax"

// castTestIronGolem is the skills.txt name of the skill that needs the ground item.
const castTestIronGolem = "IronGolem"

// itemForCast finds the ground item nearest to the hero for Iron Golem (it consumes the item). With none
// lying around it drops one next to the hero and asks the caller to wait one tick (the same pattern as corpseForCast).
func (v *Game) itemForCast() (int, int, bool) {
	px, py := v.localPlayer.GetPositionF()

	var (
		best *d2mapentity.Item
		bd   = math.MaxFloat64
	)

	for _, ent := range v.gameClient.MapEngine.Entities() {
		it, ok := ent.(*d2mapentity.Item)
		if !ok || it.IsGold() || it.Item == nil {
			continue
		}

		// only what a golem can wear (the engine's test, groundItemNear): a potion a zombie dropped is no target
		if kind := it.Item.GetInventoryItemType(); kind != d2enum.InventoryItemTypeArmor && kind != d2enum.InventoryItemTypeWeapon {
			continue
		}

		x, y := it.GetPositionF()
		if d := math.Hypot(x-px, y-py); d < bd {
			best, bd = it, d
		}
	}

	if best != nil {
		return int(best.Position.X()), int(best.Position.Y()), true
	}

	cells := v.freeDropCells(int(math.Floor(px)), int(math.Floor(py)), 3, true)
	if len(cells) == 0 {
		return 0, 0, false
	}

	it, err := v.itemFactory().ItemFromCode(castTestItemCode, d2drop.QualityNormal, v.areaLevel(), 4242)
	if err != nil {
		v.Warningf("AUTOCAST: cannot make a ground item: %v", err)

		return 0, 0, false
	}

	it.Identify()

	if _, err = v.spawnGroundItem(it, cells[len(cells)-1]); err != nil {
		v.Warningf("AUTOCAST: cannot drop the ground item: %v", err)

		return 0, 0, false
	}

	v.Infof("AUTOCAST making a ground item (%s) for a skill that targets one", castTestItemCode)

	return 0, 0, false
}
