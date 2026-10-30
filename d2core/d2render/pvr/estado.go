//go:build dreamcast

package pvr

import (
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// estado es el estado de dibujado de una Surface. Su forma y sus valores por omision son los de
// d2core/d2render/ebiten/surface_state.go, para que el motor se comporte igual en las dos plataformas.
type estado struct {
	x, y             int
	filtro           d2enum.Filter
	col              color.Color
	brillo           float64
	saturacion       float64
	efecto           d2enum.DrawEffect
	sesgoX, sesgoY   float64
	escalaX, escalaY float64
}

// los valores por omision de Ebitengine: saturacion y brillo 1, escala 1, sesgo 0 (ver
// d2core/d2render/ebiten/ebiten_surface.go:48).
const (
	brilloPorOmision     = 1.0
	saturacionPorOmision = 1.0
)

func estadoInicial() estado {
	return estado{
		efecto:     d2enum.DrawEffectNone,
		saturacion: saturacionPorOmision,
		brillo:     brilloPorOmision,
		escalaX:    1.0,
		escalaY:    1.0,
		filtro:     d2enum.FilterNearest,
	}
}
