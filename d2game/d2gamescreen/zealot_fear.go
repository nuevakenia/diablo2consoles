package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// zealotAfraid is the Director's ZealotAfraid hook (MONAI_Think_ZakarumZealot 0x5f5ef0): the local hero is feared when
// the level is in Act III and the hero's quest record has bit 0 of quest slot 21 (QUESTREC_GetFlag(rec, 0x15, 0),
// the Blackened Temple) for the current difficulty.
func (v *Game) zealotAfraid(p *d2mapentity.Player) bool {
	if p != v.localPlayer || v.questRT == nil || d2level.ActOfLevel(v.currentLevel()) != 3 {
		return false
	}

	slot, ok := d2s.QuestSlot(3, 5)

	return ok && slot == 21 && v.questBits(slot)&(1<<d2s.QuestBitDone) != 0
}
