// Package ebiten: la version de CONSOLA del paquete que el motor usa para dibujar, teclado y loop
// principal. NO es Ebitengine: es un esqueleto del mismo ABI con la misma forma.
//
// Por que existe: el motor esta escrito contra Ebitengine (PC), pero los lugares donde lo toca estan
// AISLADOS (d2core/d2render/ebiten, d2core/d2input/ebiten, d2core/d2audio/ebiten). Con este paquete el
// motor COMPILA para SH-4 y se mide cuanto falta de verdad; despues esta misma capa se llena con el
// backend de la Dreamcast (PVR para dibujar, el joystick para los controles, KOS para el audio).
//
// Que hace hoy y que no:
//   - Image, DrawImage, ColorM, los colores y el loop (RunGame) existen para que compile y para que el
//     motor arranque su juego de datos.
//   - Dibujar de verdad: NO. Se guarda el estado (imagenes, opciones) para que la capa de arriba no
//     cambie, y el pintado real (PVR) entra cuando esta capa se conecte al hardware. Ya esta probado
//     aparte que el PVR de la consola pinta (POC motor-menu: 300 cuadros, 0 errores del PVR).
//   - El joystick: las teclas existen como constantes y IsKeyPressed devuelve el estado de un mapa que
//     hoy nadie llena. Conectarlo al control de la consola es el paso que "mueve al personaje".
package ebiten

import (
	"image"
	"image/color"
)

// ================================================================ imagenes

// Image es la imagen de Ebitengine. MEDIDO en la de verdad (v2.0.2, con la que el motor declara en
// go.mod): es un STRUCT, no una interfaz, y su SubImage devuelve image.Image (la interfaz de Go, no
// *Image). Las dos cosas importan, porque el motor hace SubImage(rect).(*ebiten.Image): esa asercion
// solo es legal porque SubImage devuelve una INTERFAZ.
type Image struct {
	pixeles []uint8
	ancho   int
	alto    int
}

// NewImage devuelve una imagen transparente del tamano pedido (8 bits por pixel por canal, igual que
// Ebitengine).
func NewImage(w, h int) *Image {
	return &Image{pixeles: make([]uint8, w*h*4), ancho: w, alto: h}
}

// NewImageFromImage copia una image.Image de Go.
func NewImageFromImage(src image.Image) *Image {
	b := src.Bounds()
	img := NewImage(b.Dx(), b.Dy())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := (y*img.ancho + x) * 4
			img.pixeles[i+0] = uint8(r >> 8)
			img.pixeles[i+1] = uint8(g >> 8)
			img.pixeles[i+2] = uint8(bl >> 8)
			img.pixeles[i+3] = uint8(a >> 8)
		}
	}
	return img
}

// ColorModel: parte de la interfaz image.Image (la pide SubImage al devolver image.Image).
func (i *Image) ColorModel() color.Model { return color.RGBAModel }

// Bounds: el rectangulo de la imagen.
func (i *Image) Bounds() image.Rectangle {
	return image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: i.ancho, Y: i.alto}}
}

// Size: el tamano.
func (i *Image) Size() (int, int) { return i.ancho, i.alto }

// Clear: borra la imagen con el color dado.
func (i *Image) Clear() {
	for j := range i.pixeles {
		i.pixeles[j] = 0
	}
}

// SetRGBA: pinta un pixel con los valores crudos de 8 bits (es lo que usa SubImage al recortar).
func (i *Image) SetRGBA(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= i.ancho || y >= i.alto {
		return
	}
	j := (y*i.ancho + x) * 4
	i.pixeles[j+0] = c.R
	i.pixeles[j+1] = c.G
	i.pixeles[j+2] = c.B
	i.pixeles[j+3] = c.A
}

// Set: pinta un pixel.
func (i *Image) Set(x, y int, c Color) {
	if x < 0 || y < 0 || x >= i.ancho || y >= i.alto {
		return
	}
	j := (y*i.ancho + x) * 4
	i.pixeles[j+0] = uint8(c.R)
	i.pixeles[j+1] = uint8(c.G)
	i.pixeles[j+2] = uint8(c.B)
	i.pixeles[j+3] = uint8(c.A)
}

// At: lee un pixel. Devuelve color.Color (no Color) porque es la firma que pide image.Image, y
// SubImage devuelve image.Image como en la Ebitengine de verdad.
func (i *Image) At(x, y int) color.Color {
	if x < 0 || y < 0 || x >= i.ancho || y >= i.alto {
		return color.RGBA{}
	}
	j := (y*i.ancho + x) * 4
	return color.RGBA{R: i.pixeles[j+0], G: i.pixeles[j+1], B: i.pixeles[j+2], A: i.pixeles[j+3]}
}

