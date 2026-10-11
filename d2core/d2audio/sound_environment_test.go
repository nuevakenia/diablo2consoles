package d2audio

import (
	"fmt"
	"math"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

type fakeSink struct {
	loops    []int
	stopped  []int
	events   []int
	musicOff bool
}

func (f *fakeSink) musicEnabled() bool { return !f.musicOff }

func (f *fakeSink) loop(index int) func() {
	f.loops = append(f.loops, index)
	return func() { f.stopped = append(f.stopped, index) }
}

func (f *fakeSink) event(index int, _ func(int) int) string {
	f.events = append(f.events, index)
	return fmt.Sprint(index)
}

func (f *fakeSink) handle(index int) string { return fmt.Sprint("s", index) }

func testEnvs() map[int]*d2records.SoundEnvironRecord {
	return map[int]*d2records.SoundEnvironRecord{
		0: {Handle: "NONE", Index: 0, EventDelay: 250},
		2: {Handle: "WILD", Index: 2, Song: 4679, DayAmbience: 70, NightAmbience: 71, DayEvent: 192, NightEvent: 197, EventDelay: 250},
		4: {Handle: "CAVE", Index: 4, Song: 4657, DayAmbience: 54, NightAmbience: 54, DayEvent: 87, NightEvent: 87, EventDelay: 200, Indoors: 1},
		5: {Handle: "TOWN", Index: 5, Song: 4673, DayAmbience: 70, NightAmbience: 71},
	}
}

func newTestEnv(sink *fakeSink) *SoundEnvironment {
	var logs int

	e := newSoundEnvironment(sink, testEnvs(), func(n int) int { return n / 2 },
		func(string, ...interface{}) { logs++ })

	return &e
}

func TestSelectDayNight(t *testing.T) {
	envs := testEnvs()

	tests := []struct {
		env   int
		night bool
		want  Selection
	}{
		{2, false, Selection{Song: 4679, Ambience: 70, Event: 192}},
		{2, true, Selection{Song: 4679, Ambience: 71, Event: 197, Night: true}},
		{4, true, Selection{Song: 4657, Ambience: 54, Event: 87, Night: true}},
		{0, false, Selection{}},
	}

	for _, tc := range tests {
		if got := Select(envs[tc.env], tc.night); got != tc.want {
			t.Errorf("Select(env %d, night=%v) = %+v, want %+v", tc.env, tc.night, got, tc.want)
		}
	}

	if got := Select(nil, true); got.Song != 0 || !got.Night {
		t.Errorf("Select(nil) = %+v", got)
	}
}

func TestIsNight(t *testing.T) {
	want := map[int]bool{0: true, 1: false, 2: false, 3: false, 4: true, 5: true}
	for phase, night := range want {
		if IsNight(phase) != night {
			t.Errorf("IsNight(%d) = %v, want %v", phase, !night, night)
		}
	}
}

func TestNextEventTicksRange(t *testing.T) {
	// d + rand[-d/3, d/3]: the first draw is rand(2*(d/3)+1) - d/3
	tests := []struct{ delay, r, want int }{
		{300, 0, 200}, {300, 100, 300}, {300, 200, 400}, {250, 0, 250 - 83}, {250, 166, 250 + 83}, {2, 0, 2}, {0, 0, 1},
	}
	for _, tc := range tests {
		var asked int

		got := NextEventTicks(tc.delay, func(n int) int { asked = n; return tc.r })
		if got != tc.want {
			t.Errorf("NextEventTicks(%d) with draw %d = %d, want %d", tc.delay, tc.r, got, tc.want)
		}

		if tc.delay >= 3 && asked != 2*(tc.delay/3)+1 {
			t.Errorf("NextEventTicks(%d) drew below %d, want %d", tc.delay, asked, 2*(tc.delay/3)+1)
		}
	}
}

func TestFirstEventElapsed(t *testing.T) {
	if got := FirstEventElapsed(250, func(n int) int { return n - 1 }); got != 249 {
		t.Errorf("elapsed %d, want 249", got)
	}

	if FirstEventElapsed(0, func(int) int { t.Fatal("drew with no interval"); return 0 }) != 0 {
		t.Error("zero interval must not wait")
	}
}

func TestEventOffsetRange(t *testing.T) {
	for sign := 0; sign < 2; sign++ {
		for d := 0; d < 0x2ee-0x1c2+1; d += 13 {
			calls := 0
			dx, dy := EventOffset(func(n int) int {
				calls++
				switch calls {
				case 1:
					return sign
				case 2:
					return d
				}

				return n - 1 // y draw: rand(201) = 200 -> +100
			})

			ax := math.Abs(dx)
			if ax < 450 || ax > 750 || (sign == 0) != (dx < 0) || dy != 100 {
				t.Fatalf("sign %d draw %d: offset (%v,%v)", sign, d, dx, dy)
			}
		}
	}

	// the extremes of the x range
	if dx, _ := EventOffset(func(n int) int { return 0 }); dx != -450 {
		t.Errorf("lowest draw gives dx %v, want -450", dx)
	}
}

func TestEventPositionIsAudibleForFalloffTwo(t *testing.T) {
	// the wilderness/cave events use Falloff 2 (radius 1000): every offset is inside it
	for _, draw := range []int{0, 1, 100, 200} {
		x, y := EventPosition(10, -20, func(n int) int {
			if draw >= n {
				return n - 1
			}

			return draw
		})
		// the bank doubles y when measuring distance
		if dist := math.Hypot(x-10, 2*(y+20)); dist < 450 || dist > 1000 {
			t.Fatalf("draw %d: distance %.1f outside 450..1000", draw, dist)
		}
	}
}

func TestEnvironmentSwitchesMusicAndAmbience(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink)

	e.SetEnv(2) // wilderness, day
	if want := []int{4679, 70}; fmt.Sprint(sink.loops) != fmt.Sprint(want) {
		t.Fatalf("loops %v, want %v", sink.loops, want)
	}

	e.SetEnv(2) // same env: nothing restarts
	if len(sink.loops) != 2 {
		t.Fatalf("re-entering the same env restarted sounds: %v", sink.loops)
	}

	e.SetDayPhase(4) // night: ambience swaps, music stays
	if want := []int{4679, 70, 71}; fmt.Sprint(sink.loops) != fmt.Sprint(want) {
		t.Fatalf("loops after dusk %v, want %v", sink.loops, want)
	}

	if fmt.Sprint(sink.stopped) != "[70]" {
		t.Fatalf("stopped %v, want [70]", sink.stopped)
	}

	e.SetDayPhase(5) // still night: no change
	if len(sink.loops) != 3 {
		t.Fatalf("night to night restarted sounds: %v", sink.loops)
	}

	e.SetEnv(4) // cave: music and ambience change (ambience 54 same day/night)
	e.Advance(float64(MusicSwapTicks)/ticksPerSecond + 0.1)

	if fmt.Sprint(sink.loops) != "[4679 70 71 54 4657]" {
		t.Fatalf("loops after entering the cave %v", sink.loops)
	}

	if got := e.Selection(); got.Event != 87 || !got.Night {
		t.Fatalf("cave selection %+v", got)
	}

	e.SetDayPhase(2) // day inside the cave: nothing changes (both columns equal) but events keep
	if len(sink.loops) != 5 {
		t.Fatalf("cave day/night changed loops: %v", sink.loops)
	}
}

