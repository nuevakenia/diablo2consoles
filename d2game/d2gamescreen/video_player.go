package d2gamescreen

import (
	"image/color"
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2data/d2video"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2screen"
)

// PlayVideosEnabled reports whether OD2_PLAY_VIDEOS=1: cinematics are played from the user's Bink files.
// It is off by default; the decoder has no audio yet, so videos play silent.
func PlayVideosEnabled() bool { return os.Getenv("OD2_PLAY_VIDEOS") == "1" }

// VideoPlayer is a game screen that plays one Bink cinematic letterboxed on a black screen. Esc, Space, Enter or
// any mouse click skips it; when it ends (or fails to load) onDone is called once to go to the next screen.
type VideoPlayer struct {
	asset    *d2asset.AssetManager
	renderer d2interface.Renderer
	input    d2interface.InputManager
	path     string
	onDone   func()

	player  *d2video.Player
	surface d2interface.Surface
	done    bool
	bound   bool

	*d2util.Logger
}

// CreateVideoPlayer creates a screen that plays the Bink file at the asset path, then calls onDone.
func CreateVideoPlayer(asset *d2asset.AssetManager, renderer d2interface.Renderer, input d2interface.InputManager,
	path string, l d2util.LogLevel, onDone func()) *VideoPlayer {
	v := &VideoPlayer{asset: asset, renderer: renderer, input: input, path: path, onDone: onDone}
	v.Logger = d2util.NewLogger()
	v.Logger.SetPrefix("Video")
	v.Logger.SetLevel(l)

	return v
}

// OnLoad opens the video. A video that cannot be opened ends the screen at the first Advance.
func (v *VideoPlayer) OnLoad(_ d2screen.LoadingState) {
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

	v.player = d2video.NewPlayer(dec)
	w, h := v.player.Size()
	v.surface = v.renderer.NewSurface(w, h)
	v.Infof("playing %s (%dx%d, revision %c, %d frames)", v.path, w, h, dec.Revision(), dec.NumFrames())

	if err := v.input.BindHandler(v); err == nil {
		v.bound = true
	}
}

// OnUnload releases the input handler.
func (v *VideoPlayer) OnUnload() error {
	if v.bound {
		v.bound = false

		return v.input.UnbindHandler(v)
	}

	return nil
}

func (v *VideoPlayer) finish() {
	if v.done {
		return
	}

	v.done = true

	if v.onDone != nil {
		v.onDone()
	}
}

// Advance decodes the frames that are due.
func (v *VideoPlayer) Advance(elapsed float64) error {
	if v.player == nil {
		v.finish()
		return nil
	}

	if v.done {
		return nil
	}

	changed, err := v.player.Advance(elapsed)
	if err != nil {
		v.Errorf("playback stopped: %v", err)
		v.finish()

		return nil
	}

	if changed {
		v.surface.ReplacePixels(v.player.Frame())
	}

	if v.player.Done() {
		v.finish()
	}

	return nil
}

// Render draws the current frame at its largest size that fits the screen, with black bars around it.
func (v *VideoPlayer) Render(target d2interface.Surface) {
	target.Clear(color.Black)

	if v.player == nil || v.player.Frame() == nil {
		return
	}

	vw, vh := v.player.Size()
	sw, sh := target.GetSize()
	x, y, w, h := d2display.VideoRect(d2display.Size{W: sw, H: sh}, vw, vh)

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
		v.finish()
		return true
	}

	return false
}

// OnMouseButtonUp skips the video on a click.
func (v *VideoPlayer) OnMouseButtonUp(_ d2interface.MouseEvent) bool {
	v.finish()

	return true
}
