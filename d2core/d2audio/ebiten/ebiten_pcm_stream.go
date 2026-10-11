package ebiten

import (
	"errors"
	"os"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio/d2pcm"
)

// outputLatencySeconds estimates the delay between Ebiten's player reading samples from a stream and them being
// audible: the Ebiten player pulls 2048 byte chunks (11.6 ms) through one in flight chunk (11.6 ms) into the
// device buffer of 8192 bytes (46 ms) it uses on desktops. Not measured with a microphone; the figure only places
// the picture against the sound, and a few tens of milliseconds are not visible.
const outputLatencySeconds = 0.07

var _ d2interface.PCMStreamer = &AudioProvider{}

// pcmStream is a d2pcm.Stream played by an Ebiten player.
type pcmStream struct {
	*d2pcm.Stream
	player *audio.Player
}

// NewPCMStream starts a raw PCM stream (see d2interface.PCMStreamer). Under OD2_AUTOTEST_MUTE the stream is
// played at volume 0: the device still pulls the data, so the clock and the underrun count are real, but silent.
func (eap *AudioProvider) NewPCMStream(sampleRate, channels int) (d2interface.PCMStream, error) {
	if eap.audioContext == nil {
		return nil, errors.New("no audio context")
	}

	s := d2pcm.NewStream(sampleRate, channels, eap.audioContext.SampleRate(), outputLatencySeconds)

	player, err := audio.NewPlayer(eap.audioContext, s)
	if err != nil {
		return nil, err
	}

	if os.Getenv("OD2_AUTOTEST_MUTE") != "" {
		player.SetVolume(0)
	} else {
		player.SetVolume(eap.sfxVolume)
	}

	player.Play()

	return &pcmStream{Stream: s, player: player}, nil
}

// Close stops the player and drops the queue.
func (p *pcmStream) Close() {
	p.Stream.Close()

	if p.player != nil {
		_ = p.player.Close()
		p.player = nil
	}
}
