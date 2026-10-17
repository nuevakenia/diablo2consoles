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
	"fmt"
	"image"
	"image/color"
	"math"
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
// Fill: pinta toda la imagen de un color. MEDIDO en ebiten v2.0.2 (image.go): la firma es
// Fill(clr color.Color). Lo pide d2render/ebiten/ebiten_panic_screen.go:44 (pinta la pantalla de error).
func (i *Image) Fill(clr color.Color) {
	r, g, b, a := clr.RGBA()
	i.LlenarDeRGBA(color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
}

// LlenarDeRGBA: pinta todo de un color (la parte de adentro, sin la conversion).
func (i *Image) LlenarDeRGBA(c color.RGBA) {
	for y := 0; y < i.alto; y++ {
		for x := 0; x < i.ancho; x++ {
			i.SetRGBA(x, y, c)
		}
	}
}

// ReplacePixels: cambia TODOS los pixeles de una vez. MEDIDO en ebiten v2.0.2 (image.go:699): recibe los
// bytes en RGBA (4 por pixel, en el orden rojo, verde, azul, alfa) y el largo tiene que ser el de la
// imagen; si no, Ebitengine revienta. Aca, si el largo no da, no se hace nada (en la consola se prefiere no
// cambiar nada antes que corromper la pantalla o caerse).
// Lo pide d2core/d2render/ebiten/ebiten_surface.go:450, que es como el motor sube un cuadro al PVR.
func (i *Image) ReplacePixels(pixeles []byte) {
	if len(pixeles) != len(i.pixeles) {
		return
	}
	copy(i.pixeles, pixeles)
}

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

// ColorM: la matriz de color (4x4 mas el desplazamiento), la misma cuenta que hace Ebitengine.
//
// PARA QUE SE USA: el motor la usa para oscurecer la pantalla cuando se abre un menu, para las
// transiciones (fundidos), y para el destello rojo cuando te pegan.
//
// ES LA CUENTA DE VERDAD, no una marca de "hubo cambio". La forma esta COPIADA de la implementacion real
// medida en ebiten v2.0.2 (internal/affine/colorm.go): los 16 numeros de la matriz mas los 4 del
// desplazamiento, y el mismo orden de indices (SetElement(i, j) escribe body[i + j*4], y con j==4 escribe
// el desplazamiento i). Si el orden fuera otro, un tinte daria otro color y nadie lo notaria hasta verlo
// en pantalla.
type ColorM struct {
	guardada bool
	cuerpo   [16]float64
	traslado [4]float64
}

// la identidad: la matriz que deja los colores como estan
var coloresIdentidad = [16]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

// iniciar: deja la matriz lista para escribir (partiendo de la identidad).
func (c *ColorM) iniciar() {
	if c.guardada {
		return
	}
	c.cuerpo = coloresIdentidad
	c.traslado = [4]float64{0, 0, 0, 0}
	c.guardada = true
}

// Reset: vuelve a la identidad.
func (c *ColorM) Reset() { c.guardada = false }

// Scale: multiplica cada canal por su factor (así se oscurece: Scale(0.5, 0.5, 0.5, 1)).
func (c *ColorM) Scale(r, g, b, a float64) {
	c.iniciar()
	factores := [4]float64{r, g, b, a}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c.cuerpo[i+j*4] *= factores[j]
		}
	}
}

// ScaleAlpha: escala solo el canal del alfa (la transparencia).
func (c *ColorM) ScaleAlpha(a float64) { c.Scale(1, 1, 1, a) }

// Translate: suma un valor a cada canal (así se aclara: Translate(0.2, 0.2, 0.2, 0)).
func (c *ColorM) Translate(r, g, b, a float64) {
	c.iniciar()
	c.traslado[0] += r
	c.traslado[1] += g
	c.traslado[2] += b
	c.traslado[3] += a
}

// Element: un numero de la matriz. Con j==4 es el desplazamiento del canal i.
// MEDIDO (internal/affine/colorm.go:323): Element(i, j) es body[i+j*4] y con j==4 es traslado[i].
func (c *ColorM) Element(i, j int) float64 {
	if i < 0 || i >= 4 || j < 0 || j >= 5 {
		return 0
	}
	if !c.guardada {
		if j < 4 {
			return coloresIdentidad[i+j*4]
		}
		return 0
	}
	if j < 4 {
		return c.cuerpo[i+j*4]
	}
	return c.traslado[i]
}

// SetElement: escribe un numero de la matriz.
func (c *ColorM) SetElement(i, j int, elemento float64) {
	if i < 0 || i >= 4 || j < 0 || j >= 5 {
		return
	}
	c.iniciar()
	if j < 4 {
		c.cuerpo[i+j*4] = elemento
	} else {
		c.traslado[i] = elemento
	}
}

