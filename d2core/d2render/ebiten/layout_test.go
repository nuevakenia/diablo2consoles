package ebiten

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
)

// Menus use the 800x600 logical size (Ebiten scales it up to the window); the game uses the window size.
func TestLayoutMenuFitsAndGameFillsWindow(t *testing.T) {
	r := &Renderer{}

	defer d2display.SetAnchor(d2display.AnchorBottom)

	d2display.SetAnchor(d2display.AnchorCenter)

	if w, h := r.Layout(1512, 982); w != d2display.BaseW || h != d2display.BaseH {
		t.Fatalf("menu layout = %dx%d, want %dx%d", w, h, d2display.BaseW, d2display.BaseH)
	}

	d2display.SetAnchor(d2display.AnchorBottom)

	if w, h := r.Layout(1512, 982); w != 1512 || h != 982 {
		t.Fatalf("game layout = %dx%d, want 1512x982", w, h)
	}
}

// With OD2_DISPLAY pinned, menus still use 800x600 (scaled to the window) and the game the pinned size.
func TestLayoutPinnedMenuAndGame(t *testing.T) {
	r := &Renderer{}

	d2display.Set(d2display.Size{W: 1920, H: 1080})
	d2display.SetFixed(true)

	defer func() {
		d2display.SetFixed(false)
		d2display.SetAnchor(d2display.AnchorBottom)
	}()

	d2display.SetAnchor(d2display.AnchorCenter)

	if w, h := r.Layout(1000, 700); w != d2display.BaseW || h != d2display.BaseH {
		t.Fatalf("pinned menu layout = %dx%d, want %dx%d", w, h, d2display.BaseW, d2display.BaseH)
	}

	d2display.SetAnchor(d2display.AnchorBottom)

	if w, h := r.Layout(1000, 700); w != 1920 || h != 1080 {
		t.Fatalf("pinned game layout = %dx%d, want 1920x1080", w, h)
	}

	if win := d2display.WindowSize(); win.W != 1920 || win.H != 1080 {
		t.Fatalf("pinned window = %v", win)
	}
}
