package d2player

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
)

// statRepairDurability is item_replenish_durability (ItemStatCost id 252): "repairs durability" of Ethereal
// Edge style items. The step is ITEMMODE_ServerReplenishStatStep 0x560a70 (shared with stat 253): the durability
// (stat 0x48) rises by 1 up to its maximum, the next step comes max(125, 2500/value+1) frames later, and when the
// new value is above 0 and the item flag 0x100 (broken) is set, ITEM_RestoreBrokenItem 0x55d710 runs (VERIFIED).
const statRepairDurability = 252

// repairState remembers which worn items were broken at the last look and the stat 252 timers.
type repairState struct {
	broken map[d2equip.Loc]bool
	acc    map[d2equip.Loc]float64
}

// restoredLocs returns the locations that were broken before and are not any more. In the exe the flag 0x100 is
// cleared by ITEM_RestoreBrokenItem when the durability rises above 0 (NPC repair, stat 252); the Go engine
// derives "broken" from durability 0, so what is left to port is the re-adding of the stat list, i.e. a recalc.
func restoredLocs(prev, now map[d2equip.Loc]bool) []d2equip.Loc {
	var out []d2equip.Loc

	for l := d2equip.Loc(1); l < d2equip.NumLocs; l++ {
		if prev[l] && !now[l] {
			out = append(out, l)
		}
	}

	return out
}

// repairStep is one step of ITEMMODE_ServerReplenishStatStep for durability: the new value (never above max).
func repairStep(cur, maxDur int, value int64) (int, bool) {
	if value <= 0 || maxDur <= 0 || cur+1 > maxDur {
		return cur, false
	}

	return cur + 1, true
}

func (g *GameControls) wornBroken() map[d2equip.Loc]bool {
	out := map[d2equip.Loc]bool{}

	if g.inventory == nil {
		return out
	}

	for l := d2equip.Loc(1); l < d2equip.NumLocs; l++ {
		if it, ok := g.inventory.WornAt(l).(*diablo2item.Item); ok && it.IsBroken() {
			out[l] = true
		}
	}

	return out
}

// advanceRepair applies the stat 252 timers of the worn items and, when a broken worn item was repaired (by
// that stat or by an NPC), recalculates the hero's totals so its properties come back.
func (g *GameControls) advanceRepair(elapsed float64) {
	if g.inventory == nil || g.hero == nil || g.hero.Stats == nil {
		return
	}

	if g.repair.acc == nil {
		g.repair.acc = map[d2equip.Loc]float64{}
	}

	if g.hero.Stats.Health > 0 {
		for l := d2equip.Loc(1); l < d2equip.NumLocs; l++ {
			it, ok := g.inventory.WornAt(l).(*diablo2item.Item)
			if !ok {
				continue
			}

			every := replenishSeconds(int64(it.StatTotal(statRepairDurability)))
			if every <= 0 {
				continue
			}

			cur, maxDur := it.Durability()
			if cur >= maxDur {
				g.repair.acc[l] = 0

				continue
			}

			for g.repair.acc[l] += elapsed; g.repair.acc[l] >= every; g.repair.acc[l] -= every {
				n, ok := repairStep(cur, maxDur, 1)
				if !ok {
					break
				}

				cur = n
				it.SetDurability(cur)
			}
		}
	}

	now := g.wornBroken()

	if len(restoredLocs(g.repair.broken, now)) > 0 {
		g.Infof("DURABILITY restored broken worn item: %s", g.afterEquipChange("repair"))
	}

	g.repair.broken = now
}
