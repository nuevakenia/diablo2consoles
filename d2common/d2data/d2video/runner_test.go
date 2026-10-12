package d2video

import (
	"testing"
	"time"
)

func TestRunnerDecodesAheadOfTheCaller(t *testing.T) {
	d := syntheticVideo(t, 6) // no sound: the wall clock drives it
	c, err := NewCinematic(d, nil, 44100)
	if err != nil {
		t.Fatal(err)
	}

	r := NewRunner(c)
	defer r.Stop()

	got := 0
	deadline := time.Now().Add(5 * time.Second)

	for !r.Done() && time.Now().Before(deadline) {
		if r.TakeFrame(func(px []byte) {
			if len(px) == 0 {
				t.Error("empty frame")
			}
		}) {
			got++
		}

		time.Sleep(time.Millisecond)
	}

	if !r.Done() || r.Err() != nil {
		t.Fatalf("not done: %v", r.Err())
	}

	if got == 0 {
		t.Fatal("no frame was handed over")
	}

	r.Stop()
	r.Stop() // twice is fine

	if d.FrameIndex() != 6 {
		t.Fatalf("decoded %d frames, want 6", d.FrameIndex())
	}
}

func TestRunnerStopBeforeEnd(t *testing.T) {
	d := syntheticVideo(t, 600)
	c, _ := NewCinematic(d, nil, 44100)
	r := NewRunner(c)

	time.Sleep(20 * time.Millisecond)
	r.Stop()

	if r.Done() {
		t.Fatal("a stopped runner of a long movie is not done")
	}

	n := d.FrameIndex()
	time.Sleep(50 * time.Millisecond)

	if d.FrameIndex() != n {
		t.Fatal("decoding went on after Stop")
	}
}
