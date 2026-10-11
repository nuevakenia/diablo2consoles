package d2display

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestMenuRegion(t *testing.T) {
	tests := []struct {
		w, h int
		want image.Rectangle
	}{
		{800, 600, image.Rect(0, 0, 800, 600)},
		{1920, 1080, image.Rect(240, 0, 1680, 1080)},
		{1512, 982, image.Rect(101, 0, 1410, 982)},
		{1200, 1200, image.Rect(0, 150, 1200, 1050)},
	}

	for _, tt := range tests {
		if got := MenuRegion(tt.w, tt.h); got != tt.want {
			t.Errorf("MenuRegion(%d,%d) = %v, want %v", tt.w, tt.h, got, tt.want)
		}
	}
}

func picture(w, h int, r image.Rectangle) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, r, &image.Uniform{C: color.RGBA{R: 90, G: 60, B: 30, A: 255}}, image.Point{}, draw.Src)

	return img
}

func TestNonBlackBounds(t *testing.T) {
	r := image.Rect(10, 20, 50, 70)
	if got := NonBlackBounds(picture(100, 100, r), 8); got != r {
		t.Errorf("bounds %v, want %v", got, r)
	}

	if got := NonBlackBounds(picture(10, 10, image.Rectangle{}), 8); !got.Empty() {
		t.Errorf("black picture: %v", got)
	}

	// noise below the threshold is black
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 5, G: 5, B: 5, A: 255})

	if got := NonBlackBounds(img, 8); !got.Empty() {
		t.Errorf("dark noise counted: %v", got)
	}
}

func TestCheckMenuCentredTop(t *testing.T) {
	const w, h = 1512, 982

	r := MenuRegion(w, h) // (101,0)-(1410,982)

	tests := []struct {
		name    string
		content image.Rectangle
		wantErr bool
	}{
		{"dark bottom 45px", image.Rect(r.Min.X, 0, r.Max.X, 937), false},
		{"full", r, false},
		{"unscaled column", image.Rect(356, 191, 1156, 763), true},
		{"shifted down", image.Rect(r.Min.X, 60, r.Max.X, 982), true},
		{"shifted right", image.Rect(r.Min.X+60, 0, r.Max.X+60, 937), true},
		{"too short", image.Rect(r.Min.X, 0, r.Max.X, 600), true},
	}

	for _, tt := range tests {
		if err := CheckMenuCentredTop(tt.content, w, h, 12, 0.8); (err != nil) != tt.wantErr {
			t.Errorf("%s: err = %v, want error %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestFrameInWindow(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 800, 600))
	draw.Draw(frame, frame.Bounds(), &image.Uniform{C: color.RGBA{R: 200, G: 100, B: 50, A: 255}}, image.Point{}, draw.Src)

	if FrameInWindow(frame, 800, 600) != image.Image(frame) {
		t.Error("a frame of the window size must be returned unchanged")
	}

	for _, sz := range [][2]int{{1512, 982}, {1920, 1080}, {1200, 1200}, {3440, 1440}} {
		out := FrameInWindow(frame, sz[0], sz[1])
		if b := out.Bounds(); b.Dx() != sz[0] || b.Dy() != sz[1] {
			t.Fatalf("%v: size %v", sz, b)
		}

		got := NonBlackBounds(out, 8)
		if want := MenuRegion(sz[0], sz[1]); got != want {
			t.Errorf("%v: content %v, want the centred region %v", sz, got, want)
		}

		if err := CheckMenuCentred(got, sz[0], sz[1], 2, 0.99); err != nil {
			t.Errorf("%v: %v", sz, err)
		}
	}
}

func TestCheckMenuCentred(t *testing.T) {
	const w, h = 1920, 1080

	region := MenuRegion(w, h)

	tests := []struct {
		name    string
		content image.Rectangle
		wantErr bool
	}{
		{"fills the region", region, false},
		{"slightly short", region.Inset(10), false},
		{"800x600 in the corner (the bug)", image.Rect(0, 0, 800, 600), true},
		{"unscaled 800x600 column in the middle", image.Rect(560, 240, 1360, 840), true},
		{"full window, no bars", image.Rect(0, 0, w, h), true},
		{"shifted right", region.Add(image.Pt(60, 0)), true},
		{"black", image.Rectangle{}, true},
	}

	for _, tt := range tests {
		err := CheckMenuCentred(tt.content, w, h, 12, 0.8)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err = %v, want error %v", tt.name, err, tt.wantErr)
		}
	}
}