// ColorAt: la version que devuelve el Color de Ebitengine (0 a 1), que es lo que usa el motor.
func (i *Image) ColorAt(x, y int) Color {
	if x < 0 || y < 0 || x >= i.ancho || y >= i.alto {
		return Color{}
	}
	j := (y*i.ancho + x) * 4
	return Color{R: float64(i.pixeles[j+0]) / 255, G: float64(i.pixeles[j+1]) / 255, B: float64(i.pixeles[j+2]) / 255, A: float64(i.pixeles[j+3]) / 255}
}

// SubImage: la ventana sobre la imagen.
func (i *Image) SubImage(r image.Rectangle) image.Image {
	// se recorta de verdad: el motor pide cada glifo de la tabla de la fuente como una subimagen
	ancho := r.Dx()
	alto := r.Dy()
	if ancho <= 0 || alto <= 0 {
		return &Image{ancho: 0, alto: 0}
	}
	nueva := &Image{pixeles: make([]uint8, ancho*alto*4), ancho: ancho, alto: alto}
	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			c := i.At(r.Min.X+x, r.Min.Y+y)
			if cu, ok := c.(color.RGBA); ok {
				nueva.SetRGBA(x, y, cu)
			}
		}
	}
	return nueva
}

// DrawImage: el metodo con el que el motor dibuja (target.DrawImage(s, op)). Src es *Image (el motor
// pasa lo que le devolvio la asercion de SubImage).
func (i *Image) DrawImage(src *Image, op *DrawImageOptions) { Dibujos++ }

// ================================================================ colores y opciones

// Color: el color de Ebitengine (RGBA de 0 a 1).
type Color struct{ R, G, B, A float64 }

// ColorM: la matriz de color. Aca se guarda tal cual y no se aplica: el filtrado de color real entra
// con el backend (el motor la usa para oscurecer y para las transiciones).
type ColorM struct {
	guardada bool
}

// Scale / Translate / ScaleAlpha: las operaciones que arma el motor. Se guardan como "hubo cambio".
func (c *ColorM) Scale(r, g, b, a float64)   { c.guardada = true }
func (c *ColorM) Translate(r, g, b, a float64) { c.guardada = true }
func (c *ColorM) ScaleAlpha(a float64)       { c.guardada = true }
func (c *ColorM) Reset()                     { c.guardada = false }

// DrawImageOptions: las opciones de dibujado (posicion, escala, color).
type DrawImageOptions struct {
	GeoM             GeoM
	ColorM           ColorM
	CompositeMode    CompositeMode
	Filter           Filter
	DisableMipmaps   bool
}

// GeoM: la matriz de geometria.
type GeoM struct{ guardada bool }

// Translate / Scale / Rotate: las que arma el motor.
func (g *GeoM) Translate(x, y float64) { g.guardada = true }
func (g *GeoM) Scale(x, y float64)     { g.guardada = true }
func (g *GeoM) Rotate(t float64)       { g.guardada = true }
func (g *GeoM) Reset()                 { g.guardada = false }
func (g *GeoM) ResetGeoM()             { g.guardada = false }

// CompositeMode: como se mezcla lo dibujado con lo que habia.
type CompositeMode int

const (
	CompositeModeSourceOver CompositeMode = iota
	CompositeModeCopy
	CompositeModeLighter
)

// Filter: el filtro de escalado.
type Filter int

const (
	FilterNearest Filter = iota
	FilterLinear
)

// ================================================================ el lienzo y el loop

var pantalla = NewImage(640, 480)

// Screen devuelve el lienzo de la consola.
func Screen() *Image { return pantalla }

// DrawImage dibuja src en dst con las opciones. Aca se deja registrado (contando) para poder medir que
// el motor realmente esta pidiendo dibujar; el pintado con el PVR entra con el backend.
var Dibujos int

func DrawImage(dst, src *Image, op *DrawImageOptions) {
	Dibujos++
}

// DrawTriangles: el robot de dibujado directo. Igual que DrawImage: se cuenta.
var DibujosTriangulos int

func DrawTriangles(dst *Image, vertices []Vertex, indices []uint32, op *DrawImageOptions) {
	DibujosTriangulos++
}

// Vertex: el vertice de Ebitengine.
type Vertex struct {
	DstX, DstY float32
	SrcX, SrcY float32
	ColorR     float32
	ColorG     float32
	ColorB     float32
	ColorA     float32
}

