package d2monster

import (
	"strings"
	"testing"
)

type mummyWorld struct {
	*fakeWorld
	scan        MummyScan
	gotRadius   int
	gotRadament bool
	states      []int
	speeds      []int
}

func (w *mummyWorld) MummyScan(_ *Brain, radius int, radament bool) MummyScan {
	w.gotRadius, w.gotRadament = radius, radament

	return w.scan
}

func (w *mummyWorld) SetUnitState(_ *Brain, s int, on bool) {
	if on {
		w.states = append(w.states, s)
	}
}

func (w *mummyWorld) SetSpeed(_ *Brain, v int) { w.speeds = append(w.speeds, v) }

func TestGreaterMummyScanFlow(t *testing.T) {
	corpse := &Target{ID: 9, X: 105, Y: 100, Size: 1}

	for _, tc := range []struct {
		name   string
		prof   *Profile
		scan   MummyScan
		class  int
		want   string
		radius int
	}{
		{"wounded neighbour: Skill2 cast with no target", withSkills(profile("GreaterMummy", 0, 0, 100, 0, 10), 1),
			MummyScan{Wounded: true, InReach: 1}, 1, "cast1", 10},
		{"corpse: Skill1 raises it", withSkills(profile("GreaterMummy", 0, 100, 0, 0, 10), 0),
			MummyScan{Corpse: corpse, InReach: 1}, 1, "cast0", 10},
		{"Radament widens the scan by 10", withSkills(profile("GreaterMummy", 0, 100, 0, 0, 10), 0),
			MummyScan{Corpse: corpse, InReach: 1}, radamentClass, "cast0", 20},
		{"nobody in reach: walk", profile("GreaterMummy", 0, 0, 0, 0, 10), MummyScan{}, 1, "walk-target/3", 10},
		{"neighbours in reach, no skill: wait or circle", profile("GreaterMummy", 0, 0, 0, 0, 10),
			MummyScan{InReach: 2}, 1, "", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &mummyWorld{fakeWorld: newFake(9, false), scan: tc.scan}
			b := brainAt(tc.prof)
			b.Class = tc.class
			Tick(w, b)

			if got := strings.Join(w.log, ","); tc.want != "" && got != tc.want {
				t.Fatalf("log %q, want %q", got, tc.want)
			}

			if tc.want == "" && len(w.log) > 0 && !strings.HasPrefix(w.log[0], "walk-to(") {
				t.Fatalf("with neighbours the mummy must not walk at the target: %v", w.log)
			}

			if w.gotRadius != tc.radius || w.gotRadament != (tc.class == radamentClass) {
				t.Fatalf("scan radius %d radament %v", w.gotRadius, w.gotRadament)
			}
		})
	}
}

func TestCorruptRogueRunDistanceByDifficulty(t *testing.T) {
	for _, tc := range []struct {
		diff Difficulty
		dist int
		run  bool
	}{{Normal, 20, false}, {Normal, 21, true}, {Nightmare, 17, false}, {Nightmare, 18, true},
		{Hell, 14, false}, {Hell, 15, true}} {
		w := newFake(tc.dist, false)
		b := NewBrain(7, 1, tc.diff, profile("CorruptRogue", 0, 5, 0, 50, 100), testSeed)
		b.X, b.Y = 100, 100
		Tick(w, b)

		if got := strings.HasPrefix(w.last(), "run-target/3"); got != tc.run {
			t.Errorf("diff %d dist %d: run=%v log %v", tc.diff, tc.dist, got, w.log)
		}
	}
}

func TestCorruptRogueFailedRunUsesIdleWait(t *testing.T) {
	w := newFake(30, false)
	w.failMove = true
	b := brainAt(profile("CorruptRogue", 100, 40, 100, 50, 100))
	Tick(w, b)

	if b.Wake == 40 {
		t.Fatalf("a failed run request must not wait aip2 frames (the exe just ends the tick), wake=%d", b.Wake)
	}
}

// TestWalkAwayKeepsQueuedSpeed: 0x5ddedb calls the speed setter with EDX 0 and
// a zero percent, and 0x5a3c60 ignores zero arguments, so a walk-away of any
// length leaves the speed percent the caller queued (w23; batch 8 had this
// the other way round).
func TestWalkAwayKeepsQueuedSpeed(t *testing.T) {
	for _, n := range []int{1, 4, 5, 6, 8, 15, 16} {
		w := &mummyWorld{fakeWorld: newFake(9, false)}
		b := brainAt(profile("Vampire", 0))
		c := &Ctx{B: b, W: w}
		c.SetSpeed(80)
		c.WalkAway(Target{X: 110, Y: 100}, n)

		if len(w.speeds) != 1 || w.speeds[0] != 80 {
			t.Errorf("n=%d: speed requests %v, want only the caller's 80", n, w.speeds)
		}
	}
}

func TestMegademonLeavesInfernoState(t *testing.T) {
	w := &mummyWorld{fakeWorld: newFake(3, false)}
	w.frame = 1
	b := brainAt(withSkills(profile("Megademon", 100, 0, 0, 0, 0, 40), 0))
	b.Profile.Skills[0].Level = 8
	Tick(w, b)

	if len(w.states) != 1 || w.states[0] != inferno || w.last() != "cast0" {
		t.Fatalf("states %v log %v", w.states, w.log)
	}
}
