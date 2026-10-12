package d2quest

// QuestOpen is the exe's "pending quest" test of the area-entry hero voice
// (UI_HasPendingQuestForNpc 0x4a0900): none of the flags completed-now (13), reward pending (1),
// reward granted (0) and completed-before (14) is set on the quest's record. The exe also asks a
// per quest byte of packet 0x5e and, for the Den of Evil, the length of the quest log (both UNVERIFIED,
// not modelled). An unknown id is not open.
func (g *Game) QuestOpen(id int) bool {
	q := g.byID[id]
	if q == nil || g.Rec == nil {
		return false
	}

	return !g.Rec.Get(q.Slot, FlagCompletedNow) && !g.Rec.Get(q.Slot, FlagRewardPending) &&
		!g.Rec.Get(q.Slot, FlagRewardGranted) && !g.Rec.Get(q.Slot, FlagCompletedEarly)
}
