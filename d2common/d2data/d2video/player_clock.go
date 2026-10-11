package d2video

import (
	"io"
	"math"
)

// maxClockCatchUp bounds how many frames one AdvanceTo decodes; a video that falls behind its clock catches up over
// several calls instead of stalling one update. Time is never dropped: the picture follows the clock.
const maxClockCatchUp = 6

// AdvanceTo shows the frames due at media time t (seconds since the start): frame k is due at k/fps, so frame 0 is
// shown at t=0. The clock is the master (the audio clock of the cinematic); frames are decoded in order, which a
// Bink inter frame requires, and a stalled host decodes up to maxClockCatchUp of them per call until it has caught
// up. It returns true when a new frame is available from Frame. Do not mix it with Advance.
func (p *Player) AdvanceTo(t float64) (bool, error) {
	if p.done {
		return false, nil
	}

	frameTime := float64(p.dec.FPSDen) / float64(p.dec.FPSNum)
	due := int(math.Floor(t/frameTime)) + 1

	changed := false

	for n := 0; p.dec.FrameIndex() < due && n < maxClockCatchUp; n++ {
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

// Lag is how many frames the picture is behind a clock reading of t seconds (0 when on time).
func (p *Player) Lag(t float64) int {
	frameTime := float64(p.dec.FPSDen) / float64(p.dec.FPSNum)
	lag := int(math.Floor(t/frameTime)) + 1 - p.dec.FrameIndex()

	if lag < 0 || p.done {
		return 0
	}

	return lag
}

// FrameTime is the duration of one video frame in seconds.
func (p *Player) FrameTime() float64 { return float64(p.dec.FPSDen) / float64(p.dec.FPSNum) }

// Decoder gives access to the decoder (audio tracks, frame count) behind the player.
func (p *Player) Decoder() *BinkDecoder { return p.dec }
