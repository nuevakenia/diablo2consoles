package d2video

import (
	"sync"
	"time"
)

// runnerPause is how long the decode goroutine sleeps between two Advance calls. A Bink frame of a 640x292 movie
// takes a few milliseconds to decode, and a frame is due every 33-67 ms, so this keeps the picture within a few
// milliseconds of the clock without spinning a core.
const runnerPause = 3 * time.Millisecond

// Runner decodes a Cinematic on its own goroutine, ahead of the game loop: the audio is topped up and the video
// frames due at the clock are decoded without the render/update thread ever waiting for a decoder, so a slow frame
// or a long Bink block costs no stutter in the game loop. The game loop only takes the newest finished frame.
//
// The Cinematic is owned by the goroutine until Stop returns; afterwards its counters can be read again.
type Runner struct {
	c    *Cinematic
	stop chan struct{}
	wg   sync.WaitGroup

	mu    sync.Mutex
	frame []byte // newest frame (a private copy: the decoder reuses its buffer)
	fresh bool   // frame was not taken yet
	clock float64
	done  bool
	err   error
	steps int // Advance calls made
	slow  time.Duration
}

// NewRunner starts decoding c. Call Stop when the cinematic is over or skipped.
func NewRunner(c *Cinematic) *Runner {
	r := &Runner{c: c, stop: make(chan struct{})}

	r.wg.Add(1)

	go r.loop()

	return r
}

func (r *Runner) loop() {
	defer r.wg.Done()

	last := time.Now()

	for {
		select {
		case <-r.stop:
			return
		default:
		}

		now := time.Now()
		dt := now.Sub(last).Seconds()
		last = now

		changed, err := r.c.Advance(dt)

		took := time.Since(now)

		r.mu.Lock()
		r.steps++

		if took > r.slow {
			r.slow = took
		}

		if changed {
			px := r.c.Player().Frame()
			if cap(r.frame) < len(px) {
				r.frame = make([]byte, len(px))
			}

			r.frame = r.frame[:len(px)]
			copy(r.frame, px)
			r.fresh = true
		}

		r.clock = r.c.Clock()
		r.done = r.c.Done()
		r.err = err
		stopNow := r.done || err != nil
		r.mu.Unlock()

		if stopNow {
			return
		}

		select {
		case <-r.stop:
			return
		case <-time.After(runnerPause):
		}
	}
}

// TakeFrame calls fn with the newest frame if it is new since the last call (fn runs under the runner's lock, keep it
// short: it uploads the pixels). It reports whether fn ran.
func (r *Runner) TakeFrame(fn func(rgba []byte)) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.fresh {
		return false
	}

	r.fresh = false
	fn(r.frame)

	return true
}

// Clock is the media time of the picture in seconds, as of the last decode step.
func (r *Runner) Clock() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.clock
}

// Done reports that the last frame was decoded, or the decoder failed (see Err).
func (r *Runner) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.done || r.err != nil
}

// Err is the error that ended the playback, nil otherwise.
func (r *Runner) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.err
}

// SlowestStep is the longest single decode step so far.
func (r *Runner) SlowestStep() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.slow
}

// Stop ends the goroutine and waits for it. It is safe to call more than once.
func (r *Runner) Stop() {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}

	r.wg.Wait()
}
