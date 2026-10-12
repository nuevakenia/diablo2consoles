package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Prison of Ice, the world side (notes: w22-quest-world.md). The frozen Anya is the object objects.txt 558 ("frozen
// anya", OperateFn 67, InitFn 74) of the Frozen River. VERIFIED in D2MOO (A5Q3.cpp) and the exe quest handler 0x587460:
//   - the "Drehya Start Outside Town" marker (object 460, InitFn 67) of the level makes the frozen block when the
//     quest is on (25 frames after the level is built);
//   - operating it with Malah's potion in the pack deletes the potion, sets the primary goal and the pending reward and
//     melts the block (it goes to its opened mode); without the potion it sends Anya's hint (message 20131).
// UNVERIFIED: the melted block is only opened here; the original later swaps it for the iced Drehya monster and moves
// her to Harrogath after Malah's scroll (the town Anya is not placed).

const objDrehyaStartOutside = 460

// anyaEnter runs when the hero arrives in a level: the Frozen River gets its block when the quest is on.
func (v *Game) anyaEnter(level int) {
	if level != d2quest.LevelFrozenRiver || v.gameClient == nil || v.gameClient.MapEngine == nil {
		return
	}

	r := v.quests()
	if r == nil {
		return
	}

	q := r.g.Quest(d2quest.QuestPrison)
	if q == nil || !q.NotIntro {
		v.Infof("ANYA the Prison of Ice is not on: no frozen Anya")

		return
	}

	if blocks := v.objectsOf(d2quest.ObjectFrozenAnya); len(blocks) > 0 {
		v.Infof("ANYA the level carries %d frozen Anya block(s) of its own", len(blocks))
		v.anyaMelted(blocks[0])

		return
	}

	markers := v.objectsOf(objDrehyaStartOutside)
	if len(markers) == 0 {
		v.Infof("ANYA the Frozen River has no Drehya marker (460): no frozen Anya placed")

		return
	}

	x, y := markers[0].GetPositionF()

	ob := v.newObjectAt(d2quest.ObjectFrozenAnya, int(x*subtilesInTile), int(y*subtilesInTile))
	if ob == nil {
		return
	}

	v.Infof("ANYA frozen Anya placed at the marker (%.1f,%.1f)", x, y)
	v.anyaMelted(ob)
}

// anyaMelted opens the block when the potion was already used (reward pending or paid).
func (v *Game) anyaMelted(ob *d2mapentity.Object) {
	r := v.quests()
	if r == nil {
		return
	}

	if q := r.g.Quest(d2quest.QuestPrison); q != nil && (r.g.Rec.Get(q.Slot, d2quest.FlagRewardPending) ||
		r.g.Rec.Get(q.Slot, d2quest.FlagRewardGranted)) {
		if _, err := ob.Open(); err != nil {
			v.Warningf("ANYA opening the melted block: %v", err)
		}
	}
}

// anyaOperate is the hero using the frozen block (OperateFn 67). It reports whether the object was the block.
func (v *Game) anyaOperate(ob *d2mapentity.Object) bool {
	if ob.Record().Index != d2quest.ObjectFrozenAnya {
		return false
	}

	r := v.quests()
	if r == nil {
		return true
	}

	v.syncQuestItems()

	if r.g.Items[d2quest.ItemDefrostPotion] <= 0 {
		// no potion: the block sends Anya's hint (message 20131) and the log moves on
		v.Infof("ANYA the frozen block is operated without Malah's potion")
		v.questTalkRef(v.refForClass(d2quest.NPCAnyaFrozen))

		return true
	}

	v.Infof("QUEST object operated id=%d name=%q", ob.Record().Index, ob.Label())
	v.questDispatch(d2quest.Event{Kind: d2quest.EvObjectOperated, Object: d2quest.ObjectFrozenAnya, Level: r.area})
	v.anyaMelted(ob)

	return true
}
