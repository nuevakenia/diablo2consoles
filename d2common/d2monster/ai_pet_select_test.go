package d2monster

import "testing"

// TestFBNSelectPetModes pins MONAI_SelectPetActionByRange 0x5e3510 (VERIFIED w39).
func TestFBNSelectPetModes(t *testing.T) {
	step := func(ownerDist int, mode Mode, tgt *Target, inRange, stay bool) (*fakePet, *Brain, bool) {
		f, b := fbnPetNew("NecroPet", ownerDist)
		f.owner.Mode = mode
		c := &Ctx{B: b, W: f}
		l, _ := fbnLeader(c)

		return f, b, fbnSelectPet(c, tgt, l, inRange, stay, 7)
	}

	foe := &Target{ID: 50, X: 104, Y: 100, Size: 1}

	// a foe within 80 of the leader: the pet fights (returns 0, no action)
	if f, b, acted := step(30, ModeNeutral, foe, false, false); acted || len(f.log) != 0 || b.Wake != 0 {
		t.Errorf("foe near: acted=%v log=%v", acted, f.log)
	}

	// a foe but the leader is over 80 away: teleport (mode 3) and wait 5
	if f, b, acted := step(90, ModeNeutral, foe, false, false); !acted || f.teleports != 1 || b.Wake != 5 {
		t.Errorf("foe far: acted=%v teleports=%d wake=%d", acted, f.teleports, b.Wake)
	}

	// idle, mode 2: a 15 frame wait whatever the 10 percent roll says
	if _, b, acted := step(5, ModeNeutral, nil, false, false); !acted || b.Wake != 15 {
		t.Errorf("idle: acted=%v wake=%d", acted, b.Wake)
	}

	// idle with the stay bit: mode 2 hands control back
	if _, b, acted := step(5, ModeNeutral, nil, false, true); acted || b.Wake != 0 {
		t.Errorf("stay bit: acted=%v wake=%d", acted, b.Wake)
	}

	// beyond 50: teleport even with the stay bit (mode 3)
	if f, _, acted := step(60, ModeNeutral, nil, false, true); !acted || f.teleports != 1 {
		t.Errorf("far: acted=%v teleports=%d", acted, f.teleports)
	}

	// the leader walks: mode 0 (an actual move, no idle wait)
	if f, b, acted := step(12, ModeWalk, nil, false, false); !acted || len(f.log) == 0 || b.Wake == 15 {
		t.Errorf("walking leader: acted=%v log=%v wake=%d", acted, f.log, b.Wake)
	}

	// the leader runs (player mode 3 or the merc style ModeRun) is treated the same
	for _, m := range []Mode{3, ModeRun} {
		if !leaderRuns(m) || leaderWalks(m) {
			t.Errorf("mode %d should be a run", m)
		}
	}

	// next to the leader and not in range: mode 5 nudge (a move); in range the nudge is skipped (mode 2: wait)
	if f, _, acted := step(1, ModeNeutral, nil, false, false); !acted || len(f.log) == 0 {
		t.Errorf("adjacent idle: acted=%v log=%v", acted, f.log)
	}

	if _, b, acted := step(1, ModeNeutral, nil, true, false); !acted || b.Wake != 15 {
		t.Errorf("adjacent in range: acted=%v wake=%d", acted, b.Wake)
	}
}
