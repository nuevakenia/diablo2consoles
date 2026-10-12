package ebiten

import (
	"image/color"
	"testing"
)

// The memoised tinted HSV matrix must equal ChangeHSV applied to the colour matrix, on the first call
// (miss) and on the following ones (hit).
func TestApplyHSVWithColourMatchesChangeHSV(t *testing.T) {
	colours := []color.Color{
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
		color.RGBA{R: 200, G: 180, B: 120, A: 255},
		color.RGBA{R: 40, G: 50, B: 90, A: 128},
		color.RGBA{A: 0},
		color.NRGBA{R: 10, G: 200, B: 30, A: 77},
	}
	hsvs := [][2]float64{{1, 0}, {0.5, 1.5}, {0, 1}, {1, 1.2}}

	for _, c := range colours {
		for _, h := range hsvs {
			s := createEbitenSurface(nil, nil)
			s.stateCurrent.color = c
			s.stateCurrent.saturation, s.stateCurrent.brightness = h[0], h[1]

			ref := s.createDrawImageOptions()
			ref.ColorM.ChangeHSV(0, h[0], h[1])

			for pass := 0; pass < 3; pass++ {
				opts := s.createDrawImageOptions()
				s.applyHSV(opts)

				for i := 0; i < 4; i++ {
					for j := 0; j < 5; j++ {
						if got, want := opts.ColorM.Element(i, j), ref.ColorM.Element(i, j); got != want {
							t.Fatalf("colour %v hsv %v pass %d element (%d,%d): got %v want %v", c, h, pass, i, j, got, want)
						}
					}
				}
			}
		}
	}
}
