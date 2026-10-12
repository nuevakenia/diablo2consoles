package d2gamescreen

import (
	"image/color"
	"os"
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2data/d2video"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2screen"
)

// skipGrace is how long a new video ignores skip input: the click or key that started it is still being released.
const skipGrace = 0.25

// VideoPlayer plays one Bink cinematic letterboxed on a black screen. It is a game screen of its own (the intro, the
// Cinematics menu) or an overlay that a running Game drives (act-change movies, see game_movie.go). Esc, Space, Enter
// or any mouse click skips it; when it ends (or fails to load) onDone is called once.
//
// The soundtrack (Bink audio) is streamed to the sound device at the sound effect volume and is the clock of the
// picture; without a sound device the video follows the wall clock. Decoding runs on its own goroutine
// (d2video.Runner), the game loop only uploads the newest frame. UNVERIFIED: the original draws no subtitles on
// the movies and none were found in the exe, so none are drawn.
type VideoPlayer struct {
	asset    *d2asset.AssetManager
	renderer d2interface.Renderer
	input    d2interface.InputManager
	audio    d2interface.AudioProvider
	path     string
	onDone   func()

	cin     *d2video.Cinematic
	runner  *d2video.Runner
	player  *d2video.Player
	stream  d2interface.PCMStream
	surface d2interface.Surface
	done    bool
	bound   bool
	overlay bool    // input at high priority: the movie sits over a running game
	age     float64 // seconds on screen

	// OD2_VIDEO_SECONDS=<n> ends the video after n seconds of playback (an automated test); with OD2_AUTOEXIT the
	// game then quits instead of going on to the next screen.
	limit float64
	wall  float64
	maxDt float64 // longest update step seen: a stall of the game loop longer than the queued audio underruns

	*d2util.Logger
}

// CreateVideoPlayer creates a screen that plays the Bink file at the asset path, then calls onDone.
func CreateVideoPlayer(asset *d2asset.AssetManager, renderer d2interface.Renderer, input d2interface.InputManager,
	audio d2interface.AudioProvider, path string, l d2util.LogLevel, onDone func()) *VideoPlayer {
	v := &VideoPlayer{asset: asset, renderer: renderer, input: input, audio: audio, path: path, onDone: onDone}
	v.Logger = d2util.NewLogger()
	v.Logger.SetPrefix("Video")
	v.Logger.SetLevel(l)

	return v
}

// OnLoad opens the video. A video that cannot be opened ends the screen at the first Advance.
func (v *VideoPlayer) OnLoad(_ d2screen.LoadingState) { v.Open() }

// Open loads the Bink file, starts the sound and the decoder, and binds the skip keys.
func (v *VideoPlayer) Open() {
	data, err := v.asset.LoadFile(v.path)
	if err != nil {
		v.Errorf("cannot load %s: %v", v.path, err)
		v.done = true

		return
	}

	dec, err := d2video.CreateBinkDecoder(data)
	if err != nil {
		v.Errorf("cannot decode %s: %v", v.path, err)
		v.done = true

		return
	}

	var sink d2video.AudioSink

	if streamer, ok := v.audio.(d2interface.PCMStreamer); ok && len(dec.AudioTracks) > 0 {
		if ad, aerr := dec.NewAudioDecoder(0); aerr != nil {
			v.Warningf("no audio: %v", aerr)
		} else if st, serr := streamer.NewPCMStream(ad.SampleRate(), ad.Channels()); serr != nil {
			v.Warningf("no audio: %v", serr)
		} else {
			v.stream, sink = st, st
		}
	}

	v.cin, err = d2video.NewCinematic(dec, sink, 44100)
	if err != nil {
		v.Errorf("cannot start %s: %v", v.path, err)
		v.closeStream()

		v.done = true

		return
	}

	v.player = v.cin.Player()
	w, h := v.player.Size()
	v.surface = v.renderer.NewSurface(w, h)
	v.Infof("playing %s (%dx%d, revision %c, %d frames, %d audio track(s), clock: %s)", v.path, w, h, dec.Revision(),
		dec.NumFrames(), len(dec.AudioTracks), map[bool]string{true: "audio", false: "wall"}[v.stream != nil])

	if s, perr := strconv.ParseFloat(os.Getenv("OD2_VIDEO_SECONDS"), 64); perr == nil && s > 0 {
		v.limit = s
	}

	v.runner = d2video.NewRunner(v.cin)

	prio := d2enum.PriorityDefault
	if v.overlay {
		prio = d2enum.PriorityHigh
	}

	if err := v.input.BindHandlerWithPriority(v, prio); err == nil {
		v.bound = true
	}
}

// OnUnload stops the decoder and releases the input handler.
func (v *VideoPlayer) OnUnload() error {
	v.stopRunner()
	v.closeStream()

	if v.bound {
		v.bound = false

		return v.input.UnbindHandler(v)
	}

	return nil
}

func (v *VideoPlayer) stopRunner() {
	if v.runner != nil {
		v.runner.Stop()
	}
}

