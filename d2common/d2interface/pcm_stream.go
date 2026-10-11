package d2interface

// PCMStream plays raw PCM that a producer writes while it plays (the soundtrack of a Bink cinematic). It is also the
// audio clock: Clock stands still while the stream is starved, so a video that follows it stays in sync.
type PCMStream interface {
	// Write queues interleaved 16 bit samples (a whole number of frames of the stream's channel count).
	Write(samples []int16)
	// Buffered is the number of sample frames queued and not yet taken by the sound device.
	Buffered() int
	// Written is the number of sample frames queued so far, at the device rate.
	Written() int64
	// Played is the number of sample frames the device has taken.
	Played() int64
	// Underruns counts the times the device found the queue empty while more input was expected.
	Underruns() int
	// Clock is the playback position in seconds (taken frames minus the estimated device latency).
	Clock() float64
	// Drained reports that Finish was called and everything has been taken by the device.
	Drained() bool
	// Finish says no more samples follow; the stream ends once the queue is played.
	Finish()
	// Close stops the stream and releases the device player.
	Close()
}

// PCMStreamer is implemented by audio providers that can play a PCMStream. It is a separate interface so that
// providers (and test doubles) that only load files do not have to implement it.
type PCMStreamer interface {
	// NewPCMStream starts a stream of interleaved samples (channels 1 or 2) at the given rate. The stream plays
	// as soon as data is written; it is silent when the game runs muted for automated tests.
	NewPCMStream(sampleRate, channels int) (PCMStream, error)
}
