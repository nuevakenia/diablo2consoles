package d2audio

import (
	"fmt"
	"testing"
)

type resumeFake struct {
	fakeSink
	from []string
}

func (f *resumeFake) loopFrom(index, ticks int) func() {
	f.from = append(f.from, fmt.Sprintf("%d@%d", index, ticks))
	return func() { f.stopped = append(f.stopped, index) }
}

// A track left after 500 ticks continues at its last 125 tick cue point
// when the hero comes back.
func TestMusicResumesAtCuePoint(t *testing.T) {
	sink := &resumeFake{}

	e := newSoundEnvironment(sink, testEnvs(), func(n int) int { return n / 2 }, func(string, ...interface{}) {})

	e.SetEnv(2) // wilderness song 4679, from the start
	e.Advance(10)
	e.SetEnv(5) // town song 4673 replaces it after the swap delay
	e.Advance(10)

	e.SetEnv(2) // back in the wilderness: the song continues
	e.Advance(10)

	if len(sink.from) != 1 || sink.from[0] != "4679@500" {
		t.Fatalf("resumed tracks %v, want [4679@500]", sink.from)
	}

	// the town song played 250 ticks, it resumes there
	e.SetEnv(5)
	e.Advance(10)

	if len(sink.from) != 2 || sink.from[1] != "4673@250" {
		t.Fatalf("town song did not resume: %v", sink.from)
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
