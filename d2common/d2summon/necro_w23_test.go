package d2summon

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// SKILL_SpawnSummonedMinion 0x5c22c0 (wave 23): the maximum life, with the passive maxhp already on the unit, is
// multiplied by calc1 percent.
func TestMasteryLifeIsMultipliedByPercent(t *testing.T) {
	sk := must(t, loadFixture(t), "necroskeleton")

	s := Compute(sk, Normal, Mods{HPPct: 100, Passive: []d2skill.StatMod{{Stat: "maxhp", Value: 10 * 256}}}, nil)
	if s.MaxHP != (21+10)*2 {
		t.Errorf("max life %d, want %d", s.MaxHP, (21+10)*2)
	}

	// no percent: the flat life is simply added
	if s := Compute(sk, Normal, Mods{Passive: []d2skill.StatMod{{Stat: "maxhp", Value: 10 * 256}}}, nil); s.MaxHP != 31 {
		t.Errorf("flat only %d", s.MaxHP)
	}
}

// item_normaldamage (Skeleton Mastery passivestat2) is flat damage on both ends of the hit.
func TestMasteryDamageIsFlat(t *testing.T) {
	sk := must(t, loadFixture(t), "necroskeleton")

	s := Compute(sk, Normal, Mods{Passive: []d2skill.StatMod{{Stat: "item_normaldamage", Value: 6}}}, nil)
	if s.DmgMin != 7 || s.DmgMax != 8 {
		t.Errorf("damage %d-%d, want 7-8", s.DmgMin, s.DmgMax)
	}
}
