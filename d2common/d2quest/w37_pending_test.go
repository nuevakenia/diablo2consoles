package d2quest

import "testing"

func TestQuestOpen(t *testing.T) {
	g, _ := newGame(t)

	if !g.QuestOpen(QuestDenOfEvil) {
		t.Fatal("a fresh Den of Evil is open")
	}

	slot := g.Quest(QuestDenOfEvil).Slot

	for _, bit := range []int{FlagCompletedNow, FlagRewardPending, FlagRewardGranted, FlagCompletedEarly} {
		g.Rec.Set(slot, bit)

		if g.QuestOpen(QuestDenOfEvil) {
			t.Errorf("flag %d closes the quest", bit)
		}

		g.Rec.Clear(slot, bit)
	}

	if g.QuestOpen(999) {
		t.Error("an unknown quest is not open")
	}
}
