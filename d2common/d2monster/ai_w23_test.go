package d2monster

import (
	"fmt"
	"strings"
	"testing"
)

// w23World records speed requests, unit states and fails casts on demand.
type w23World struct {
	*fakeWorld
	speeds   []int
	states   []string
	failCast bool
}

func (w *w23World) SetSpeed(_ *Brain, v int) { w.speeds = append(w.speeds, v) }

func (w *w23World) SetUnitState(_ *Brain, s int, on bool) {
	w.states = append(w.states, fmt.Sprintf("%x=%v", s, on))
}

func (w *w23World) Cast(b *Brain, slot int, t Target) bool {
	if w.failCast {
		return false
	}

	return w.fakeWorld.Cast(b, slot, t)
}

func w23Ctx(w World, b *Brain, dist int, inRange bool) *Ctx {
	t := Target{ID: 1, X: 100 + dist, Y: 100, Size: 1, IsPlayer: true}

	return &Ctx{B: b, W: w, Params: Params{Target: &t, Dist: dist, InRange: inRange}}
}

// TestW23QueuedSpeedSurvivesWalkAway: Baboon in its rage (boost = run*100/walk
// - 100) and Mosquito in its retreat queue a speed and then walk away by more
// than 5 subtiles; the exe's speed setter ignores the zero percent the
// walk-away passes (0x5ddedb, 0x5a3c60), so exactly one speed request is made.
func TestW23QueuedSpeedSurvivesWalkAway(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func() (*Brain, *Ctx, *w23World)
		want  int
	}{
		{"baboon rage flee", func() (*Brain, *Ctx, *w23World) {
			p := profile("Baboon", 0, 0, 0, 0, 0)
			p.Walk, p.Run = 100, 150
			b := brainAt(p)
			b.Scratch[0], b.HPPercent = 3, 50
			w := &w23World{fakeWorld: newFake(10, false)}

			return b, w23Ctx(w, b, 10, false), w
		}, 50},
		{"mosquito retreat", func() (*Brain, *Ctx, *w23World) {
			b := brainAt(profile("Mosquito", 0, 0, 0, 0, 5))
			b.Scratch[0] = 2
			w := &w23World{fakeWorld: newFake(4, false)}

			return b, w23Ctx(w, b, 4, false), w
		}, 100},
	} {
		_, c, w := tc.build()
		if strings.HasPrefix(tc.name, "baboon") {
			thinkBaboon(c)
		} else {
			thinkMosquito(c)
		}

		if len(w.speeds) != 1 || w.speeds[0] != tc.want {
			t.Errorf("%s: speed requests %v, want [%d]", tc.name, w.speeds, tc.want)
		}

		if !strings.HasPrefix(w.last(), "walk-to(") {
			t.Errorf("%s: last action %q, want a walk-away", tc.name, w.last())
		}
	}
}

// TestW23ClawViperBuffIgnoresCastResult: both ClawViper AIs switch the buff
// state on and set Scratch[0] even when the cast request is refused.
func TestW23ClawViperBuffIgnoresCastResult(t *testing.T) {
	for _, tc := range []struct {
		name  string
		think Think
		aip   []int
		state string
	}{
		{"ClawViper state 5a", thinkClawViper, []int{100, 10, 0, 0, 6, 1}, "5a=true"},
		{"ClawViper state 5b", thinkClawViper, []int{100, 10, 0, 0, 6, 2}, "5b=true"},
		{"ClawViperEx state 5a", thinkClawViperEx, []int{100, 10, 0, 0, 6, 1, 0, 0}, "5a=true"},
		{"ClawViperEx state 5b", thinkClawViperEx, []int{100, 10, 0, 0, 6, 2, 0, 0}, "5b=true"},
	} {
		for _, fail := range []bool{false, true} {
			b := brainAt(withSkills(profile(tc.name, tc.aip...), 0))
			w := &w23World{fakeWorld: newFake(6, false), failCast: fail}
			tc.think(w23Ctx(w, b, 6, false))

			if b.Scratch[0] != 1 || len(w.states) != 1 || w.states[0] != tc.state {
				t.Errorf("%s fail=%v: scratch=%d states=%v", tc.name, fail, b.Scratch[0], w.states)
			}
		}
	}
}

// TestW23CorruptLancerReach: MONAI_RunToTargetWithin 0x5ddd90 turns a zero
// distance into 1; a monstats2 MeleeRng above that is passed through.
func TestW23CorruptLancerReach(t *testing.T) {
	for _, reach := range []int{0, 1, 3, 9} {
		want := reach
		if want < 1 {
			want = 1
		}

		b := brainAt(profile("CorruptLancer", 0, 0, 0, 0, 10))
		b.Profile.ChaseReach = reach
		w := &w23World{fakeWorld: newFake(12, false)}
		thinkCorruptLancer(w23Ctx(w, b, 12, false))

		if got, wantLog := w.last(), fmt.Sprintf("run-target/%d", want); got != wantLog {
			t.Errorf("ChaseReach %d: %q, want %q", reach, got, wantLog)
		}
	}
}

// TestW23PantherJavelinEndsAfterRefusedMove: the exe does not test the result
// of the walk-away and walk-near requests, so a refused move ends the tick
// without a throw or any further roll.
func TestW23PantherJavelinEndsAfterRefusedMove(t *testing.T) {
	for _, tc := range []struct {
		name      string
		aip       []int
		dist      int
		checkSeed bool
	}{
		{"walk away refused", []int{100, 100, 12, 100, 15, 20}, 5, true},
		{"walk near refused", []int{100, 100, 12, 0, 15, 20}, 30, false},
	} {
		w := &w23World{fakeWorld: newFake(tc.dist, false)}
		w.failMove = true
		b := brainAt(profile("PantherJavelin", tc.aip...))
		before := *shadow(b)
		c := w23Ctx(w, b, tc.dist, false)
		thinkPantherJavelin(c)

		if len(w.log) != 0 || c.Acted() {
			t.Errorf("%s: log %v acted %v, want nothing after the refused move", tc.name, w.log, c.Acted())
		}

		if tc.checkSeed {
			// exactly one roll was drawn (the test chance), not the throw roll
			r := before
			r.Roll(100)

			if *b.Seed != r {
				t.Errorf("%s: seed advanced more than one roll", tc.name)
			}
		}
	}
}