func (v *VideoPlayer) closeStream() {
	if v.stream != nil {
		v.stream.Close()
		v.stream = nil
	}
}

// logStats writes the line the video scenario of scripts/verify.d checks.
func (v *VideoPlayer) logStats() {
	if v.cin == nil {
		return
	}

	clock := v.runner.Clock()
	slow := v.runner.SlowestStep().Milliseconds()

	st := v.stream
	if st == nil {
		v.Infof("VIDEO end audio=none frames=%d clock=%.2fs decode_step_max=%dms", v.player.Decoder().FrameIndex(), clock, slow)

		return
	}

	v.Infof("VIDEO end audio_decoded=%d written=%d played=%d underruns=%d bad_packets=%d max_update_gap=%.0fms frames=%d clock=%.2fs decode_step_max=%dms",
		v.cin.DecodedFrames, st.Written(), st.Played(), st.Underruns(), v.cin.BadPackets, v.maxDt*1000,
		v.player.Decoder().FrameIndex(), clock, slow)
}

func (v *VideoPlayer) finish() {
	if v.done {
		return
	}

	v.done = true
	v.stopRunner()
	v.logStats()
	v.closeStream()

	if v.limit > 0 && os.Getenv("OD2_AUTOEXIT") != "" {
		os.Exit(0)
	}

	if v.onDone != nil {
		v.onDone()
	}
}

// Active reports that the video is loaded and still playing.
func (v *VideoPlayer) Active() bool { return v.player != nil && !v.done }

// Skip ends the video at once (what Esc, Space, Enter and a click do).
func (v *VideoPlayer) Skip() { v.finish() }

// Advance uploads the newest decoded frame and ends the video when the decoder is done.
func (v *VideoPlayer) Advance(elapsed float64) error {
	if v.player == nil {
		v.finish()
		return nil
	}

	if v.done {
		return nil
	}

	v.wall += elapsed
	v.age += elapsed

	if elapsed > v.maxDt {
		v.maxDt = elapsed
	}

	v.runner.TakeFrame(func(px []byte) { v.surface.ReplacePixels(px) })

	if err := v.runner.Err(); err != nil {
		v.Errorf("playback stopped: %v", err)
		v.finish()

		return nil
	}

	if v.runner.Done() || (v.limit > 0 && v.runner.Clock() >= v.limit) {
		v.finish()
	}

	return nil
}

// Render draws the current frame at its largest size that fits the screen, with black bars around it.
func (v *VideoPlayer) Render(target d2interface.Surface) {
	target.Clear(color.Black)

	if v.player == nil || v.surface == nil || v.done {
		return
	}

	vw, vh := v.player.Size()
	// the logical screen (the window divided by the UI scale), not the size of the render target
	x, y, w, h := d2display.VideoRect(d2display.Get(), vw, vh)

	if w <= 0 || h <= 0 {
		return
	}

	// the target is in column space (the 800x600 interface column centred on the screen): move the video's
	// screen-space rectangle into it
	ox, oy := d2display.ColumnOrigin()
	x, y = x-ox, y-oy

	target.PushTranslation(x, y)
	target.PushScale(float64(w)/float64(vw), float64(h)/float64(vh))
	target.Render(v.surface)
	target.PopN(2)
}

// OnKeyUp skips the video on Esc, Space or Enter.
func (v *VideoPlayer) OnKeyUp(event d2interface.KeyEvent) bool {
	switch event.Key() {
	case d2enum.KeyEscape, d2enum.KeySpace, d2enum.KeyEnter:
		if v.age >= skipGrace || v.limit > 0 {
			v.finish()
		}

		return true
	}

	return v.overlay
}

// OnKeyDown keeps the keys away from the game under an overlay movie.
func (v *VideoPlayer) OnKeyDown(_ d2interface.KeyEvent) bool { return v.overlay }

// OnKeyRepeat keeps held keys away from the game under an overlay movie.
func (v *VideoPlayer) OnKeyRepeat(_ d2interface.KeyEvent) bool { return v.overlay }

// OnMouseButtonDown keeps clicks away from the game under an overlay movie.
func (v *VideoPlayer) OnMouseButtonDown(_ d2interface.MouseEvent) bool { return v.overlay }

// OnMouseButtonRepeat keeps held buttons away from the game under an overlay movie.
func (v *VideoPlayer) OnMouseButtonRepeat(_ d2interface.MouseEvent) bool { return v.overlay }

// OnMouseButtonUp skips the video on a click.
func (v *VideoPlayer) OnMouseButtonUp(_ d2interface.MouseEvent) bool {
	if v.age >= skipGrace {
		v.finish()
	}

	return true
}

// DisplayAnchor makes the video screen use the whole window (a cinematic is not the 800x600 interface): the logical
// screen is the window size, so the picture is letterboxed or pillarboxed against the window.
func (v *VideoPlayer) DisplayAnchor() d2display.Anchor { return d2display.AnchorBottom }
