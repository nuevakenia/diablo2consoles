package d2records

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// dwBits4 bit 31 is the skills.txt interrupt column (DATATBL_LoadSkillsTable 0x613450, VERIFIED): the shipped
// melee attack and the Concentration aura have it, Zeal does not.
func TestPipelineSkillHoldBitIsInterruptColumn(t *testing.T) {
	rm := loadRealRecords(t)

	want := map[string]bool{"Attack": true, "Concentration": true, "Zeal": false}
	seen := 0

	for _, r := range rm.Skill.Details {
		w, ok := want[r.Skill]
		if !ok {
			continue
		}

		seen++

		if got := r.PipelineSkill().Bits4&d2skill.Bits4Hold != 0; got != w || r.Interrupt != w {
			t.Errorf("%s: hold bit %v, want %v", r.Skill, got, w)
		}
	}

	if seen != len(want) {
		t.Fatalf("found %d of %d skills", seen, len(want))
	}
}
