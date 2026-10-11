package d2video

import "io"

// maxCatchUp bounds how many frames are decoded in one Advance when the host stalls; the rest of the
// elapsed time is dropped so a slow frame does not make the video race to catch up.
const maxCatchUp = 4

// Player paces a decoder against wall-clock time.
type Player struct {
	dec     *BinkDecoder
	elapsed float64 // seconds since the last frame was shown
	frame   []byte
	started bool
	done    bool
}

// NewPlayer wraps a decoder.
func NewPlayer(dec *BinkDecoder) *Player { return &Player{dec: dec} }

// Size is the video size in pixels.
func (p *Player) Size() (w, h int) { return int(p.dec.VideoWidth), int(p.dec.VideoHeight) }

// Done reports whether the last frame has been shown.
func (p *Player) Done() bool { return p.done }

// Frame is the most recent RGBA frame (nil before the first Advance).
func (p *Player) Frame() []byte { return p.frame }

// Advance moves the clock by dt seconds. It returns true when a new frame is available from Frame.
func (p *Player) Advance(dt float64) (bool, error) {
	if p.done {
		return false, nil
	}

	frameTime := float64(p.dec.FPSDen) / float64(p.dec.FPSNum)

	if !p.started {
		p.started = true
		p.elapsed = frameTime // show the first frame at once
	} else {
		p.elapsed += dt
	}

	changed := false

	for n := 0; p.elapsed >= frameTime; n++ {
		if n >= maxCatchUp {
			p.elapsed = 0

			break
		}

		p.elapsed -= frameTime

		px, err := p.dec.NextFrame()
		if err == io.EOF {
			p.done = true

			return changed, nil
		}

		if err != nil {
			p.done = true

			return changed, err
		}

		p.frame = px
		changed = true
	}

	return changed, nil
}