func TestEnvironmentKeepsSharedLoops(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink)
	e.SetEnv(2)
	e.Advance(10)
	e.SetEnv(5) // town 'TOWN' has another song
	e.Advance(10)
	e.SetEnv(2)
	e.Advance(10)

	if fmt.Sprint(sink.loops) != "[4679 70 4673 4679]" {
		t.Fatalf("loops %v", sink.loops)
	}
}

func TestEnvironmentEvents(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink) // rnd(n) = n/2, so delay 250 -> 125 + 125 = 250 ticks = 10 s

	e.SetEnv(2)

	// interval 250 (no jitter), first draw already 125 ticks in
	if got := e.NextEventIn(); got != 125 {
		t.Fatalf("first event in %v ticks, want 125", got)
	}

	e.Advance(4.9)
	if len(sink.events) != 0 {
		t.Fatalf("event fired early: %v", sink.events)
	}

	e.Advance(0.2)
	if fmt.Sprint(sink.events) != "[192]" {
		t.Fatalf("events %v, want the day event [192]", sink.events)
	}

	e.SetDayPhase(4)
	e.Advance(5.2)

	if fmt.Sprint(sink.events) != "[192 197]" {
		t.Fatalf("events %v, want day then night event", sink.events)
	}
}

func TestEventScheduleKeptWhenEventIDRepeats(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink)

	e.SetEnv(2)
	e.Advance(2)
	left := e.NextEventIn()

	e.SetEnv(4) // another event id: new schedule
	if e.NextEventIn() == left {
		t.Fatalf("a new event id kept the old schedule")
	}

	// day cave and night cave share event 87: the schedule is not reset
	e.Advance(1)
	left = e.NextEventIn()
	e.SetDayPhase(2)
	e.SetDayPhase(4)

	if e.NextEventIn() != left {
		t.Fatalf("same event id was rescheduled: %v -> %v", left, e.NextEventIn())
	}
}

