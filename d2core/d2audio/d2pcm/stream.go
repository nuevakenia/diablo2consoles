// Package d2pcm is a raw PCM stream source for the audio backends: a producer (the Bink audio decoder of a
// cinematic) writes interleaved 16 bit samples, the device side reads 16 bit little endian stereo bytes at the
// device rate. The package has no dependency on a sound device, so the buffering, the played-sample clock and the
// underrun counting are unit tested. The Ebiten backend wraps a Stream in a Player.
package d2pcm

import (
	"io"
	"sync"
)

const bytesPerFrame = 4 // stereo, 16 bit

// Stream is a FIFO of PCM between a producer and an audio device. It is safe for use by one producer and one
// reader goroutine.
type Stream struct {
	mu sync.Mutex

	inRate, outRate, inChannels int

	queue []byte // stereo s16le at outRate, not yet read
	head  int

	// linear resampler state (only used when inRate != outRate)
	pos        float64 // position in input frames of the next output frame, relative to the last frame fed
	last       [2]float64
	haveLast   bool
	resampling bool

	written   int64 // frames queued, at the output rate
	played    int64 // frames handed to the device
	underruns int
	starved   bool
	finished  bool
	closed    bool

	latency float64 // seconds between a frame being read by the device side and being heard
}

// NewStream creates a stream for input of the given rate and channel count (1 or 2) that is read at outRate.
// latency is the estimated delay (seconds) between Read handing samples over and the speakers playing them.
func NewStream(inRate, inChannels, outRate int, latency float64) *Stream {
	if inChannels < 1 || inChannels > 2 {
		inChannels = 2
	}

	return &Stream{
		inRate: inRate, outRate: outRate, inChannels: inChannels,
		resampling: inRate != outRate && inRate > 0,
		latency:    latency,
	}
}

// Write queues interleaved samples (a whole number of frames of the stream's channel count).
func (s *Stream) Write(samples []int16) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.finished {
		return
	}

	n := len(samples) / s.inChannels

	if !s.resampling {
		for i := 0; i < n; i++ {
			l, r := s.frame(samples, i)
			s.push(l, r)
		}

		return
	}

	// linear interpolation between consecutive input frames; s.pos is the output frame position measured from the
	// previous input frame (last), so a block boundary is seamless
	step := float64(s.inRate) / float64(s.outRate)

	for i := 0; i < n; i++ {
		l, r := s.frame(samples, i)
		cur := [2]float64{float64(l), float64(r)}

		if !s.haveLast {
			s.last, s.haveLast, s.pos = cur, true, 0
			s.push(l, r)
			s.pos = step

			continue
		}

		for s.pos < 1 {
			a := s.pos
			s.push(int16(s.last[0]+(cur[0]-s.last[0])*a), int16(s.last[1]+(cur[1]-s.last[1])*a))
			s.pos += step
		}

		s.pos--
		s.last = cur
	}
}

func (s *Stream) frame(samples []int16, i int) (l, r int16) {
	if s.inChannels == 1 {
		return samples[i], samples[i]
	}

	return samples[2*i], samples[2*i+1]
}

func (s *Stream) push(l, r int16) {
	s.queue = append(s.queue, byte(l), byte(l>>8), byte(r), byte(r>>8))
	s.written++
}

// Read fills p with queued stereo s16le data. When the queue runs dry before Finish the rest of p is silence
// (and the first such read of a starvation is counted as an underrun); after Finish and a drained queue it
// returns io.EOF. It never blocks.
func (s *Stream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, io.EOF
	}

	p = p[:len(p)-len(p)%bytesPerFrame]
	avail := len(s.queue) - s.head

	if avail == 0 && s.finished {
		return 0, io.EOF
	}

	n := copy(p, s.queue[s.head:])
	s.head += n
	s.played += int64(n / bytesPerFrame)

	if s.head > 1<<16 && s.head*2 > len(s.queue) {
		s.queue = append(s.queue[:0], s.queue[s.head:]...)
		s.head = 0
	}

	if n < len(p) {
		for i := n; i < len(p); i++ {
			p[i] = 0
		}

		if s.finished {
			return n, nil
		}

		// silence is only an underrun once data has been played: before the first Write the device just waits
		if s.written > 0 && !s.starved {
			s.starved = true
			s.underruns++
		}

		return len(p), nil
	}

	s.starved = false

	return n, nil
}

// Finish says that no more samples will be written; Read returns io.EOF once the queue is drained.
func (s *Stream) Finish() {
	s.mu.Lock()
	s.finished = true
	s.mu.Unlock()
}

// Close drops the queue and makes Read return io.EOF.
func (s *Stream) Close() {
	s.mu.Lock()
	s.closed = true
	s.queue, s.head = nil, 0
	s.mu.Unlock()
}

// Buffered is the number of frames (at the output rate) queued and not yet read.
func (s *Stream) Buffered() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return (len(s.queue) - s.head) / bytesPerFrame
}

// Written is the number of frames queued so far, at the output rate.
func (s *Stream) Written() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.written
}

// Played is the number of real (not padding) frames handed to the device so far.
func (s *Stream) Played() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.played
}

// Underruns counts the times the device asked for data while the queue was empty and input was not finished.
func (s *Stream) Underruns() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.underruns
}

// Drained reports that the input is finished and everything has been handed to the device.
func (s *Stream) Drained() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.finished && len(s.queue) == s.head
}

// Clock is the playback position in seconds: the frames handed to the device minus the estimated device latency.
// It stands still while the stream is starved, so a video following it waits with the sound.
func (s *Stream) Clock() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := float64(s.played)/float64(s.outRate) - s.latency
	if t < 0 {
		return 0
	}

	return t
}

// OutRate is the rate the device side reads at.
func (s *Stream) OutRate() int { return s.outRate }