// Concat: multiplica esta matriz por la otra (en ese orden). Es como se combinan dos cambios de color.
func (c *ColorM) Concat(otra ColorM) {
	c.iniciar()
	var nuevo [16]float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			suma := 0.0
			for k := 0; k < 4; k++ {
				suma += c.cuerpo[i+k*4] * otra.Element(k, j)
			}
			nuevo[i+j*4] = suma
		}
		// el desplazamiento tambien se arrastra
		suma := c.traslado[i]
		for k := 0; k < 4; k++ {
			suma += c.cuerpo[i+k*4] * otra.Element(k, 4)
		}
		c.traslado[i] = suma
	}
	c.cuerpo = nuevo
}

// Apply: aplica la matriz a un color. La cuenta es la de Ebitengine, medida en
// internal/affine/colorm.go:118: se desmultiplica el alfa (Go guarda los colores ya multiplicados por el
// alfa), se multiplica el vector (r, g, b, a, 1) por la matriz, y se recorta a [0, 1].
func (c *ColorM) Apply(clr color.Color) color.Color {
	if !c.guardada {
		return clr
	}

	r, g, b, a := clr.RGBA()
	rf, gf, bf, af := 0.0, 0.0, 0.0, 0.0
	if a > 0 {
		rf = float64(r) / float64(a)
		gf = float64(g) / float64(a)
		bf = float64(b) / float64(a)
		af = float64(a) / 0xffff
	}

	r2 := c.cuerpo[0]*rf + c.cuerpo[4]*gf + c.cuerpo[8]*bf + c.cuerpo[12]*af + c.traslado[0]
	g2 := c.cuerpo[1]*rf + c.cuerpo[5]*gf + c.cuerpo[9]*bf + c.cuerpo[13]*af + c.traslado[1]
	b2 := c.cuerpo[2]*rf + c.cuerpo[6]*gf + c.cuerpo[10]*bf + c.cuerpo[14]*af + c.traslado[2]
	a2 := c.cuerpo[3]*rf + c.cuerpo[7]*gf + c.cuerpo[11]*bf + c.cuerpo[15]*af + c.traslado[3]

	return color.NRGBA64{
		R: uint16(recortar01(r2) * 0xffff),
		G: uint16(recortar01(g2) * 0xffff),
		B: uint16(recortar01(b2) * 0xffff),
		A: uint16(recortar01(a2) * 0xffff),
	}
}

// recortar01: deja el valor entre 0 y 1 (asi lo hace Ebitengine).
func recortar01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Los dos cambios de base del matiz, con los numeros de Ebitengine (internal/affine/colorm.go:547 y 555).
var (
	rgbAYCbCr = [16]float64{
		0.2990, -0.1687, 0.5000, 0,
		0.5870, -0.3313, -0.4187, 0,
		0.1140, 0.5000, -0.0813, 0,
		0, 0, 0, 1,
	}
	yCbCrARgb = [16]float64{
		1, 1, 1, 0,
		0, -0.34414, 1.77200, 0,
		1.40200, -0.71414, 0, 0,
		0, 0, 0, 1,
	}
)

// matrizDe: arma una ColorM con esos numeros (para el ChangeHSV).
func matrizDe(cuerpo [16]float64) ColorM {
	return ColorM{guardada: true, cuerpo: cuerpo}
}

// ChangeHSV: cambia el matiz (el angulo), la saturacion y el brillo.
//
// Es la cuenta de Ebitengine, copiada de internal/affine/colorm.go:566: pasa el color a YCbCr (donde el
// matiz es un giro), gira, escala, y vuelve a RGB. Los tres pasos son: Concat(rgbToYCbCr),
// Concat(el giro), Scale(brillo, sat*brillo, sat*brillo, 1), Concat(yCbCrToRgb).
func (c *ColorM) ChangeHSV(anguloMatiz, escalaSaturacion, escalaBrillo float64) {
	seno, coseno := math.Sincos(anguloMatiz)

	c.iniciar()
	c.Concat(matrizDe(rgbAYCbCr))
	c.Concat(matrizDe([16]float64{
		1, 0, 0, 0,
		0, coseno, seno, 0,
		0, -seno, coseno, 0,
		0, 0, 0, 1,
	}))
	c.Scale(escalaBrillo, escalaSaturacion*escalaBrillo, escalaSaturacion*escalaBrillo, 1)
	c.Concat(matrizDe(yCbCrARgb))
}

// RotateHue: gira el matiz (es ChangeHSV con saturacion y brillo en 1).
func (c *ColorM) RotateHue(angulo float64) { c.ChangeHSV(angulo, 1, 1) }

// IsInvertible: si la matriz se puede invertir (si su determinante no es cero).
func (c *ColorM) IsInvertible() bool {
	if !c.guardada {
		return true
	}
	return math.Abs(c.determinante3x3()) > 1e-12
}

