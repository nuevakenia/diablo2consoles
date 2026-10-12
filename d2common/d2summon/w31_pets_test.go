package d2summon

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// w31 (pets follow-up). COMBAT_RollPhysicalDamage 0x579120 (VERIFIED): a unit without a weapon adds its whole strength
// stat to the percent accumulator that damagepercent feeds, then each end of the roll is scaled by (100 + acc) / 100.
// The Valkyrie is the only summon with strength (aurastat1 = lvl * Param2 = 25 per level).
func TestW31StrengthIsDamagePercent(t *testing.T) {
	tp, err := LoadTemplates([]byte("Id\thcIdx\tAI\tVelocity\tRun\tminHP\tmaxHP\tAC\tA1MinD\tA1MaxD\tA1TH\nvalk\t1\tNecroPet\t1\t1\t400\t480\t141\t9\t24\t250\n"))
	if err != nil {
		t.Fatal(err)
	}

	valk, _ := tp.ByID("valk")

	for _, tc := range []struct {
		name         string
		str, dmgPct  int
		wantMin, max int
	}{
		{"no strength", 0, 0, 9, 24},
		{"level 1", 25, 0, 11, 30},   // 9 * 125 / 100, 24 * 125 / 100
		{"level 10", 250, 0, 31, 84}, // 9 * 350 / 100, 24 * 350 / 100
		{"level 20", 500, 0, 54, 144},
		{"strength and damagepercent share one accumulator", 100, 50, 22, 60}, // +150 percent, not 2.0 * 1.5
		{"floored at -90", 0, -200, 0, 2},                                     // 9 * 10 / 100, 24 * 10 / 100
	} {
		var aura []d2skill.StatMod
		if tc.str != 0 {
			aura = append(aura, d2skill.StatMod{Stat: "strength", Value: tc.str})
		}

		if tc.dmgPct != 0 {
			aura = append(aura, d2skill.StatMod{Stat: "damagepercent", Value: tc.dmgPct})
		}

		s := Compute(valk, Normal, Mods{Aura: aura}, nil)
		if s.DmgMin != tc.wantMin || s.DmgMax != tc.max {
			t.Errorf("%s: damage %d-%d, want %d-%d", tc.name, s.DmgMin, s.DmgMax, tc.wantMin, tc.max)
		}
	}
}

// The flat item_normaldamage part is added first, then the shared percent applies to both ends.
func TestW31StrengthAfterFlatDamage(t *testing.T) {
	tp, _ := LoadTemplates([]byte("Id\thcIdx\tAI\tVelocity\tRun\tminHP\tmaxHP\tAC\tA1MinD\tA1MaxD\tA1TH\nm\t1\tNecroPet\t1\t1\t10\t10\t0\t2\t4\t0\n"))
	m, _ := tp.ByID("m")

	s := Compute(m, Normal, Mods{Passive: []d2skill.StatMod{{Stat: "item_normaldamage", Value: 2}, {Stat: "strength", Value: 100}}}, nil)
	if s.DmgMin != 8 || s.DmgMax != 12 { // (2+2) * 2, (4+2) * 2
		t.Errorf("damage %d-%d, want 8-12", s.DmgMin, s.DmgMax)
	}
}

// Stats.Live is what petworld hands the minion for a SkillDamage template; Compute itself never sets it.
func TestW31LiveUnsetByCompute(t *testing.T) {
	tp := loadFixture(t)

	if s := Compute(must(t, tp, "druidhawk"), Normal, Mods{}, nil); s.Live != nil {
		t.Error("Compute set Live")
	}
}
