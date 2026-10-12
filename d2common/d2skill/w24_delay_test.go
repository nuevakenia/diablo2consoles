package d2skill

import "testing"

// Wave 24: the skill delay is ONE player state (0x79) shared by every delayed skill
// (0x56d6a0 / 0x56cde0 / 0x648cd0, VERIFIED), not a cooldown per skill.
func TestSkillDelayIsSharedByDelayedSkills(t *testing.T) {
	f := newFixture(map[string]int{"Fire Bolt": 1, "Test Delay": 1, "Test Delay B": 1})
	f.u.base = f.u.levels

	if do := f.p.Do(f.u, f.id("Test Delay"), Target{X: 5}); !do.OK || do.Cooldown != 35 {
		t.Fatalf("%+v", do)
	}

	// another delayed skill is refused while the state is on, a skill without a delay is not
	f.w.frame = 10
	if st := f.p.Start(f.u, f.id("Test Delay B"), Target{}); st.OK || st.Reason != ReasonCooldown {
		t.Fatalf("delayed skill must wait for the shared state: %+v", st)
	}

	if st := f.p.Start(f.u, f.id("Fire Bolt"), Target{X: 5}); !st.OK {
		t.Fatalf("a skill without delay is not blocked: %+v", st)
	}

	f.w.frame = 35
	if st := f.p.Start(f.u, f.id("Test Delay B"), Target{}); !st.OK {
		t.Fatalf("state over: %+v", st)
	}
}