// ================================================================ teclado y joystick

// Key: la tecla.
type Key int

// GamepadID: el id del joystick.
type GamepadID int

// GamepadButton: el boton.
type GamepadButton int

// Teclas: las que el motor nombra. No son todas las del PC: las que hacen falta para compilar y las que
// el juego usa de verdad (ver el conteo en la evidencia).
const (
	KeyA Key = iota
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key0 Key = 100 + iota
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeyKP0
	KeyKP1
	KeyKP2
	KeyKP3
	KeyKP4
	KeyKP5
	KeyKP6
	KeyKP7
	KeyKP8
	KeyKP9
	KeyEnter
	KeyEscape
	KeySpace
	KeyBackspace
	KeyTab
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyShift
	KeyControl
	KeyAlt
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyDelete
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
)

// Las teclas que el motor nombra y NO estaban. Medido con:
//   grep -oh "ebiten\.Key[A-Za-z0-9]*" d2core/d2input/ebiten/*.go | sed 's/ebiten\.//' | sort -u
// y comparado contra las de arriba: 37 nombres (medido el 2026-10-16). Los cuatro KeyUp/KeyDown/
// KeyLeft/KeyRight son ALIAS de los Arrow, exactamente como en Ebitengine (ahi siguen por
// compatibilidad). El resto son valores nuevos.
const (
	KeyUp    Key = KeyArrowUp
	KeyDown  Key = KeyArrowDown
	KeyLeft  Key = KeyArrowLeft
	KeyRight Key = KeyArrowRight
)

const (
	KeyApostrophe Key = 1000 + iota
	KeyBackslash
	KeyCapsLock
	KeyComma
	KeyEqual
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyGraveAccent
	KeyInsert
	KeyKPAdd
	KeyKPDecimal
	KeyKPDivide
	KeyKPEnter
	KeyKPEqual
	KeyKPMultiply
	KeyKPSubtract
	KeyLeftBracket
	KeyMenu
	KeyMinus
	KeyNumLock
	KeyPause
	KeyPeriod
	KeyPrintScreen
	KeyRightBracket
	KeyScrollLock
	KeySemicolon
	KeySlash
)

// el estado de las teclas: hoy lo llena nadie (el backend de la consola lo va a llenar con el joystick)
var teclas = map[Key]bool{}

// PonerTecla: la usa el backend de la consola para marcar una tecla. Es la costura por donde entra el
// joystick: el motor pregunta con IsKeyPressed y no se entera de quien la apreto.
func PonerTecla(k Key, apretada bool) { teclas[k] = apretada }

// IsKeyPressed: si la tecla esta apretada.
func IsKeyPressed(k Key) bool { return teclas[k] }

// IsKeyJustPressed: si se apreto en este cuadro (el motor lo usa para los menus). Aca es lo mismo que
// IsKeyPressed: quien tenga que distinguir el flanco lo hace al llenar el estado.
func IsKeyJustPressed(k Key) bool { return teclas[k] }

// AppendInputChars: el texto tipeado (para los campos de texto del juego).
func AppendInputChars(runes []rune) int { return 0 }

// ================================================================ el loop

// Game: la interfaz que el motor implementa (su juego).
type Game interface {
	Update() error
	Draw(screen *Image)
	Layout(ancho, alto int) (int, int)
}

// RunGame arranca el loop. Aca el loop es de la consola: KOS da los cuadros y quien lo llame tiene que
// ceder con el resto del sistema (asi ya corre el resto de los POCs).
func RunGame(g Game, ancho, alto int) error {
	return nil
}

// SetScreenSize: el tamano de la ventana. En la consola es fijo (640x480, lo que da el PVR).
func SetScreenSize(w, h int) {}

// SetFullscreen / SetVsyncEnabled / SetWindowTitle / SetWindowResizingMode: configuracion de PC; en la
// consola no hay ventana.
func SetFullscreen(v bool)      {}
func SetVsyncEnabled(v bool)    {}
func SetWindowTitle(s string)   {}
func SetWindowResizingMode(m int) {}
func SetTPS(tps int)            {}
func SetCursorMode(m int)       {}

// IsFullscreen / WindowSize: para que el motor pueda preguntar.
func IsFullscreen() bool       { return true }
func WindowSize() (int, int)   { return 640, 480 }

// TPS: los cuadros por segundo objetivo.
func TPS() int { return 60 }

// CurrentFPS: medido por el backend.
func CurrentFPS() float64 { return 60 }

// SetWindowIcon: no hay ventana.
func SetWindowIcon(icon []image.Image) {}
