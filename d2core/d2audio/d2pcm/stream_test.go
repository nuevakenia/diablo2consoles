package d2pcm

import (
	"io"
	"math"
	"testing"
)

func frames(s []byte) []int16 {
	out := make([]int16, len(s)/2)
	for i := range out {
		out[i] = int16(uint16(s[2*i]) | uint16(s[2*i+1])<<8)
	}

	return out
}

func TestStreamPassThroughStereo(t *testing.T) {
	s := NewStream(44100, 2, 44100, 0)
	s.Write([]int16{1, -1, 2, -2, 3, -3})

	if s.Buffered() != 3 || s.Written() != 3 {
		t.Fatalf("buffered %d written %d", s.Buffered(), s.Written())
	}

	buf := make([]byte, 12)

	n, err := s.Read(buf)
	if n != 12 || err != nil {
		t.Fatalf("read %d %v", n, err)
	}

	got := frames(buf)
	for i, w := range []int16{1, -1, 2, -2, 3, -3} {
		if got[i] != w {
			t.Fatalf("sample %d = %d, want %d", i, got[i], w)
		}
	}

	if s.Played() != 3 || s.Underruns() != 0 {
		t.Fatalf("played %d underruns %d", s.Played(), s.Underruns())
	}
}

func TestStreamMonoIsDuplicated(t *testing.T) {
	s := NewStream(44100, 1, 44100, 0)
	s.Write([]int16{5, 6})

	buf := make([]byte, 8)
	s.Read(buf)

	if g := frames(buf); g[0] != 5 || g[1] != 5 || g[2] != 6 || g[3] != 6 {
		t.Fatalf("got %v", g)
	}
}

func TestStreamUnderrunCountedOncePerStarvation(t *testing.T) {
	s := NewStream(44100, 2, 44100, 0)
	buf := make([]byte, 16)

	// before any data: silence, but not an underrun
	n, _ := s.Read(buf)
	if n != 16 || s.Underruns() != 0 {
		t.Fatalf("idle read: n=%d underruns=%d", n, s.Underruns())
	}

	s.Write([]int16{9, 9})

	n, _ = s.Read(buf) // 1 frame of data, 3 of silence
	if n != 16 || s.Underruns() != 1 || s.Played() != 1 {
		t.Fatalf("n=%d underruns=%d played=%d", n, s.Underruns(), s.Played())
	}

	if g := frames(buf); g[0] != 9 || g[2] != 0 || g[7] != 0 {
		t.Fatalf("padding is not silence: %v", g)
	}

	s.Read(buf)
	s.Read(buf)

	if s.Underruns() != 1 {
		t.Fatalf("one starvation counted %d times", s.Underruns())
	}

	s.Write(make([]int16, 8))
	s.Read(buf)
	s.Read(buf) // dry again: a second episode

	if s.Underruns() != 2 {
		t.Fatalf("underruns = %d, want 2", s.Underruns())
	}
}

func TestStreamFinishDrainsThenEOF(t *testing.T) {
	s := NewStream(44100, 2, 44100, 0)
	s.Write([]int16{1, 1, 2, 2, 3, 3})
	s.Finish()
	s.Write([]int16{7, 7}) // ignored after Finish

	if s.Drained() {
		t.Fatal("not drained yet")
	}

	buf := make([]byte, 8)

	n, err := s.Read(buf)
	if n != 8 || err != nil {
		t.Fatalf("first read %d %v", n, err)
	}

	n, err = s.Read(buf)
	if n != 4 || err != nil {
		t.Fatalf("last partial read %d %v", n, err)
	}

	if !s.Drained() {
		t.Fatal("should be drained")
	}

	if n, err = s.Read(buf); n != 0 || err != io.EOF {
		t.Fatalf("after drain: %d %v", n, err)
	}

	if s.Underruns() != 0 {
		t.Fatal("a finished stream does not underrun")
	}
}

func TestStreamClockFollowsPlayedMinusLatency(t *testing.T) {
	s := NewStream(1000, 2, 1000, 0.1)
	s.Write(make([]int16, 2*500))

	if s.Clock() != 0 {
		t.Fatalf("clock before play: %v", s.Clock())
	}

	buf := make([]byte, 4*300)
	s.Read(buf)

	if c := s.Clock(); math.Abs(c-0.2) > 1e-9 {
		t.Fatalf("clock %v, want 0.3-0.1", c)
	}

	s.Read(buf)
	s.Read(buf) // starved: the clock stands still at 500 frames
	s.Read(buf)

	if c := s.Clock(); math.Abs(c-0.4) > 1e-9 {
		t.Fatalf("clock %v, want 0.5-0.1", c)
	}
}

func TestStreamResamples(t *testing.T) {
	// 22050 -> 44100: a ramp must come out twice as long and still be monotonic and continuous across Writes
	s := NewStream(22050, 1, 44100, 0)

	in := make([]int16, 200)
	for i := range in {
		in[i] = int16(i * 100)
	}

	s.Write(in[:77])
	s.Write(in[77:])

	if got := s.Written(); got < 395 || got > 400 {
		t.Fatalf("%d frames out, want about 400", got)
	}

	buf := make([]byte, s.Buffered()*4)
	s.Read(buf)
	g := frames(buf)

	prev := int16(-1)

	for i := 0; i < len(g); i += 2 {
		if g[i] < prev || g[i] != g[i+1] {
			t.Fatalf("frame %d: %d after %d", i/2, g[i], prev)
		}

		if prev >= 0 && g[i]-prev > 60 {
			t.Fatalf("jump of %d at output frame %d", g[i]-prev, i/2)
		}

		prev = g[i]
	}
}

func TestStreamResampleDownKeepsDuration(t *testing.T) {
	s := NewStream(48000, 2, 44100, 0)
	s.Write(make([]int16, 2*48000))

	if got := s.Written(); got < 44099 || got > 44101 {
		t.Fatalf("%d frames for one second", got)
	}
}

func TestStreamClose(t *testing.T) {
	s := NewStream(44100, 2, 44100, 0)
	s.Write([]int16{1, 1})
	s.Close()

	if _, err := s.Read(make([]byte, 8)); err != io.EOF {
		t.Fatalf("read after close: %v", err)
	}

	s.Write([]int16{1, 1})

	if s.Buffered() != 0 {
		t.Fatal("write after close buffered data")
	}
}
