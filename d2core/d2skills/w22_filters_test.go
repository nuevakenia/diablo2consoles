package d2skills

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// MONSTATS_IsRevivableType (0x5dc1e0): walk mode, not boss, switchai.
func TestConvertibleType(t *testing.T) {
	walk := &d2records.MonStat2Record{}
	walk.HasAnimationMode[2] = true

	boss := &d2records.MonStatRecord{}
	boss.IsSpecialBoss, boss.IsAiSwitchable = true, true

	fixed := &d2records.MonStatRecord{} // switchai 0: totems, traps, eggs, the prime evils
	plain := &d2records.MonStatRecord{}
	plain.IsAiSwitchable = true

	for _, c := range []struct {
		name string
		stat *d2records.MonStatRecord
		ex   *d2records.MonStat2Record
		want bool
	}{
		{"ordinary monster", plain, walk, true},
		{"boss", boss, walk, false},
		{"switchai off", fixed, walk, false},
		{"no walk mode", plain, &d2records.MonStat2Record{}, false},
		{"no stats2 row", plain, nil, true},
		{"no stats row", nil, walk, true},
	} {
		if got := convertibleType(c.stat, c.ex); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Holy Freeze on a monster: the slow is capped by its cold effect; a monster without one is not slowed.
func TestAuraTargetModsClampsSlow(t *testing.T) {
	stats := []d2skill.StatMod{{Stat: "velocitypercent", Value: -60}, {Stat: "attackrate", Value: -60},
		{Stat: "other_animrate", Value: -60}}

	val := func(mods []d2state.StatMod, name string) int {
		for _, m := range mods {
			if m.Stat == name {
				return m.Value
			}
		}

		return 1 << 20
	}

	boss := &d2mapentity.Monster{Stat: &d2records.MonStatRecord{}}
	boss.Stat.ColdSensitivityNormal = -25

	immune := &d2mapentity.Monster{Stat: &d2records.MonStatRecord{}}

	got := auraTargetMods(boss, stats)
	if val(got, "velocitypercent") != -25 || val(got, "attackrate") != -25 || val(got, "other_animrate") != -60 {
		t.Errorf("cold effect -25: %v", got)
	}

	got = auraTargetMods(immune, stats)
	if val(got, "velocitypercent") != 0 || val(got, "attackrate") != 0 {
		t.Errorf("cold effect 0: %v", got)
	}

	if got := auraTargetMods(&d2mapentity.Monster{}, stats); val(got, "velocitypercent") != -60 {
		t.Errorf("no record: %v", got)
	}
}
