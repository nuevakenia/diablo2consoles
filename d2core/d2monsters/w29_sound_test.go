package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestInitPlans(t *testing.T) {
	if got := initPlans(nil); got != nil {
		t.Errorf("nil record: %v", got)
	}

	if got := initPlans(&d2records.MonsterSoundRecord{}); got != nil {
		t.Errorf("no Init: %v", got)
	}

	got := initPlans(&d2records.MonsterSoundRecord{Init: "swarm_loop"})
	if len(got) != 1 || got[0].kind != "init" || got[0].handle != "swarm_loop" {
		t.Errorf("Init plan: %v", got)
	}
}

func TestImpactPlans(t *testing.T) {
	for _, tc := range []struct {
		class int
		ids   []int
	}{
		{0, nil},
		{0xc, []int{0x145}},         // zombie: body layer only
		{0x21, []int{0x141, 0x164}}, // both layers
		{0x50, []int{0x178}},
	} {
		got := impactPlans(&d2records.MonStat2Record{HitClass: tc.class})
		if len(got) != len(tc.ids) {
			t.Errorf("class %#x: got %v want ids %v", tc.class, got, tc.ids)
			continue
		}

		for i, p := range got {
			if p.id != tc.ids[i] || p.kind != "impact" {
				t.Errorf("class %#x [%d]: %+v want id %#x", tc.class, i, p, tc.ids[i])
			}
		}
	}

	if impactPlans(nil) != nil {
		t.Error("nil stat2")
	}
}
