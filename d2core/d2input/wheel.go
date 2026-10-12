package d2input

import (
	"math"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

// The original game delivers the mouse wheel as the pseudo keys 0x103 / 0x104 of its key bindings (default.key binds
// them to "select previous / next skill"). The port does the same: a wheel notch is a key press and release of
// d2enum.KeyMouseWheelUp / KeyMouseWheelDown at the cursor, sent through the same handler chain as every key, so the
// binding table (key_map.go) decides what it does.

// maxWheelNotches caps the events of one frame (a fast trackpad flick reports a large delta).
const maxWheelNotches = 8

// wheelService is implemented by an input service that reports the wheel.
type wheelService interface {
	MouseWheel() (dx, dy float64)
}

// WheelKeys turns the vertical wheel delta of one frame into the pseudo key and the number of presses. Positive dy is
// wheel up (away from the user). A delta of zero (or NaN) gives n = 0; fractional trackpad deltas count as one notch.
func WheelKeys(dy float64) (k d2enum.Key, n int) {
	if dy == 0 || math.IsNaN(dy) {
		return 0, 0
	}

	k = d2enum.KeyMouseWheelUp
	if dy < 0 {
		k, dy = d2enum.KeyMouseWheelDown, -dy
	}

	n = int(math.Ceil(math.Min(dy, maxWheelNotches)))
	if n < 1 {
		n = 1
	}

	return k, n
}

func (im *inputManager) updateWheel(e HandlerEvent) {
	ws, ok := im.inputService.(wheelService)
	if !ok {
		return
	}

	_, dy := ws.MouseWheel()
	k, n := WheelKeys(dy)

	for i := 0; i < n; i++ {
		im.sendWheel(k, e)
	}
}

// sendWheel sends one notch as a key press followed by its release.
func (im *inputManager) sendWheel(k d2enum.Key, e HandlerEvent) {
	event := KeyEvent{HandlerEvent: e, key: k, duration: 1}

	im.propagate(func(h d2interface.InputEventHandler) bool {
		if l, ok := h.(d2interface.KeyDownHandler); ok {
			return l.OnKeyDown(&event)
		}

		return false
	})
	im.propagate(func(h d2interface.InputEventHandler) bool {
		if l, ok := h.(d2interface.KeyUpHandler); ok {
			return l.OnKeyUp(&event)
		}

		return false
	})
}

// InjectWheel sends wheel notches (up = away from the user) like the polled wheel does.
func (im *inputManager) InjectWheel(up bool, notches int, mod d2enum.KeyMod) {
	k := d2enum.KeyMouseWheelDown
	if up {
		k = d2enum.KeyMouseWheelUp
	}

	for i := 0; i < notches; i++ {
		im.sendWheel(k, im.syntheticBase(mod, im.cursorX, im.cursorY))
	}
}
