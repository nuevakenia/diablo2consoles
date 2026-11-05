package ebiten

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

type surfaceState struct {
	x              int
	y              int
	filter         ebiten.Filter
	color          color.Color
	brightness     float64
	saturation     float64
	effect         d2enum.DrawEffect
	skewX, skewY   float64
	scaleX, scaleY float64

	// canales: DC-002.159o -- los cuatro canales del color, resueltos en `PushColor` (con la interfaz recien
	// armada) para que el dibujo no tenga que despachar `color.Color.RGBA()`. `color` queda para el camino viejo
	// (la palanca apagada) y para la sonda, que compara las dos palabras de la interfaz.
	canales canalesDelColor159o
}
