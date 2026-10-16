// Package image: las imagenes de la biblioteca estandar, en version para la consola.
//
// Lo que se implementa es lo que el motor usa de verdad: las imagenes con PALETA (los DC6 y las
// superficies del juego son indices + paleta, que es exactamente como trabaja el PowerVR2) y las
// RGBA/NRGBA para lo que se compone en memoria. Las interfaces son las de Go, palabra por palabra,
// asi que este paquete se puede probar contra el de verdad en el host.
//
// Limite declarado: NO estan los formatos de archivo ni image/draw. El juego no los necesita en la
// consola: los assets entran ya decodificados y el dibujo lo hace el PVR.
package image

import (
	"image/color"
)

// Point y Rectangle: las dos coordenadas que usa el motor.
type Point struct{ X, Y int }

func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }
func (p Point) In(r Rectangle) bool {
	return r.Min.X <= p.X && p.X < r.Max.X && r.Min.Y <= p.Y && p.Y < r.Max.Y
}

type Rectangle struct{ Min, Max Point }

func (r Rectangle) Dx() int      { return r.Max.X - r.Min.X }
func (r Rectangle) Dy() int      { return r.Max.Y - r.Min.Y }
func (r Rectangle) Size() Point  { return Point{r.Dx(), r.Dy()} }
func (r Rectangle) Empty() bool  { return r.Max.X <= r.Min.X || r.Max.Y <= r.Min.Y }
func (r Rectangle) Add(p Point) Rectangle {
	return Rectangle{Point{r.Min.X + p.X, r.Min.Y + p.Y}, Point{r.Max.X + p.X, r.Max.Y + p.Y}}
}
func (r Rectangle) Intersect(s Rectangle) Rectangle {
	if r.Min.X < s.Min.X {
		r.Min.X = s.Min.X
	}
	if r.Min.Y < s.Min.Y {
		r.Min.Y = s.Min.Y
	}
	if r.Max.X > s.Max.X {
		r.Max.X = s.Max.X
	}
	if r.Max.Y > s.Max.Y {
		r.Max.Y = s.Max.Y
	}
	if r.Empty() {
		return Rectangle{}
	}
	return r
}

// Image es la interfaz de Go, igual.
type Image interface {
	ColorModel() color.Model
	Bounds() Rectangle
	At(x, y int) color.Color
}

// ---------------------------------------------------------------- Paletted: indices + paleta
//
// Esta es la importante para este port: un DC6, una superficie del juego o una textura del PVR son
// indices de 8 bits + una paleta de 256 colores. Tenerla como una imagen mas hace que el motor la
// trate como cualquier otra.
type Paletted struct {
	Pix    []uint8
	Stride int
	Rect   Rectangle
	Palette color.Palette
}

func NewPaletted(r Rectangle, p color.Palette) *Paletted {
	return &Paletted{Pix: make([]uint8, (r.Dx()*r.Dy()+3)&^3), Stride: r.Dx(), Rect: r, Palette: p}
}

func (p *Paletted) ColorModel() color.Model { return p.Palette }
func (p *Paletted) Bounds() Rectangle       { return p.Rect }

func (p *Paletted) At(x, y int) color.Color {
	if !(Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	i := p.PixOffset(x, y)
	return p.Palette[p.Pix[i]]
}

func (p *Paletted) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x - p.Rect.Min.X)
}

func (p *Paletted) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = uint8(p.Palette.Index(c))
}

func (p *Paletted) SetColorIndex(x, y int, index uint8) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	p.Pix[p.PixOffset(x, y)] = index
}

func (p *Paletted) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	if r.Empty() {
		return &Paletted{Rect: r, Palette: p.Palette}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &Paletted{
		Pix:     p.Pix[i:],
		Stride:  p.Stride,
		Rect:    r,
		Palette: p.Palette,
	}
}

// Opaque: si no hay ningun indice con alfa 0 en la paleta usada, la imagen es opaca.
func (p *Paletted) Opaque() bool {
	used := make(map[uint8]bool, 256)
	for _, v := range p.Pix {
		used[v] = true
	}
	for v := range used {
		_, _, _, a := p.Palette[v].RGBA()
		if a != 0xffff {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- RGBA (la imagen de trabajo)
type RGBA struct {
	Pix    []uint8 // R, G, B, A por pixel
	Stride int
	Rect   Rectangle
}

func NewRGBA(r Rectangle) *RGBA {
	return &RGBA{Pix: make([]uint8, 4*r.Dx()*r.Dy()), Stride: 4 * r.Dx(), Rect: r}
}

func (p *RGBA) ColorModel() color.Model { return color.RGBAModel }
func (p *RGBA) Bounds() Rectangle       { return p.Rect }

func (p *RGBA) At(x, y int) color.Color {
	if !(Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4]
	return color.RGBA{s[0], s[1], s[2], s[3]}
}

func (p *RGBA) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.RGBAModel.Convert(c).(color.RGBA)
	s := p.Pix[i : i+4 : i+4]
	s[0], s[1], s[2], s[3] = c1.R, c1.G, c1.B, c1.A
}

func (p *RGBA) SetRGBA(x, y int, c color.RGBA) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4]
	s[0], s[1], s[2], s[3] = c.R, c.G, c.B, c.A
}

func (p *RGBA) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*4
}

func (p *RGBA) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	if r.Empty() {
		return &RGBA{Rect: r}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &RGBA{Pix: p.Pix[i:], Stride: p.Stride, Rect: r}
}

// ---------------------------------------------------------------- las funciones de conversion
//
// ModelToRGBA / ModelToPaletted / RGBAAtIndex: las que usa el codigo del motor para pasar de la
// imagen de indices a una RGBA (para tintes y transparencias) y al reves.

// RGBAAtIndex saca el color RGBA de un indice en una paleta.
func RGBAAtIndex(pal color.Palette, i uint8) color.RGBA {
	r, g, b, a := pal[i].RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

// ToRGBA convierte cualquier imagen a RGBA (el equivalente a image/draw.Draw sin el dibujo).
func ToRGBA(src Image) *RGBA {
	b := src.Bounds()
	dst := NewRGBA(Rectangle{Point{0, 0}, Point{b.Dx(), b.Dy()}})
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, src.At(x+b.Min.X, y+b.Min.Y))
		}
	}
	return dst
}

// IndicesDePaletted devuelve los indices crudos, que es lo que se sube al PVR como textura.
func IndicesDePaletted(p *Paletted) []byte { return p.Pix }

// Rect devuelve el rectangulo que va de (x0,y0) a (x1,y1), con los minimos y maximos bien puestos
// (igual que go). El motor lo usa en muchos lugares, y ebiten.Bounds tambien.
func Rect(x0, y0, x1, y1 int) Rectangle {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return Rectangle{Min: Point{X: x0, Y: y0}, Max: Point{X: x1, Y: y1}}
}
