package d2video

import (
	"encoding/binary"
	"testing"
)

func syntheticVideo(t *testing.T, frames int) *BinkDecoder {
	t.Helper()

	hdr := syntheticHeader('i', 32, 16, frames) // 30 fps
	off := len(hdr) + 4*(frames+1)

	var tbl []byte
	for i := 0; i <= frames; i++ {
		tbl = binary.LittleEndian.AppendUint32(tbl, uint32(off+i*2000))
	}

	body := append(append(hdr, tbl...), make([]byte, frames*2000)...)

	d, err := CreateBinkDecoder(body)
	if err != nil {
		t.Fatal(err)
	}

	return d
}

func TestPlayerPacing(t *testing.T) {
	d := syntheticVideo(t, 8)
	p := NewPlayer(d)

	if ch, err := p.Advance(0); err != nil || !ch || d.FrameIndex() != 1 {
		t.Fatalf("first frame must show immediately: %v %v %d", ch, err, d.FrameIndex())
	}

	if ch, _ := p.Advance(0.01); ch {
		t.Fatal("10 ms is less than a 33 ms frame")
	}

	if ch, _ := p.Advance(0.03); !ch || d.FrameIndex() != 2 {
		t.Fatalf("expected the second frame, at %d", d.FrameIndex())
	}

	// a long stall decodes at most maxCatchUp frames
	if ch, _ := p.Advance(10); !ch || d.FrameIndex() != 2+maxCatchUp {
		t.Fatalf("catch-up: frame index %d", d.FrameIndex())
	}

	for i := 0; i < 5 && !p.Done(); i++ {
		_, _ = p.Advance(1)
	}

	if !p.Done() {
		t.Fatal("player should be done after the last frame")
	}
}
