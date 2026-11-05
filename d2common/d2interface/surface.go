package d2interface

import (
	"image"
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// Surface represents a renderable surface.
type Surface interface {
	Renderer() Renderer
	Clear(color color.Color)
	DrawRect(width, height int, color color.Color)
	DrawLine(x, y int, color color.Color)
	DrawTextf(format string, params ...interface{})
	GetSize() (width, height int)
	GetDepth() int
	Pop()
	PopN(n int)
	PushColor(color color.Color)
	PushEffect(effect d2enum.DrawEffect)
	PushFilter(filter d2enum.Filter)
	PushTranslation(x, y int)
	PushSkew(x, y float64)
	PushScale(x, y float64)
	PushBrightness(brightness float64)
	PushSaturation(saturation float64)
	Render(surface Surface)
	// Renders a section of the surface enclosed by bounds
	RenderSection(surface Surface, bound image.Rectangle)
	// RenderRecorte draws ONLY the region [0,width) x [0,height) of `surface`, in the position and with the
	// state's transformation. Unlike RenderSection it does not build a temporary buffer: reads just that
	// window of the origin (DC-002.159g -- the animation's reused surface).
	RenderRecorte(surface Surface, width, height int)
	ReplacePixels(pixels []byte)
	Screenshot() *image.RGBA
}
