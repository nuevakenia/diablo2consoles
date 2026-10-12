package d2audio

import (
	"fmt"
	"testing"
)

type resumeFake struct {
	fakeSink
	from    []string
	blocks  map[int][3]int
	active  bool
	stings  []int
	noFades []bool
}

func (f *resumeFake) loopFrom(index, ticks int, noFade bool) func() {
	f.from = append(f.from, fmt.Sprintf("%d@%d", index, ticks))
	f.noFades = append(f.noFades, noFade)

	return func() { f.stopped = append(f.stopped, index) }
}

func (f *resumeFake) cues(index int) [3]int {
	if b, ok := f.blocks[index]; ok {
		return b
	}

	return [3]int{-1, -1, -1}
}

func (f *resumeFake) musicActive() bool { return f.active }

func (f *resumeFake) sting(index int) { f.stings = append(f.stings, index) }

// The exe refreshes the resume position every 125 ticks to the NEXT cue block after the stream
// position, so a track left early continues at its first block (Block 1 = 500000 samples = 566 ticks).
func TestMusicResumesAtNextCue(t *testing.T) {
	sink := &resumeFake{blocks: map[int][3]int{4679: {500000, 900000, -1}}}

	e := newSoundEnvironment(sink, testEnvs(), func(n int) int { return n / 2 }, func(string, ...interface{}) {})

	e.SetEnv(2) // wilderness song 4679, from the start, no fade (nothing played before)
	e.Advance(5)
	e.Advance(5) // 250 ticks: beats stored on the way
	e.SetEnv(5)  // town song 4673 replaces it after the swap delay
	e.Advance(10)

	e.SetEnv(2) // back in the wilderness: the song continues at the cue after the last beat
	e.Advance(10)

	if len(sink.from) < 3 || sink.from[0] != "4679@0" || sink.from[2] != "4679@566" {
		t.Fatalf("started tracks %v", sink.from)
	}

	if !sink.noFades[0] || sink.noFades[2] {
		t.Errorf("only the very first track starts without fade: %v", sink.noFades)
	}
}

func TestNextCue(t *testing.T) {
	b := [3]int{100, 200, 300}

	for pos, want := range map[int]int{-1: 0, 0: 100, 99: 100, 100: 200, 199: 200, 200: 300, 299: 300, 300: 0, 1000: 0} {
		if got := NextCue(pos, b); got != want {
			t.Errorf("NextCue(%d) = %d, want %d", pos, got, want)
		}
	}

	if NextCue(50, [3]int{-1, -1, -1}) != 0 {
		t.Error("unset blocks resume at the start")
	}

	if got := NextCue(50, [3]int{100, -1, -1}); got != 100 {
		t.Errorf("one block: %d", got)
	}
}

// A quest line stops the music for its Silence ticks, plays the sting on time and the track comes back
// from the chosen cue block with its fade in.
func TestQuestMusicDucks(t *testing.T) {
	sink := &resumeFake{blocks: map[int][3]int{4679: {500000, 900000, -1}}}

	e := newSoundEnvironment(sink, testEnvs(), func(n int) int { return n / 2 }, func(string, ...interface{}) {})
	e.SetEnv(2)
	e.Advance(0.08) // 2 ticks

	e.QuestMusic(QuestPlan{Silence: 100, StingDelay: 25, Cue: 1}, 4688)

	if len(sink.stopped) != 1 {
		t.Fatalf("music not stopped: %v", sink.stopped)
	}

	e.Advance(0.48) // 12 more ticks
	if len(sink.stings) != 0 {
		t.Fatal("sting too early")
	}

	e.Advance(1.52) // 50 ticks since the call
	if len(sink.stings) != 1 || sink.stings[0] != 4688 {
		t.Fatalf("stings %v", sink.stings)
	}

	if len(sink.from) != 1 {
		t.Fatalf("music restarted during the silence: %v", sink.from)
	}

	e.Advance(3) // 125 ticks: the silence is over
	if len(sink.from) != 2 || sink.from[1] != "4679@566" || sink.noFades[1] {
		t.Fatalf("music did not resume at block 1: %v %v", sink.from, sink.noFades)
	}
}

// Without a seekable sink the music simply starts over (older sinks).
func TestMusicWithoutResumeSink(t *testing.T) {
	sink := &fakeSink{}
	e := newTestEnv(sink)

	e.SetEnv(2)
	e.Advance(10)
	e.SetEnv(5)
	e.Advance(10)
	e.SetEnv(2)
	e.Advance(10)

	if fmt.Sprint(sink.loops) != "[4679 70 4673 4679]" {
		t.Fatalf("loops %v", sink.loops)
	}
}
