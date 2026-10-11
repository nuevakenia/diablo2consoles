package d2video

import "testing"

type fakeSink struct {
	written  int64
	played   int64
	finished bool
	writes   int
}

func (f *fakeSink) Write(s []int16) { f.written += int64(len(s) / 2); f.writes++ }
func (f *fakeSink) Buffered() int   { return int(f.written - f.played) }
func (f *fakeSink) Played() int64   { return f.played }
func (f *fakeSink) Clock() float64  { return float64(f.played) / 44100 }
func (f *fakeSink) Drained() bool   { return f.finished && f.played >= f.written }
func (f *fakeSink) Finish()         { f.finished = true }

func (f *fakeSink) play(seconds float64) {
	f.played += int64(seconds * 44100)
	if f.played > f.written {
		f.played = f.written
	}
}

// silentAudioFile is a 30 fps file of n frames whose audio packets are one silent RDFT block each.
func silentAudioFile(t *testing.T, n int) *BinkDecoder {
	t.Helper()

	ad, _ := NewBinkAudioDecoder(44100, 2, false, false)
	packets := make([][]byte, n)

	for i := range packets {
		w := &bitWriter{}
		encodeBlock(w, ad, make([]synthBlock, 1))
		packets[i] = audioPacket(w.buf, 3840*2)
	}

	return audioFile(t, n, packets)
}

func TestCinematicFollowsAudioClock(t *testing.T) {
	d := silentAudioFile(t, 40) // 40 frames at 30 fps = 1.33 s of video, 40 blocks = 1.74 s of audio
	sink := &fakeSink{}

	c, err := NewCinematic(d, sink, 44100)
	if err != nil {
		t.Fatal(err)
	}

	// the soundtrack is queued ahead of playback, and until the device takes a sample the picture waits at frame 0
	if ch, err := c.Advance(0.5); err != nil || !ch || d.FrameIndex() != 1 {
		t.Fatalf("first frame: %v %v %d", ch, err, d.FrameIndex())
	}

	if sink.Buffered() < 26000 || sink.Buffered() > 26460+1920*2 {
		t.Fatalf("lead of %d frames queued, want about 0.6 s", sink.Buffered())
	}

	if _, _ = c.Advance(0.2); d.FrameIndex() != 1 {
		t.Fatalf("the picture ran ahead of a silent device: frame %d", d.FrameIndex())
	}

	// the device takes 0.2 s: frame k is due at k/30, so frames 0..6 are out
	sink.play(0.2)
	c.Advance(0.016)

	if d.FrameIndex() != 7 {
		t.Fatalf("after 0.2 s of audio the picture is at frame index %d, want 7", d.FrameIndex())
	}

	// real time passing without the device taking anything (starved) does not move the picture
	for i := 0; i < 30; i++ {
		c.Advance(0.016)
	}

	if d.FrameIndex() > 7+0 && sink.Played() == int64(0.2*44100) && d.FrameIndex() != 7 {
		t.Fatalf("picture moved without audio: %d", d.FrameIndex())
	}

	// play on to the end: all packets get decoded, the sink is finished, the video completes
	for i := 0; i < 400 && !c.Done(); i++ {
		sink.play(0.016)
		if _, err := c.Advance(0.016); err != nil {
			t.Fatal(err)
		}
	}

	if !c.Done() {
		t.Fatal("cinematic did not finish")
	}

	if !sink.finished || c.DecodedFrames != 40*1920 || sink.written != 40*1920 {
		t.Fatalf("finished=%v decoded=%d written=%d", sink.finished, c.DecodedFrames, sink.written)
	}

	if c.BadPackets != 0 {
		t.Fatalf("bad packets: %d %v", c.BadPackets, c.LastAudioErr)
	}
}

func TestCinematicWithoutAudioUsesWallClock(t *testing.T) {
	d := silentAudioFile(t, 30)

	c, err := NewCinematic(d, nil, 44100)
	if err != nil {
		t.Fatal(err)
	}

	c.Advance(5) // the first call follows the load and does not count
	c.Advance(0.5)

	for i := 0; i < 4; i++ { // catching up is bounded per call
		c.Advance(0)
	}

	if d.FrameIndex() != 16 { // 0.5 s * 30 fps: frames 0..15 are due
		t.Fatalf("frame index %d", d.FrameIndex())
	}

	if c.HasAudio() {
		t.Fatal("no sink, no audio")
	}
}

func TestCinematicDropsAudioOnDeadDevice(t *testing.T) {
	d := silentAudioFile(t, 90) // 3 s
	sink := &fakeSink{}
	c, _ := NewCinematic(d, sink, 44100)

	for i := 0; i < 60; i++ { // 1.6 s of updates, the device never takes a sample
		c.Advance(0.027)
	}

	if c.HasAudio() || c.AudioErr == nil {
		t.Fatalf("the dead device should have been given up: %v", c.AudioErr)
	}

	if !sink.finished {
		t.Fatal("the abandoned sink must be finished")
	}

	at := d.FrameIndex()

	for i := 0; i < 20; i++ {
		c.Advance(0.05)
	}

	if d.FrameIndex() <= at {
		t.Fatalf("picture stuck at %d after falling back to the wall clock", at)
	}
}

func TestCinematicSurvivesBadPacket(t *testing.T) {
	ad, _ := NewBinkAudioDecoder(44100, 2, false, false)
	good := &bitWriter{}
	encodeBlock(good, ad, make([]synthBlock, 1))

	packets := [][]byte{audioPacket(good.buf, 0), audioPacket(good.buf[:40], 0), audioPacket(good.buf, 0)}
	d := audioFile(t, 3, packets)
	sink := &fakeSink{}
	c, _ := NewCinematic(d, sink, 44100)
	c.Advance(0.1)

	if c.BadPackets != 1 || c.LastAudioErr == nil || !sink.finished {
		t.Fatalf("bad=%d err=%v finished=%v", c.BadPackets, c.LastAudioErr, sink.finished)
	}

	if sink.written != 2*1920 {
		t.Fatalf("the good packets are still played: %d frames", sink.written)
	}
}

func TestCinematicStaysPutAfterAudioEnds(t *testing.T) {
	// the sound is shorter than the picture: the wall clock carries the video on once the sink is drained
	ad, _ := NewBinkAudioDecoder(44100, 2, false, false)
	w := &bitWriter{}
	encodeBlock(w, ad, make([]synthBlock, 1))
	packets := [][]byte{audioPacket(w.buf, 0), nil, nil, nil, nil, nil, nil, nil, nil, nil}
	d := audioFile(t, 10, packets)
	sink := &fakeSink{}
	c, _ := NewCinematic(d, sink, 44100)

	c.Advance(0)
	sink.play(1)

	for i := 0; i < 200 && !c.Done(); i++ {
		c.Advance(0.02)
	}

	if !c.Done() {
		t.Fatal("video should complete on the wall clock after the sound ended")
	}
}
