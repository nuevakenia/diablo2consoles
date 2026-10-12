package d2hero

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

func TestJoinSyncPacketSizes(t *testing.T) {
	var rec d2s.QuestRecord

	slot, _ := d2s.QuestSlot(1, 1)
	rec.Set(slot, d2s.QuestBitDone)

	j := PlanJoinSync(&rec, 2)

	if n := len(j.EncodeActive()); n != 38 {
		t.Fatalf("active packet %d bytes, want 38", n)
	}

	if n := len(j.EncodeRecord(1)); n != 103 {
		t.Fatalf("record packet %d bytes, want 103", n)
	}

	if q := j.EncodeQuestInfo(); len(q) != 97 || q[0] != 0x29 || q[1+2*slot] != 1 {
		t.Fatalf("quest info packet wrong: len %d", len(q))
	}

	if b := j.EncodeQuest1(); len(b) != 2 || b[0] != 0x89 {
		t.Fatalf("0x89 packet missing: %v", b)
	}

	if j.RevealLevel != 40 {
		t.Fatalf("act 2 reveals level %d, want 40", j.RevealLevel)
	}

	if PlanJoinSync(&d2s.QuestRecord{}, 1).EncodeQuest1() != nil {
		t.Fatal("0x89 sent for a fresh hero")
	}
}

func TestMarkCompleteOnEntry(t *testing.T) {
	var rec d2s.QuestRecord

	a, _ := d2s.QuestSlot(1, 2)
	b, _ := d2s.QuestSlot(2, 1)
	c, _ := d2s.QuestSlot(2, 3)
	rec.Set(a, d2s.QuestBitDone)
	rec.Set(b, d2s.QuestBitCompleteOnEntry)
	rec.Set(c, d2s.QuestBitClosed) // not complete

	per := MarkCompleteOnEntry(&rec)

	if per[0] != 1 || per[1] != 1 {
		t.Fatalf("per act %v, want 1 and 1", per)
	}

	if !rec.Get(a, d2s.QuestBitCompleteOnEntry) || rec.Get(c, d2s.QuestBitCompleteOnEntry) {
		t.Fatal("complete-on-entry bit not set exactly on completed quests")
	}
}

func TestMercNeedsRecreate(t *testing.T) {
	if MercNeedsRecreate(nil) || MercNeedsRecreate(&MercState{Dead: true, ID: 5}) || MercNeedsRecreate(&MercState{}) {
		t.Fatal("no merc, dead merc or empty merc must not be recreated")
	}

	if !MercNeedsRecreate(&MercState{ID: 5}) {
		t.Fatal("a living merc is recreated")
	}
}