func TestEnvironmentWithoutEventsStaysQuiet(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink)

	e.SetEnv(5) // no events
	e.Advance(1000)

	if len(sink.events) != 0 {
		t.Fatalf("events %v from an environment without any", sink.events)
	}
}

func TestEnvironmentUnknownEnvIgnored(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink)
	e.SetEnv(99)

	if len(sink.loops) != 0 {
		t.Fatalf("unknown env started %v", sink.loops)
	}
}

func TestSubtileToSound(t *testing.T) {
	x, y := SubtileToSound(0, 0)
	if x != 0 || y != 0 {
		t.Fatalf("origin maps to %v,%v", x, y)
	}

	// equal subtile steps along either axis are equally far once y is doubled
	ax, ay := SubtileToSound(5, 0)
	bx, by := SubtileToSound(0, 5)

	da, db := math.Hypot(ax, 2*ay), math.Hypot(bx, 2*by)
	if math.Abs(da-db) > 1e-9 {
		t.Fatalf("anisotropic: %v vs %v", da, db)
	}

	// 12 subtiles (the AUTOMONSTER ring) must be audible for Falloff 0 (400)
	mx, my := SubtileToSound(12, 0)
	if d := math.Hypot(mx, 2*my); d >= 400 {
		t.Fatalf("12 subtiles = %.0f units, beyond the 400 radius", d)
	}
}

func TestStepMusic(t *testing.T) {
	tests := []struct {
		name              string
		cur, want, status int
		since             float64
		on                bool
		out               int
	}{
		{"nothing playing adopts at once", 0, 4679, 0, 0, true, 4679},
		{"playing song kept for 74 ticks", 4679, 4673, 0, 74, true, 4679},
		{"swapped after 75 ticks", 4679, 4673, 0, 75, true, 4673},
		{"music off adopts at once", 4679, 4673, 0, 1, false, 4673},
		{"same song stays", 4679, 4679, 0, 500, true, 4679},
		{"target none is held until the delay", 4679, 0, 0, 10, true, 4679},
		{"target none after the delay", 4679, 0, 0, 80, true, 0},
	}
	for _, tc := range tests {
		if got := StepMusic(tc.cur, tc.want, tc.since, tc.on); got != tc.out {
			t.Errorf("%s: StepMusic = %d, want %d", tc.name, got, tc.out)
		}
	}
}

func TestMusicWaitsForSwapDelayAndStopsWhenDisabled(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink)

	e.SetEnv(2)
	e.SetEnv(5)
	e.Advance(2) // 50 ticks: wilderness song still plays

	for _, l := range sink.loops {
		if l == 4673 {
			t.Fatalf("town song started before the swap delay: %v", sink.loops)
		}
	}

	e.Advance(1.5) // 87 ticks in

	found := false

	for _, l := range sink.loops {
		found = found || l == 4673
	}

	if !found {
		t.Fatalf("town song never started: %v", sink.loops)
	}

	sink.musicOff = true
	e.Advance(0.1)

	stoppedSong := false

	for _, st := range sink.stopped {
		stoppedSong = stoppedSong || st == 4673
	}

	if !stoppedSong {
		t.Fatalf("music kept playing with music disabled: stopped %v", sink.stopped)
	}

	sink.musicOff = false
	e.Advance(0.1)

	if sink.loops[len(sink.loops)-1] != 4673 {
		t.Fatalf("music did not resume: %v", sink.loops)
	}
}
