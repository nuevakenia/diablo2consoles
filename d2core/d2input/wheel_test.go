package d2input

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
)

func TestWheelKeys(t *testing.T) {
	for _, c := range []struct {
		dy float64
		k  d2enum.Key
		n  int
	}{
		{0, 0, 0}, {1, d2enum.KeyMouseWheelUp, 1}, {-1, d2enum.KeyMouseWheelDown, 1}, {0.2, d2enum.KeyMouseWheelUp, 1},
		{-2.5, d2enum.KeyMouseWheelDown, 3}, {100, d2enum.KeyMouseWheelUp, maxWheelNotches},
	} {
		if k, n := WheelKeys(c.dy); n != c.n || (n > 0 && k != c.k) {
			t.Errorf("WheelKeys(%v) = %v,%d want %v,%d", c.dy, k, n, c.k, c.n)
		}
	}
}

type fakeWheel struct{ dy float64 }

func (f *fakeWheel) CursorPosition() (int, int)                        { return 0, 0 }
func (f *fakeWheel) InputChars() []rune                                { return nil }
func (f *fakeWheel) IsKeyPressed(d2enum.Key) bool                      { return false }
func (f *fakeWheel) IsKeyJustPressed(d2enum.Key) bool                  { return false }
func (f *fakeWheel) IsKeyJustReleased(d2enum.Key) bool                 { return false }
func (f *fakeWheel) IsMouseButtonPressed(d2enum.MouseButton) bool      { return false }
func (f *fakeWheel) IsMouseButtonJustPressed(d2enum.MouseButton) bool  { return false }
func (f *fakeWheel) IsMouseButtonJustReleased(d2enum.MouseButton) bool { return false }
func (f *fakeWheel) KeyPressDuration(d2enum.Key) int                   { return 0 }
func (f *fakeWheel) MouseWheel() (float64, float64)                    { return 0, f.dy }

type keyLog struct{ keys []string }

func (k *keyLog) OnKeyDown(e d2interface.KeyEvent) bool {
	k.keys = append(k.keys, "down"+string(rune('0'+int(e.Key()-d2enum.KeyMouseWheelUp))))
	return false
}

func (k *keyLog) OnKeyUp(e d2interface.KeyEvent) bool {
	k.keys = append(k.keys, "up"+string(rune('0'+int(e.Key()-d2enum.KeyMouseWheelUp))))
	return false
}

// The polled wheel reaches the key handlers as WheelUp / WheelDown presses, one pair per notch, and the injected one
// does the same.
func TestPolledAndInjectedWheelReachHandlers(t *testing.T) {
	svc := &fakeWheel{dy: -2}
	im := &inputManager{inputService: svc}
	l := &keyLog{}

	if err := im.BindHandler(l); err != nil {
		t.Fatal(err)
	}

	if err := im.Advance(0.01, 0); err != nil {
		t.Fatal(err)
	}

	if len(l.keys) != 4 || l.keys[0] != "down1" || l.keys[3] != "up1" { // WheelDown is WheelUp+1
		t.Fatalf("polled wheel -2: %v", l.keys)
	}

	l.keys = nil
	svc.dy = 0
	im.InjectWheel(true, 1, 0)

	if len(l.keys) != 2 || l.keys[0] != "down0" || l.keys[1] != "up0" {
		t.Fatalf("injected wheel up: %v", l.keys)
	}
}
