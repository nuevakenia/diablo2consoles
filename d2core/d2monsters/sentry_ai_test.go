package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
)

// An armed assassin sentry with a known shot budget is driven by the ported
// monster AI; without a firer, a budget or the right AI it keeps the old timer.
func TestTrapAIDriven(t *testing.T) {
	s := petUnit(1, 0, 0, "AssassinSentry", "trap")
	s.b.Profile = &d2monster.Profile{AI: "AssassinSentry"}
	s.b.Profile.Skills[0].Name = "x"
	d := testDirector(s)
	d.byEntity[s.m.ID()] = s

	if d.TrapAIDriven(s.m) {
		t.Error("no firer, not armed")
	}

	d.SetSkillFirer(&fakeFirer{})
	d.ArmTrap(s.m, TrapSpec{Level: 7, Life: 0})

	if d.TrapAIDriven(s.m) {
		t.Error("unknown shot budget must keep the old driver")
	}

	d.ArmTrap(s.m, TrapSpec{Level: 7, Life: 12})

	if !d.TrapAIDriven(s.m) {
		t.Error("armed with a budget")
	}

	if d.SkillLevel(s.b, 0) != 7 || d.SkillCalc4(s.b, 0, 7) != 12 {
		t.Errorf("level %d life %d", d.SkillLevel(s.b, 0), d.SkillCalc4(s.b, 0, 7))
	}

	s.b.Profile.AI = "DeathSentry"

	if d.TrapAIDriven(s.m) {
		t.Error("only the assassin sentry AI is wired")
	}
}

// Units that are not armed sentries keep the AI's old fallbacks.
func TestSkillCalcFallbacks(t *testing.T) {
	u := petUnit(1, 0, 0, "Melee", "")
	u.b.Profile = &d2monster.Profile{}
	d := testDirector(u)

	if d.SkillLevel(u.b, 0) != 0 || d.SkillCalc4(u.b, 0, 1) != 1000 {
		t.Error("fallbacks")
	}

	u.b.Profile.Skills[0].Name = "Attack"

	if d.SkillLevel(u.b, 0) != 1 {
		t.Error("used slot is level 1")
	}
}
