package d2sfx

import "testing"

func TestIndexPlaying(t *testing.T) {
	h := newHarness([]Row{row(1, nil), row(3000, nil), row(4660, func(r *Row) { r.Loop = true })}, 4)

	if h.b.IndexPlaying(0xb76, 0x1230, true) {
		t.Fatal("nothing plays yet")
	}

	h.b.Play(Request{Index: 1})

	if h.b.IndexPlaying(0xb76, 0x1230, false) {
		t.Error("a sound outside the range does not count")
	}

	h.b.Play(Request{Index: 3000})

	if !h.b.IndexPlaying(0xb76, 0x1230, false) {
		t.Error("a playing speech row must count")
	}

	if h.b.IndexPlaying(0x1231, 0x125a, true) {
		t.Error("no music yet")
	}

	h.b.Play(Request{Index: 4660})

	if !h.b.IndexPlaying(0x1231, 0x125a, true) {
		t.Error("music plays")
	}
}

// A line waiting for its start delay counts for the "not finished" test only.
func TestIndexPlayingWaiting(t *testing.T) {
	h := newHarness([]Row{row(0, nil), row(3000, nil)}, 2)

	h.b.Play(Request{Index: 3000, Delay: 10})

	if h.b.IndexPlaying(0xb76, 0x1230, false) {
		t.Error("a waiting line is not playing")
	}

	if !h.b.IndexPlaying(0xb76, 0x1230, true) {
		t.Error("a waiting line is still active")
	}
}
