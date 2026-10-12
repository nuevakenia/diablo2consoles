package d2level

import (
	"errors"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

// VERIFIED w37 (0x582700): the portal rule reads bit 0 only, skips the hero's own portal, refuses a missing record.
func TestCheckPortalGate(t *testing.T) {
	to := 131

	slot := LevelQuestFlag(to, true)
	if slot == 0 {
		t.Skip("no QuestFlagEx for level 131 in the tables")
	}

	if err := CheckPortalGate(109, to, true, false, &d2s.QuestRecord{}); !errors.Is(err, ErrLevelGated) {
		t.Errorf("another hero's portal into a locked level: %v", err)
	}

	if err := CheckPortalGate(109, to, true, true, &d2s.QuestRecord{}); err != nil {
		t.Errorf("the hero's own portal is not tested: %v", err)
	}

	pending := &d2s.QuestRecord{}
	pending.Set(slot, d2s.QuestBitRewardPending)

	if err := CheckPortalGate(109, to, true, false, pending); err == nil {
		t.Error("the reward-pending bit must not open a portal (bit 0 only)")
	}

	done := &d2s.QuestRecord{}
	done.Set(slot, d2s.QuestBitDone)

	if err := CheckPortalGate(109, to, true, false, done); err != nil {
		t.Errorf("bit 0 set: %v", err)
	}

	if err := CheckPortalGate(109, to, true, false, nil); err == nil {
		t.Error("a missing record refuses")
	}

	if err := CheckPortalGate(109, 113, true, false, &d2s.QuestRecord{}); err != nil {
		t.Errorf("an ungated level: %v", err)
	}
}
