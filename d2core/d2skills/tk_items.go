package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// telekinesisItemNear is the ground item (gold included) nearest (sx, sy) within pickRadius for Telekinesis, or
// nil. Pick is the exe's item type test of SRVDO_Telekinesis (0x5c79f0): scroll, gold, thrown potion, missile
// potion type 0x38, potion or key, each through the item type's equiv chain. dist is the Chebyshev distance of the
// item to the click, so the caller can tell whether it is nearer than the unit under the cursor.
func (e *Engine) telekinesisItemNear(sx, sy int) (it *d2skill.TKItem, dist int) {
	if e.mapEngine == nil {
		return nil, 0
	}

	bd := pickRadius + 1

	var best *d2skill.TKItem

	for id, ent := range e.mapEngine.Entities() {
		item, ok := ent.(*d2mapentity.Item)
		if !ok || (item.Item == nil && !item.IsGold()) {
			continue
		}

		x, y := int(item.Position.X()), int(item.Position.Y())

		d := chebyshev(x-sx, y-sy)
		if d > bd || (d == bd && best != nil && id >= best.ID) {
			continue
		}

		bd = d
		best = &d2skill.TKItem{ID: id, X: x, Y: y, Pick: item.IsGold() || e.telekinesisType(item.Item.GetItemCode())}
	}

	return best, bd
}

// telekinesisType says whether an item code is of one of d2skill.TelekinesisPickTypes.
func (e *Engine) telekinesisType(code string) bool {
	if e.asset == nil || e.asset.Records == nil {
		return false
	}

	rec := e.asset.Records.Item.All[code]
	if rec == nil {
		return false
	}

	for _, want := range d2skill.TelekinesisPickTypes {
		if d2skill.TypeIs(e.equivOf, rec.Type, want) {
			return true
		}
	}

	return false
}
