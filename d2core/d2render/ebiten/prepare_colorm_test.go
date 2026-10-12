package ebiten

import (
	"image/color"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// prepareColorM (memoised) must give the same matrix and composite mode as applyHSV + handleStateEffect.
func TestPrepareColorMMatchesDirectPath(t *testing.T) {
	colours := []color.Color{
		nil,
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
		color.RGBA{R: 200, G: 180, B: 120, A: 255},
		color.RGBA{R: 40, G: 50, B: 90, A: 128},
		color.RGBA{A: 0},
	}
	effects := []d2enum.DrawEffect{
		d2enum.DrawEffectPctTransparency25, d2enum.DrawEffectPctTransparency50, d2enum.DrawEffectPctTransparency75,
		d2enum.DrawEffectModulate, d2enum.DrawEffectBurn, d2enum.DrawEffectNormal, d2enum.DrawEffectMod2XTrans,
		d2enum.DrawEffectMod2X, d2enum.DrawEffectNone,
	}
	hsvs := [][2]float64{{1, 1}, {1, 0}, {0.5, 1.5}}

	for _, c := range colours {
		for _, e := range effects {
			for _, h := range hsvs {
				for _, hsv := range []bool{false, true} {
					s := createEbitenSurface(nil, nil)
					s.stateCurrent.color = c
					s.stateCurrent.effect = e
					s.stateCurrent.saturation, s.stateCurrent.brightness = h[0], h[1]

					ref := s.createDrawImageOptions()
					if hsv {
						s.applyHSV(ref)
					}

					s.handleStateEffect(ref)

					for pass := 0; pass < 3; pass++ {
						opts := s.scratchDrawImageOptions()
						s.prepareColorM(opts, hsv)

						if opts.CompositeMode != ref.CompositeMode {
							t.Fatalf("colour %v effect %v: composite %v want %v", c, e, opts.CompositeMode, ref.CompositeMode)
						}

						for i := 0; i < 4; i++ {
							for j := 0; j < 5; j++ {
								if got, want := opts.ColorM.Element(i, j), ref.ColorM.Element(i, j); got != want {
									t.Fatalf("colour %v effect %v hsv %v/%v pass %d (%d,%d): got %v want %v",
										c, e, hsv, h, pass, i, j, got, want)
								}
							}
						}
					}
				}
			}
		}
	}
}
