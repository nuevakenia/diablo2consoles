// Package ebiten provides graphics and input API to develop a 2D game.
package ebiten

import (
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

var (
	//nolint:gochecknoglobals // This is a constant in all but by name, no constant map in go
	keyToEbiten = map[d2enum.Key]ebiten.Key{
		d2enum.Key0:            ebiten.Key0,
		d2enum.Key1:            ebiten.Key1,
		d2enum.Key2:            ebiten.Key2,
		d2enum.Key3:            ebiten.Key3,
		d2enum.Key4:            ebiten.Key4,
		d2enum.Key5:            ebiten.Key5,
		d2enum.Key6:            ebiten.Key6,
		d2enum.Key7:            ebiten.Key7,
		d2enum.Key8:            ebiten.Key8,
		d2enum.Key9:            ebiten.Key9,
		d2enum.KeyA:            ebiten.KeyA,
		d2enum.KeyB:            ebiten.KeyB,
		d2enum.KeyC:            ebiten.KeyC,
		d2enum.KeyD:            ebiten.KeyD,
		d2enum.KeyE:            ebiten.KeyE,
		d2enum.KeyF:            ebiten.KeyF,
		d2enum.KeyG:            ebiten.KeyG,
		d2enum.KeyH:            ebiten.KeyH,
		d2enum.KeyI:            ebiten.KeyI,
		d2enum.KeyJ:            ebiten.KeyJ,
		d2enum.KeyK:            ebiten.KeyK,
		d2enum.KeyL:            ebiten.KeyL,
		d2enum.KeyM:            ebiten.KeyM,
		d2enum.KeyN:            ebiten.KeyN,
		d2enum.KeyO:            ebiten.KeyO,
		d2enum.KeyP:            ebiten.KeyP,
		d2enum.KeyQ:            ebiten.KeyQ,
		d2enum.KeyR:            ebiten.KeyR,
		d2enum.KeyS:            ebiten.KeyS,
		d2enum.KeyT:            ebiten.KeyT,
		d2enum.KeyU:            ebiten.KeyU,
		d2enum.KeyV:            ebiten.KeyV,
		d2enum.KeyW:            ebiten.KeyW,
		d2enum.KeyX:            ebiten.KeyX,
		d2enum.KeyY:            ebiten.KeyY,
		d2enum.KeyZ:            ebiten.KeyZ,
		d2enum.KeyApostrophe:   ebiten.KeyApostrophe,
		d2enum.KeyBackslash:    ebiten.KeyBackslash,
		d2enum.KeyBackspace:    ebiten.KeyBackspace,
		d2enum.KeyCapsLock:     ebiten.KeyCapsLock,
		d2enum.KeyComma:        ebiten.KeyComma,
		d2enum.KeyDelete:       ebiten.KeyDelete,
		d2enum.KeyDown:         ebiten.KeyDown,
		d2enum.KeyEnd:          ebiten.KeyEnd,
		d2enum.KeyEnter:        ebiten.KeyEnter,
		d2enum.KeyEqual:        ebiten.KeyEqual,
		d2enum.KeyEscape:       ebiten.KeyEscape,
		d2enum.KeyF1:           ebiten.KeyF1,
		d2enum.KeyF2:           ebiten.KeyF2,
		d2enum.KeyF3:           ebiten.KeyF3,
		d2enum.KeyF4:           ebiten.KeyF4,
		d2enum.KeyF5:           ebiten.KeyF5,
		d2enum.KeyF6:           ebiten.KeyF6,
		d2enum.KeyF7:           ebiten.KeyF7,
		d2enum.KeyF8:           ebiten.KeyF8,
		d2enum.KeyF9:           ebiten.KeyF9,
		d2enum.KeyF10:          ebiten.KeyF10,
		d2enum.KeyF11:          ebiten.KeyF11,
		d2enum.KeyF12:          ebiten.KeyF12,
		d2enum.KeyGraveAccent:  ebiten.KeyGraveAccent,
		d2enum.KeyHome:         ebiten.KeyHome,
		d2enum.KeyInsert:       ebiten.KeyInsert,
		d2enum.KeyKP0:          ebiten.KeyKP0,
		d2enum.KeyKP1:          ebiten.KeyKP1,
		d2enum.KeyKP2:          ebiten.KeyKP2,
		d2enum.KeyKP3:          ebiten.KeyKP3,
		d2enum.KeyKP4:          ebiten.KeyKP4,
		d2enum.KeyKP5:          ebiten.KeyKP5,
		d2enum.KeyKP6:          ebiten.KeyKP6,
		d2enum.KeyKP7:          ebiten.KeyKP7,
		d2enum.KeyKP8:          ebiten.KeyKP8,
		d2enum.KeyKP9:          ebiten.KeyKP9,
		d2enum.KeyKPAdd:        ebiten.KeyKPAdd,
		d2enum.KeyKPDecimal:    ebiten.KeyKPDecimal,
		d2enum.KeyKPDivide:     ebiten.KeyKPDivide,
		d2enum.KeyKPEnter:      ebiten.KeyKPEnter,
		d2enum.KeyKPEqual:      ebiten.KeyKPEqual,
		d2enum.KeyKPMultiply:   ebiten.KeyKPMultiply,
		d2enum.KeyKPSubtract:   ebiten.KeyKPSubtract,
		d2enum.KeyLeft:         ebiten.KeyLeft,
		d2enum.KeyLeftBracket:  ebiten.KeyLeftBracket,
		d2enum.KeyMenu:         ebiten.KeyMenu,
		d2enum.KeyMinus:        ebiten.KeyMinus,
		d2enum.KeyNumLock:      ebiten.KeyNumLock,
		d2enum.KeyPageDown:     ebiten.KeyPageDown,
		d2enum.KeyPageUp:       ebiten.KeyPageUp,
		d2enum.KeyPause:        ebiten.KeyPause,
		d2enum.KeyPeriod:       ebiten.KeyPeriod,
		d2enum.KeyPrintScreen:  ebiten.KeyPrintScreen,
		d2enum.KeyRight:        ebiten.KeyRight,
		d2enum.KeyRightBracket: ebiten.KeyRightBracket,
		d2enum.KeyScrollLock:   ebiten.KeyScrollLock,
		d2enum.KeySemicolon:    ebiten.KeySemicolon,
		d2enum.KeySlash:        ebiten.KeySlash,
		d2enum.KeySpace:        ebiten.KeySpace,
		d2enum.KeyTab:          ebiten.KeyTab,
		d2enum.KeyUp:           ebiten.KeyUp,
		d2enum.KeyAlt:          ebiten.KeyAlt,
		d2enum.KeyControl:      ebiten.KeyControl,
		d2enum.KeyShift:        ebiten.KeyShift,
	}
	//nolint:gochecknoglobals // This is a constant in all but by name, no constant map in go
	mouseButtonToEbiten = map[d2enum.MouseButton]ebiten.MouseButton{
		d2enum.MouseButtonLeft:   ebiten.MouseButtonLeft,
		d2enum.MouseButtonMiddle: ebiten.MouseButtonMiddle,
		d2enum.MouseButtonRight:  ebiten.MouseButtonRight,
	}
)

// InputService provides an abstraction on ebiten to support handling input events
type InputService struct{}

// CursorPosition returns a position of a mouse cursor relative to the game screen (window).
func (is InputService) CursorPosition() (x, y int) {
	if d2util.QuietEnabled() {
		return 0, 0 // OD2_QUIET: the real mouse is ignored; scripts inject their own
	}

	return ebiten.CursorPosition()
}

// InputChars return "printable" runes read from the keyboard at the time update is called.
func (is InputService) InputChars() []rune {
	if d2util.QuietEnabled() {
		return nil
	}

	return ebiten.InputChars()
}

// IsKeyPressed checks if the provided key is down.
//
// ebiten.IsKeyPressed is a round trip to the window thread (about 8us when that thread is busy
// drawing) and the input manager asks for every key on every frame. inpututil samples the very
// same ebiten.IsKeyPressed once per tick, just before Update, so a press duration > 0 is the same
// answer for the whole tick without the round trip.
func (is InputService) IsKeyPressed(key d2enum.Key) bool {
	if d2util.QuietEnabled() {
		return false
	}

	if key == d2enum.KeyMeta {
		return commandHeld()
	}

	ek, ok := keyToEbiten[key]
	if !ok {
		return false // no ebiten key (mouse 3-5, wheel, tilde): never alias key 0
	}

	return inpututil.KeyPressDuration(ek) > 0
}

// IsKeyJustPressed checks if the provided key is just transitioned from up to down.
func (is InputService) IsKeyJustPressed(key d2enum.Key) bool {
	if d2util.QuietEnabled() {
		return false
	}

	if key == d2enum.KeyMeta {
		return false // no ebiten key: only the held state is known
	}

	ek, ok := keyToEbiten[key]
	if !ok {
		return false // no ebiten key (mouse 3-5, wheel, tilde): never alias key 0
	}

	return inpututil.IsKeyJustPressed(ek)
}

// IsKeyJustReleased checks if the provided key is just transitioned from down to up.
func (is InputService) IsKeyJustReleased(key d2enum.Key) bool {
	if d2util.QuietEnabled() {
		return false
	}

	if key == d2enum.KeyMeta {
		return false
	}

	ek, ok := keyToEbiten[key]
	if !ok {
		return false // no ebiten key (mouse 3-5, wheel, tilde): never alias key 0
	}

	return inpututil.IsKeyJustReleased(ek)
}

// ignoreRealMouseButtons: a scripted run (OD2_AUTOSCRIPT) takes its clicks from the script only. A physical click on
// the window of a verification game (the owner's mouse, another tool clicking on a window that lies under the pointer)
// walked the hero of 8g-portal-roundtrip to the edge of the Blood Moor before the script's first step. OD2_REAL_INPUT=1
// lets the real buttons through again (a scripted run that is driven by hand).
var ignoreRealMouseButtons = os.Getenv("OD2_AUTOSCRIPT") != "" && os.Getenv("OD2_REAL_INPUT") == ""

// IsMouseButtonPressed checks if the provided mouse button is down.
// Like IsKeyPressed it reads the once-per-tick sample of inpututil instead of asking the window thread.
func (is InputService) IsMouseButtonPressed(button d2enum.MouseButton) bool {
	if ignoreRealMouseButtons || d2util.QuietEnabled() {
		return false
	}

	return inpututil.MouseButtonPressDuration(mouseButtonToEbiten[button]) > 0
}

// IsMouseButtonJustPressed checks if the provided mouse button is just transitioned from up to down.
func (is InputService) IsMouseButtonJustPressed(button d2enum.MouseButton) bool {
	if ignoreRealMouseButtons || d2util.QuietEnabled() {
		return false
	}

	return inpututil.IsMouseButtonJustPressed(mouseButtonToEbiten[button])
}

// IsMouseButtonJustReleased checks if the provided mouse button is just transitioned from down to up.
func (is InputService) IsMouseButtonJustReleased(button d2enum.MouseButton) bool {
	if ignoreRealMouseButtons || d2util.QuietEnabled() {
		return false
	}

	return inpututil.IsMouseButtonJustReleased(mouseButtonToEbiten[button])
}

// KeyPressDuration returns how long the key is pressed in frames.
func (is InputService) KeyPressDuration(key d2enum.Key) int {
	if d2util.QuietEnabled() {
		return 0
	}

	if key == d2enum.KeyMeta {
		if commandHeld() {
			return 1
		}

		return 0
	}

	ek, ok := keyToEbiten[key]
	if !ok {
		return 0 // no ebiten key (mouse 3-5, wheel, tilde): never alias key 0
	}

	return inpututil.KeyPressDuration(ek)
}

// MouseWheel returns the wheel movement of the last tick (positive y = wheel up / away from the user). A scripted
// run ignores the real wheel like the real buttons (OD2_REAL_INPUT=1 lets it through).
func (is InputService) MouseWheel() (dx, dy float64) {
	if ignoreRealMouseButtons || d2util.QuietEnabled() {
		return 0, 0
	}

	return ebiten.Wheel()
}
