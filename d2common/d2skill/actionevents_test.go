package d2skill

import (
	"reflect"
	"testing"
)

// flagsAt builds a 12 frame flag table with one flag at a frame.
func flagsAt(n int, frame int, flag byte) []byte {
	f := make([]byte, n)
	f[frame] = flag

	return f
}

func TestScheduleActionEventsTraces(t *testing.T) {
	cases := []struct {
		name         string
		flags        []byte
		start, rate  int
		wantEv       []ActionEvent
		wantEnd      int
		framesPerDir int
	}{
		// rate 256: frame f is reached at game frame f - start; the mode ends when the position passes the last frame
		{"1fps start0", flagsAt(12, 6, 1), 0, 256, []ActionEvent{{6, 1}}, 12, 12},
		{"start2 skips two frames", flagsAt(12, 6, 1), 2, 256, []ActionEvent{{4, 1}}, 10, 12},
		{"half speed", flagsAt(12, 6, 1), 0, 128, []ActionEvent{{12, 1}}, 24, 12},
		{"double speed", flagsAt(12, 6, 1), 0, 512, []ActionEvent{{3, 1}}, 6, 12},
		// a flag on a frame before the start frame is never read
		{"flag before start", flagsAt(12, 1, 1), 2, 256, nil, 10, 12},
		// flag 3 and 4 are scheduled too
		{"sound and skill", []byte{0, 0, 3, 0, 4, 0, 0, 0}, 0, 256, []ActionEvent{{2, 3}, {4, 4}}, 8, 8},
	}

	for _, c := range cases {
		ev, end := ScheduleActionEvents(c.flags, c.start, c.rate, c.framesPerDir<<8)
		if !reflect.DeepEqual(ev, c.wantEv) || end != c.wantEnd {
			t.Errorf("%s: events %v end %d, want %v end %d", c.name, ev, end, c.wantEv, c.wantEnd)
		}
	}

	if ev, end := ScheduleActionEvents(flagsAt(12, 6, 1), 0, 0, 12<<8); ev != nil || end != 1 {
		t.Errorf("rate 0: %v %d, want no events, end 1", ev, end)
	}
}

func TestRescheduleRestartsFromForgivenElapsed(t *testing.T) {
	fl := flagsAt(12, 6, 1)

	// pct 100 forgives everything: the animation restarts at frame 0
	ne, ev, end := RescheduleActionEvents(fl, 6, 100, 256, 12<<8)
	if ne != 0 || !reflect.DeepEqual(ev, []ActionEvent{{6, 1}}) || end != 12 {
		t.Errorf("pct 100: ne=%d ev=%v end=%d", ne, ev, end)
	}

	// Fend: Param2 60 forgives 60 percent of the 6 elapsed frames (C division: 6*40/100 = 2)
	ne, ev, end = RescheduleActionEvents(fl, 6, 60, 256, 12<<8)
	if ne != 2 || !reflect.DeepEqual(ev, []ActionEvent{{4, 1}}) || end != 10 {
		t.Errorf("pct 60: ne=%d ev=%v end=%d", ne, ev, end)
	}

	// nothing forgiven at the last frame: no further event, the mode ends next frame
	ne, ev, end = RescheduleActionEvents(fl, 11, 0, 256, 12<<8)
	if ne != 11 || len(ev) != 0 || end != 2 {
		t.Errorf("pct 0 at the end: ne=%d ev=%v end=%d", ne, ev, end)
	}
}

func TestBurstGapsClosedForm(t *testing.T) {
	// at rate 256 the elapsed time at every event is the flagged frame F, so each gap is F - F*(100-pct)/100
	for _, pct := range []int{100, 60, 50, 25, 0} {
		c := NewActionClock(flagsAt(12, 6, 1), 0, 256, 12)
		got := BurstGaps(c, 5, pct)

		if pct == 0 {
			// nothing forgiven: the scan restarts one frame before the restart frame, so the flag fires at the next
			// game frame, twice, then the restart frame is past the flag and the mode ends (hand trace of 0x551b90)
			if want := []int{6, 7, 8}; !reflect.DeepEqual(got, want) {
				t.Errorf("pct 0: hits %v, want %v", got, want)
			}

			continue
		}

		gap := 6 - 6*(100-pct)/100
		want := []int{6}
		for i := 1; i < 5; i++ {
			want = append(want, want[i-1]+gap)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("pct %d: hits %v, want %v", pct, got, want)
		}
	}
}

func TestBurstGapsHalfSpeedTrace(t *testing.T) {
	// hand trace of 0x551b90 at rate 128 (see w25-melee.md): 12, 16, 22, 28, 34
	c := NewActionClock(flagsAt(12, 6, 1), 0, 128, 12)
	got := BurstGaps(c, 5, 60)

	if want := []int{12, 16, 22, 28, 34}; !reflect.DeepEqual(got, want) {
		t.Errorf("hits %v, want %v", got, want)
	}
}

