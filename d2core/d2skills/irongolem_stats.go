package d2skills

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2stats"
)

// golemItemMods turns the stat list of the item an Iron Golem consumes into minion stat mods (wave 22).
// SRVDO_057 (0x5c3100, VERIFIED) equips the item on the golem through INV_ServerEquipItem, so the golem
// carries the item's properties like any equipped item does. The port feeds them to the golem's stat build
// (d2summon.Compute): single-valued stats keep their ItemStatCost name; maxhp is whole points on an item and
// 8.8 on a stat list (the build divides flat life by 256), so it is scaled up. Stats with two or more values
// (damage ranges, procs, charges) are skipped. UNVERIFIED: the golem's own weapon damage from the item's
// base damage (the build keeps the monster template's), and which equipped-item stats the golem stat list
// ignores.
func golemItemMods(list d2stats.StatList) []d2skill.StatMod {
	if list == nil {
		return nil
	}

	var out []d2skill.StatMod

	for _, st := range list.Stats() {
		if st == nil {
			continue
		}

		vals := st.Values()
		if len(vals) != 1 || vals[0].NumberType() != d2stats.StatValueInt {
			continue
		}

		name := strings.ToLower(st.Name())
		v := vals[0].Int()

		if name == "maxhp" {
			v <<= 8
		}

		out = append(out, d2skill.StatMod{Stat: name, Value: v})
	}

	return out
}

// groundItemMods reads the stat mods of the ground item entity id (before the golem consumes it).
func (e *Engine) groundItemMods(id string) []d2skill.StatMod {
	if e.mapEngine == nil || id == "" {
		return nil
	}

	it, ok := e.mapEngine.Entities()[id].(*d2mapentity.Item)
	if !ok || it.Item == nil {
		return nil
	}

	return golemItemMods(it.Item.StatList())
}
