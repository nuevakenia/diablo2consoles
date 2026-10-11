package d2video

import (
	"encoding/binary"
	"testing"
)

func TestPlayerAdvanceTo(t *testing.T) {
	d := syntheticVideo(t, 20) // 30 fps
	p := NewPlayer(d)

	if ch, err := p.AdvanceTo(0); err != nil || !ch || d.FrameIndex() != 1 {
		t.Fatalf("frame 0 is due at t=0: %v %v %d", ch, err, d.FrameIndex())
	}

	if ch, _ := p.AdvanceTo(0.02); ch {
		t.Fatal("frame 1 is not due before 33 ms")
	}

	if ch, _ := p.AdvanceTo(0.034); !ch || d.FrameIndex() != 2 {
		t.Fatalf("frame 1 due at 33 ms, index %d", d.FrameIndex())
	}

	// a clock that jumps ahead: decoding is bounded per call but no time is lost
	if _, _ = p.AdvanceTo(0.5); d.FrameIndex() != 2+maxClockCatchUp {
		t.Fatalf("bounded catch-up, index %d", d.FrameIndex())
	}

	if lag := p.Lag(0.5); lag != 16-(2+maxClockCatchUp)+1-0 && lag <= 0 {
		t.Fatalf("lag %d", lag)
	}

	for i := 0; i < 3; i++ {
		_, _ = p.AdvanceTo(0.5)
	}

	if d.FrameIndex() != 16 || p.Lag(0.5) != 0 {
		t.Fatalf("after catching up: index %d lag %d", d.FrameIndex(), p.Lag(0.5))
	}

	// the clock going backwards never rewinds
	if ch, _ := p.AdvanceTo(0.1); ch || d.FrameIndex() != 16 {
		t.Fatal("an earlier clock must not change the picture")
	}

	for i := 0; i < 5 && !p.Done(); i++ {
		_, _ = p.AdvanceTo(10)
	}

	if !p.Done() {
		t.Fatal("not done after the clock passed the end")
	}
}

func audioFile(t *testing.T, frames int, packets [][]byte) *BinkDecoder {
	t.Helper()

	hdr := syntheticHeader('i', 32, 16, frames)
	// patch the number of audio tracks and add one track: sizes, rate+flags, id
	binary.LittleEndian.PutUint32(hdr[len(hdr)-4:], 1)
	hdr = binary.LittleEndian.AppendUint32(hdr, 2<<16)         // unknown, channels
	hdr = binary.LittleEndian.AppendUint16(hdr, 44100)         // rate
	hdr = binary.LittleEndian.AppendUint16(hdr, 0x2000|0x4000) // stereo, 16 bit, RDFT
	hdr = binary.LittleEndian.AppendUint32(hdr, 7)             // id
	off := len(hdr) + 4*(frames+1)
	table := make([]byte, 0, 4*(frames+1))
	body := make([]byte, 0)

	for i := 0; i < frames; i++ {
		table = binary.LittleEndian.AppendUint32(table, uint32(off+len(body)))
		body = binary.LittleEndian.AppendUint32(body, uint32(len(packets[i])))
		body = append(body, packets[i]...)
		body = append(body, make([]byte, 100)...) // video
		// the low bit of a frame offset is the keyframe flag, so frames start on even offsets
		for len(body)%2 != 0 {
			body = append(body, 0)
		}
	}

	table = binary.LittleEndian.AppendUint32(table, uint32(off+len(body)))

	d, err := CreateBinkDecoder(append(append(hdr, table...), body...))
	if err != nil {
		t.Fatal(err)
	}

	return d
}

func TestAudioPacketAndTrackDecoder(t *testing.T) {
	packets := [][]byte{{1, 2, 3, 4, 5}, {}, {9, 9, 9}}
	d := audioFile(t, 3, packets)

	if len(d.AudioTracks) != 1 || d.AudioTracks[0].AudioSampleRateHz != 44100 || !d.AudioTracks[0].Stereo ||
		d.AudioTracks[0].Algorithm != BinkAudioAlgorithmFFT {
		t.Fatalf("track: %+v", d.AudioTracks)
	}

	for i, want := range packets {
		got, err := d.AudioPacket(i, 0)
		if err != nil || string(got) != string(want) {
			t.Errorf("frame %d: %v %v, want %v", i, got, err, want)
		}
	}

	if _, err := d.AudioPacket(3, 0); err == nil {
		t.Error("frame past the end must fail")
	}

	if _, err := d.AudioPacket(0, 1); err == nil {
		t.Error("track 1 does not exist")
	}

	ad, err := d.NewAudioDecoder(0)
	if err != nil || ad.SampleRate() != 44100 || ad.Channels() != 2 || ad.SamplesPerBlock() != 1920 {
		t.Fatalf("audio decoder: %+v %v", ad, err)
	}

	if _, err := d.NewAudioDecoder(1); err == nil {
		t.Error("no second track")
	}

	// NextFrame and AudioPacket agree
	if _, err := d.NextFrame(); err != nil {
		t.Fatal(err)
	}

	if string(d.LastAudio[0]) != string(packets[0]) {
		t.Errorf("LastAudio %v", d.LastAudio[0])
	}
}