// Invert: da vuelta la matriz. Si no se puede, no hace nada (en Ebitengine esto explota; aca se prefiere
// dejar el color como estaba antes que caerse en la consola).
func (c *ColorM) Invert() {
	if !c.IsInvertible() {
		return
	}
	// resolver A * X = I por eliminacion de Gauss (4x4), y arrastrar el desplazamiento
	a := [4][8]float64{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			a[i][j] = c.cuerpo[i+j*4]
		}
		a[i][4+i] = 1
	}
	for col := 0; col < 4; col++ {
		pivote := col
		for f := col + 1; f < 4; f++ {
			if math.Abs(a[f][col]) > math.Abs(a[pivote][col]) {
				pivote = f
			}
		}
		a[col], a[pivote] = a[pivote], a[col]
		escala := a[col][col]
		for j := 0; j < 8; j++ {
			a[col][j] /= escala
		}
		for f := 0; f < 4; f++ {
			if f == col {
				continue
			}
			factor := a[f][col]
			for j := 0; j < 8; j++ {
				a[f][j] -= factor * a[col][j]
			}
		}
	}

	var nueva [16]float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			nueva[i+j*4] = a[i][4+j]
		}
	}

	// el desplazamiento invertido es "-inversa * traslado"
	var nuevoTraslado [4]float64
	for i := 0; i < 4; i++ {
		suma := 0.0
		for j := 0; j < 4; j++ {
			suma += nueva[i+j*4] * c.traslado[j]
		}
		nuevoTraslado[i] = -suma
	}

	c.cuerpo = nueva
	c.traslado = nuevoTraslado
}

// determinante3x3: el determinante de la parte de color (3x3), que es lo que decide si se puede invertir.
func (c *ColorM) determinante3x3() float64 {
	m := c.cuerpo
	return m[0]*(m[5]*m[10]-m[9]*m[6]) -
		m[4]*(m[1]*m[10]-m[9]*m[2]) +
		m[8]*(m[1]*m[6]-m[5]*m[2])
}

// String: el texto de la matriz, con el mismo formato que Ebitengine.
func (c *ColorM) String() string {
	return fmt.Sprintf("[[%f, %f, %f, %f, %f], [%f, %f, %f, %f, %f], [%f, %f, %f, %f, %f], [%f, %f, %f, %f, %f]]",
		c.Element(0, 0), c.Element(0, 1), c.Element(0, 2), c.Element(0, 3), c.Element(0, 4),
		c.Element(1, 0), c.Element(1, 1), c.Element(1, 2), c.Element(1, 3), c.Element(1, 4),
		c.Element(2, 0), c.Element(2, 1), c.Element(2, 2), c.Element(2, 3), c.Element(2, 4),
		c.Element(3, 0), c.Element(3, 1), c.Element(3, 2), c.Element(3, 3), c.Element(3, 4))
}

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
// Skew: inclina la figura. MEDIDO en ebiten v2.0.2 (geom.go:149): Skew(skewX, skewY float64). Lo pide
// d2core/d2render/ebiten/ebiten_surface.go:347.
func (g *GeoM) Skew(x, y float64)       { g.guardada = true }
func (g *GeoM) Reset()                 { g.guardada = false }
func (g *GeoM) ResetGeoM()             { g.guardada = false }

// CompositeMode: como se mezcla lo dibujado con lo que habia.
type CompositeMode int

const (
	CompositeModeSourceOver CompositeMode = iota
	CompositeModeCopy
	CompositeModeLighter
	// Multiply: el motor lo pide (d2render/ebiten/ebiten_surface.go:251, para oscurecer). Medido en
	// ebiten v2.0.2 (graphics.go:88): ahi existe ademas de Lighter y SourceOver.
	CompositeModeMultiply
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

// DrawTriangles: dibuja triangulos. Es lo que usa el motor para las superficies con textura
// (d2render/ebiten/ebiten_surface.go:610).
//
// MEDIDO en ebiten v2.0.2 (image.go:264): es un METODO de Image, los indices son uint16 (no uint32) y el
// ultimo parametro es otro *Image (la textura):
//
//	func (i *Image) DrawTriangles(vertices []Vertex, indices []uint16, img *Image, options *DrawTrianglesOptions)
//
// Aca estaba escrito como una funcion suelta con indices uint32, y el motor no compilaba por eso.
var DibujosTriangulos int

func (i *Image) DrawTriangles(vertices []Vertex, indices []uint16, img *Image, options *DrawTrianglesOptions) {
	DibujosTriangulos++
}

// DrawTrianglesOptions: las opciones del dibujado de triangulos. MEDIDO en ebiten v2.0.2 (image.go:229):
// el motor las arma con la matriz de color, el modo de composicion y el filtro.
type DrawTrianglesOptions struct {
	ColorM        ColorM
	CompositeMode CompositeMode
	Filter        Filter
	AntiAlias     bool
	// Address: el modo de direccionamiento de la textura. En la consola el PVR no lo necesita (las
	// texturas del juego no se salen de su marco); esta para que la estructura tenga la misma forma.
	Address int
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
func SetCursorMode(m CursorMode) { cursorModo = m }

// IsFullscreen / WindowSize: para que el motor pueda preguntar.
func IsFullscreen() bool       { return true }
func WindowSize() (int, int)   { return 640, 480 }

// TPS: los cuadros por segundo objetivo.
func TPS() int { return 60 }

// CurrentFPS: medido por el backend.
func CurrentFPS() float64 { return 60 }

// SetWindowIcon: no hay ventana.
func SetWindowIcon(icon []image.Image) {}
