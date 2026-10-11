package d2display

import (
	"image"
	"image/color"
	"image/draw"
)

// FrameInWindow returns what the player sees of a frame drawn at its own (logical) size when the window is
// w x h pixels: the frame scaled (nearest neighbour, like the renderer's default filter) to the largest rectangle
// of its aspect ratio that fits, centred, on black bars. A frame that already has the window size is returned
// unchanged. Screenshots of the menus use it: the menus are drawn at 800x600 and Ebiten scales them to the window.
func FrameInWindow(frame image.Image, w, h int) image.Image {
	fb := frame.Bounds()
	if w <= 0 || h <= 0 || fb.Empty() || (fb.Dx() == w && fb.Dy() == h) {
		return frame
	}

	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)

	// the largest rectangle with the frame's aspect that fits
	rw, rh := w, w*fb.Dy()/fb.Dx() // narrower than the frame: bars above and below
	if w*fb.Dy() >= h*fb.Dx() {    // wider than the frame: bars left and right
		rw, rh = h*fb.Dx()/fb.Dy(), h
	}

	ox, oy := (w-rw)/2, (h-rh)/2

	for y := 0; y < rh; y++ {
		sy := fb.Min.Y + y*fb.Dy()/rh

		for x := 0; x < rw; x++ {
			out.Set(ox+x, oy+y, frame.At(fb.Min.X+x*fb.Dx()/rw, sy))
		}
	}

	return out
}
