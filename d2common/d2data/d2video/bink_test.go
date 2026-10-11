package d2video

import (
	"encoding/binary"
	"io"
	"os"
	"strconv"
	"testing"
)

func TestBitReaderLSBFirst(t *testing.T) {
	b := newBitReader([]byte{0xA5, 0x0F, 0xFF})

	if got := b.bits(4); got != 0x5 {
		t.Fatalf("low nibble: got %x", got)
	}

	if got := b.bits(8); got != 0xFA {
		t.Fatalf("straddling byte: got %x", got)
	}

	b.align32()

	if b.pos != 32 {
		t.Fatalf("align32: pos %d", b.pos)
	}

	if got := b.bits(5); got != 0 {
		t.Fatalf("reads past the end are zero, got %d", got)
	}
}

func TestVLCTablesArePrefixCodes(t *testing.T) {
	for i := 0; i < 16; i++ {
		kraft := 0.0

		for s := 0; s < 16; s++ {
			kraft += 1 / float64(int(1)<<binkTreeLens[i][s])
		}

		if kraft != 1 {
			t.Errorf("tree %d: Kraft sum %v, want 1", i, kraft)
		}

		// every symbol must decode back from its own code
		for s := 0; s < 16; s++ {
			code := uint32(binkTreeBits[i][s])
			r := newBitReader([]byte{byte(code), byte(code >> 8), 0, 0})

			if got := binkVLCs[i].read(r); got != s || r.pos != int(binkTreeLens[i][s]) {
				t.Errorf("tree %d sym %d: decoded %d using %d bits", i, s, got, r.pos)
			}
		}
	}
}

func TestIDCTDCOnly(t *testing.T) {
	var blk [64]int32

	blk[0] = 100 * 8 << 0 // arbitrary DC, expect a flat block

	idct(&blk)

	for i := 1; i < 64; i++ {
		if blk[i] != blk[0] {
			t.Fatalf("DC-only block not flat at %d: %d vs %d", i, blk[i], blk[0])
		}
	}
}

func TestReadTreeIdentity(t *testing.T) {
	var tr tree

	// vlc number 0 means the identity symbol map
	if err := readTree(newBitReader([]byte{0, 0}), &tr); err != nil {
		t.Fatal(err)
	}

	for i, s := range tr.syms {
		if int(s) != i {
			t.Fatalf("syms[%d] = %d", i, s)
		}
	}
}

// synthetic builds a tiny Bink container with n frames of a given size whose planes are all coded
// as a single "fill" of every block (all bundles empty except block types).
func TestHeaderRejectsGarbage(t *testing.T) {
	if _, err := CreateBinkDecoder([]byte("not a bink file at all.......")); err == nil {
		t.Fatal("expected error")
	}

	if _, err := CreateBinkDecoder(nil); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func syntheticHeader(rev byte, w, h, frames int) []byte {
	hdr := make([]byte, 0, 64)
	hdr = append(hdr, 'B', 'I', 'K', rev)

	put := func(v uint32) { hdr = binary.LittleEndian.AppendUint32(hdr, v) }

	put(0) // file size
	put(uint32(frames))
	put(16)
	put(uint32(frames))
	put(uint32(w))
	put(uint32(h))
	put(30)
	put(1)
	put(0) // flags
	put(0) // audio tracks

	return hdr
}

func TestSyntheticSkipFrames(t *testing.T) {
	const frames = 3

	hdr := syntheticHeader('i', 32, 16, frames)
	table := 4 * (frames + 1)
	off := len(hdr) + table

	var data []byte

	for i := 0; i <= frames; i++ {
		e := uint32(off + i*2000)
		if i == 0 {
			e |= 1 // keyframe
		}

		data = binary.LittleEndian.AppendUint32(data, e)
	}

	body := append(hdr, data...)
	// each frame: 2000 zero bytes. An all-zero stream selects tree 0 everywhere and zero-length bundles,
	// so every block is decoded as block type 0 (skip) of the all-zero previous frame.
	body = append(body, make([]byte, frames*2000)...)

	d, err := CreateBinkDecoder(body)
	if err != nil {
		t.Fatal(err)
	}

	if d.Revision() != 'i' || d.NumFrames() != frames || d.VideoWidth != 32 || d.VideoHeight != 16 || d.FPS != 30 {
		t.Fatalf("header misparsed: %+v", d)
	}

	for i := 0; i < frames; i++ {
		px, err := d.NextFrame()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}

		if len(px) != 32*16*4 {
			t.Fatalf("frame %d: %d bytes", i, len(px))
		}
	}

	if _, err := d.NextFrame(); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestUnsupportedRevision(t *testing.T) {
	if _, err := CreateBinkDecoder(syntheticHeader('b', 32, 16, 1)); err == nil {
		t.Fatal("revision b must be rejected")
	}
}

// TestRealVideo decodes frames of a user-supplied Bink file. OD2_VIDEO_TEST_FILE is the .bik path (extract
// it from d2video.mpq yourself; it is never stored in the repository). OD2_VIDEO_TEST_FRAMES defaults to 600 (the intro fades in from black).
func TestRealVideo(t *testing.T) {
	path := os.Getenv("OD2_VIDEO_TEST_FILE")
	if path == "" {
		t.Skip("OD2_VIDEO_TEST_FILE not set")
	}

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	d, err := CreateBinkDecoder(src)
	if err != nil {
		t.Fatal(err)
	}

	n := 600
	if s := os.Getenv("OD2_VIDEO_TEST_FRAMES"); s != "" {
		n, _ = strconv.Atoi(s)
	}

	if n > d.NumFrames() {
		n = d.NumFrames()
	}

	t.Logf("revision %q %dx%d, %d frames, %d/%d fps, %d audio track(s)",
		d.Revision(), d.VideoWidth, d.VideoHeight, d.NumFrames(), d.FPSNum, d.FPSDen, len(d.AudioTracks))

	lit := 0

	for i := 0; i < n; i++ {
		px, err := d.NextFrame()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}

		if len(px) != int(d.VideoWidth*d.VideoHeight*4) {
			t.Fatalf("frame %d size %d", i, len(px))
		}

		var sum, min, max int
		min = 255

		for p := 0; p < len(px); p += 4 {
			l := int(px[p]) + int(px[p+1]) + int(px[p+2])
			sum += l

			if l/3 < min {
				min = l / 3
			}

			if l/3 > max {
				max = l / 3
			}
		}

		if max-min > 8 {
			lit++
		}
	}

	if lit == 0 {
		t.Fatalf("all %d decoded frames are flat/blank", n)
	}

	t.Logf("%d of %d frames had picture content", lit, n)
}
