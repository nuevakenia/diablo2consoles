package d2video

// AudioSink is where the decoded soundtrack goes: a stream on the sound device that also reports how much of it
// has been played, which makes it the master clock of a cinematic.
type AudioSink interface {
	// Write queues interleaved 16 bit samples.
	Write(samples []int16)
	// Buffered is the number of sample frames queued and not yet taken by the device.
	Buffered() int
	// Played is the number of sample frames the device has taken.
	Played() int64
	// Clock is the playback position in seconds; it stands still while the sink is starved.
	Clock() float64
	// Drained reports that Finish was called and everything has been taken by the device.
	Drained() bool
	// Finish says no more samples follow.
	Finish()
}

const (
	// audioLeadSeconds is how much decoded audio is kept queued ahead of the device. The first Bink frame carries
	// more than that by itself; the rest is topped up every update, so a 100 ms stall of the game loop is harmless.
	audioLeadSeconds = 0.6
	// audioStartTimeout is how long a sink may take to play its first sample (a machine without a sound device
	// never does) before the cinematic gives up on the audio and runs on the wall clock.
	audioStartTimeout = 1.5
	// maxPacketsPerAdvance bounds the audio decoded in one update.
	maxPacketsPerAdvance = 24
)

// Cinematic plays a Bink file: the video frames follow the audio clock (or the wall clock without audio), and the
// audio packets are decoded a little ahead of what the device has taken. It has no dependency on a window or a
// sound device, so the synchronisation is unit tested with a fake sink.
type Cinematic struct {
	player *Player
	dec    *BinkDecoder
	audio  *BinkAudioDecoder
	sink   AudioSink
	rate   int // sample rate of the sink

	wall      float64 // seconds of Advance calls (the first one does not count: it follows the load)
	origin    float64 // wall time at which the wall clock (without audio) started
	started   bool
	nextAudio int
	finished  bool
	pcm       []int16

	drainWall, drainBase float64
	drained              bool

	// DecodedFrames is the number of audio sample frames decoded so far (at the track's own rate).
	DecodedFrames int64
	// AudioErr is the error that made the cinematic give up on the audio, nil while it plays.
	AudioErr error
	// BadPackets counts audio packets that failed to decode (and the last such error).
	BadPackets   int
	LastAudioErr error
}

// NewCinematic creates a cinematic from a decoder. sink may be nil (silent, wall clock); sinkRate is the rate the
// sink plays at (the track is resampled by the sink when it differs).
func NewCinematic(dec *BinkDecoder, sink AudioSink, sinkRate int) (*Cinematic, error) {
	c := &Cinematic{player: NewPlayer(dec), dec: dec, sink: sink, rate: sinkRate}

	if sink != nil {
		ad, err := dec.NewAudioDecoder(0)
		if err != nil {
			return nil, err
		}

		c.audio = ad
	}

	return c, nil
}

// Player is the video part, for its size and current frame.
func (c *Cinematic) Player() *Player { return c.player }

// Done reports that the last video frame has been shown.
func (c *Cinematic) Done() bool { return c.player.Done() }

// HasAudio reports that the soundtrack is playing and drives the clock.
func (c *Cinematic) HasAudio() bool { return c.sink != nil }

// Clock is the media time in seconds that the picture follows.
func (c *Cinematic) Clock() float64 {
	if c.sink == nil {
		return c.wall - c.origin
	}

	if c.sink.Drained() {
		if !c.drained {
			c.drained, c.drainWall, c.drainBase = true, c.wall, c.sink.Clock()
		}

		// the sound is over (the device latency is still to run out): carry on with the wall clock
		return c.drainBase + c.wall - c.drainWall
	}

	return c.sink.Clock()
}

// Advance moves on by dt seconds of real time: tops up the audio and shows the video frames due at the clock. It
// returns true when a new frame is available from Player().Frame().
func (c *Cinematic) Advance(dt float64) (bool, error) {
	if c.started {
		c.wall += dt
	}

	c.started = true

	if c.sink != nil {
		c.feed()

		if c.sink != nil && c.sink.Played() == 0 && c.wall > audioStartTimeout {
			c.dropAudio(errNoAudioDevice)
		}
	}

	return c.player.AdvanceTo(c.Clock())
}

type cinematicError string

func (e cinematicError) Error() string { return string(e) }

const errNoAudioDevice = cinematicError("the sound device did not take any audio")

// dropAudio abandons the soundtrack: the wall clock takes over at the current media time.
func (c *Cinematic) dropAudio(err error) {
	now := c.Clock()
	c.sink.Finish()
	c.sink, c.AudioErr = nil, err
	c.origin = c.wall - now
}

func (c *Cinematic) feed() {
	if c.finished {
		return
	}

	want := int(audioLeadSeconds * float64(c.rate))

	for n := 0; c.sink.Buffered() < want && n < maxPacketsPerAdvance; n++ {
		if c.nextAudio >= c.dec.NumFrames() {
			c.finished = true
			c.sink.Finish()

			return
		}

		pkt, err := c.dec.AudioPacket(c.nextAudio, 0)
		c.nextAudio++
		c.pcm = c.pcm[:0]

		if err == nil {
			// blocks decoded before an error are kept: a damaged packet costs a glitch, not the soundtrack
			c.pcm, err = c.audio.Decode(c.pcm, pkt)
		}

		if err != nil {
			c.BadPackets++
			c.LastAudioErr = err
		}

		c.DecodedFrames += int64(len(c.pcm) / c.audio.Channels())
		c.sink.Write(c.pcm)
	}
}
