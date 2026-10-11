package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// applyPlannedUMods runs the monster modifier init callbacks (MONSTER_RunUModInitCallbacks 0x59fc40, ported in
// d2monsters.ApplyUModEffects) for every rare, champion and super unique leader of a planned population: the hit
// point bonus of the rank, +3 levels and the experience, then the effect of each modifier the leader carries; the
// followers linked to it get the minion version. Units the original does not link (party packs without SetBoss) are
// left alone.
func (v *Game) applyPlannedUMods(plan []d2mapengine.PlannedMonster, made []*d2mapentity.Monster) {
	members := map[int][]*d2mapentity.Monster{}

	for j, pm := range plan {
		if pm.Leader >= 0 && pm.Leader < len(made) && made[j] != nil {
			members[pm.Leader] = append(members[pm.Leader], made[j])
		}
	}

	for i, pm := range plan {
		if made[i] == nil || pm.Minion || !(pm.Unique || pm.Champion || pm.SuperKey != "") {
			continue
		}

		v.monsters.ApplyUModEffects(made[i], members[i])
	}
}