func TestOnlyFlagsOneAndTwoRunTheDoFunction(t *testing.T) {
	c := NewActionClock([]byte{0, 0, 3, 0, 4, 0, 2, 0, 0, 1, 0, 0}, 0, 256, 12)

	if got := c.DoEvents(); !reflect.DeepEqual(got, []int{6, 9}) {
		t.Errorf("do events %v, want [6 9]", got)
	}

	if !RunsDo(1) || !RunsDo(2) || RunsDo(3) || RunsDo(4) || RunsDo(0) {
		t.Error("RunsDo table")
	}
}

func TestAttackStartFrameTable(t *testing.T) {
	cases := []struct {
		class int
		wc    string
		want  int
	}{
		{0, "hth", 1}, {1, "hth", 1}, {0, "1hs", 2}, {1, "stf", 2}, {0, "2ht", 2}, {0, "bow", 0}, {0, "xbw", 0},
		{4, "1hs", 0}, {2, "hth", 0}, {6, "hth", 0}, {3, "2hs", 0},
	}

	for _, c := range cases {
		if got := AttackStartFrame(c.class, c.wc); got != c.want {
			t.Errorf("class %d %s: %d, want %d", c.class, c.wc, got, c.want)
		}
	}
}

func TestPlayerSequencesJabAndImpale(t *testing.T) {
	cases := []struct {
		seq  int
		wc   string
		rows int
		do   []int
	}{
		{SeqJab, "hth", 13, []int{8}},
		{SeqJab, "1ht", 18, []int{3, 9, 15}},
		{SeqJab, "2ht", 21, []int{3, 10, 17}},
		{SeqImpale, "hth", 13, []int{8}},
		{SeqImpale, "1ht", 21, []int{13}},
		{SeqImpale, "2ht", 24, []int{15}},
	}

	for _, c := range cases {
		s, ok := Sequence(c.seq, c.wc)
		if !ok || s.Rows != c.rows || !reflect.DeepEqual(s.DoRows, c.do) {
			t.Errorf("seq %d %s: %+v ok=%v", c.seq, c.wc, s, ok)
		}

		// at one row per game frame the do events fall on their rows
		clk := NewActionClock(s.Flags(), 0, 256, s.Rows)
		if got := clk.DoEvents(); !reflect.DeepEqual(got, c.do) {
			t.Errorf("seq %d %s: do ticks %v, want %v", c.seq, c.wc, got, c.do)
		}
	}

	if _, ok := Sequence(SeqJab, "bow"); ok {
		t.Error("a bow has no Jab sequence")
	}
}

// ---- bursts on the clock ----

func clockFixture(rate int) (*classFixture, *testTarget) {
	cf, z := burstFixture()
	cf.p.Opt.EventBursts = true
	cf.p.Clock = func(Unit, *Skill) *ActionClock { return NewActionClock(flagsAt(12, 6, 1), 0, rate, 12) }

	return cf, z
}

func TestZealCadenceFromTheClock(t *testing.T) {
	// Zeal Param2 100: every hit restarts the animation from frame 0, so a hit every 6 game frames at rate 256
	cf, z := clockFixture(256)
	_, r := cf.castOn("Zeal", z)

	if r.Burst == nil {
		t.Fatal("no burst")
	}

	for i := 0; i < 4; i++ {
		d, fromClock := r.Burst.NextDelay()
		if !fromClock || d != 6 {
			t.Fatalf("event %d: delay %d clock=%v, want 6 from the clock", i+1, d, fromClock)
		}

		var rr DoResult
		if !cf.p.BurstEvent(r.Burst, &rr) {
			t.Fatalf("event %d did not hit", i+1)
		}
	}

	if !r.Burst.Done() || r.Burst.Shots() != 5 {
		t.Errorf("shots %d done %v", r.Burst.Shots(), r.Burst.Done())
	}
}

func TestFendCadenceUsesParam2(t *testing.T) {
	// Fend Param2 60: gap 6 - 6*40/100 = 4
	cf, z := clockFixture(256)
	cf.addFoe("f2", 2, 1)
	cf.addFoe("f3", 1, 2)

	_, r := cf.castOn("Fend", z)
	if r.Burst == nil {
		t.Fatal("no burst")
	}

	if d, ok := r.Burst.NextDelay(); !ok || d != 4 {
		t.Errorf("delay %d clock=%v, want 4", d, ok)
	}
}

func TestBurstWithoutClockUsesFallback(t *testing.T) {
	cf, z := burstFixture()
	cf.p.Opt.EventBursts = true

	_, r := cf.castOn("Zeal", z)
	if d, ok := r.Burst.NextDelay(); ok || d != FallbackBurstGap {
		t.Errorf("fallback delay %d clock=%v", d, ok)
	}
}

func TestBurstEndsWhenTheModeEndsFirst(t *testing.T) {
	// an animation whose only flagged frame is its last: a reschedule that forgives nothing leaves no do event
	cf, z := burstFixture()
	cf.p.Opt.EventBursts = true
	cf.p.Clock = func(Unit, *Skill) *ActionClock { return NewActionClock(flagsAt(12, 11, 1), 0, 256, 12) }
	cf.p.Skills.ByName("Zeal").Params[2] = 0

	_, r := cf.castOn("Zeal", z)
	if r.Burst != nil && !r.Burst.Done() {
		t.Errorf("burst continues past the mode end: left %d", r.Burst.Left())
	}
}
