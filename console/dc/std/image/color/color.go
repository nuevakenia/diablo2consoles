// Package color: los colores de la biblioteca estandar, en version para la consola.
//
// Es la base de image y del backend de render: el motor habla de colores en todas partes (paletas,
// paletas de DC6, el fondo, los tintes de los sprites).
//
// Se implementa lo que el motor usa de verdad, con las mismas formulas que Go (las conversiones
// RGBA<->NRGBA<->Gray son cuentas exactas y se pueden comparar contra las de verdad en el host).
package color

// Color es la interfaz de Go, igual: cualquier color sabe darse en RGBA premultiplicado de 16 bits.
type Color interface {
	RGBA() (r, g, b, a uint32)
}

// Model convierte un color a otro modelo. Es la interfaz que usa image para saber en que modelo esta.
type Model interface {
	Convert(c Color) Color
}

// ---------------------------------------------------------------- los modelos

type ModelFunc func(Color) Color

func (f ModelFunc) Convert(c Color) Color { return f(c) }

var (
	RGBAModel    Model = ModelFunc(modelRGBAToRGBA)
	NRGBAModel   Model = ModelFunc(modelNRGBAToNRGBA)
	GrayModel    Model = ModelFunc(modelGrayToGray)
	AlphaModel   Model = ModelFunc(modelAlphaToAlpha)
)

func modelRGBAToRGBA(c Color) Color {
	if _, ok := c.(RGBA); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	return RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func modelNRGBAToNRGBA(c Color) Color {
	if _, ok := c.(NRGBA); ok {
		return c
	}
	return toNRGBA(c)
}

func modelGrayToGray(c Color) Color {
	if _, ok := c.(Gray); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	// el gris de Go: 19595/65536 de rojo + 38470/65536 de verde + 7471/65536 de azul
	y := (19595*uint32(r) + 38470*uint32(g) + 7471*uint32(b) + 1<<15) >> 24
	return Gray{uint8(y)}
}

func modelAlphaToAlpha(c Color) Color {
	if _, ok := c.(Alpha); ok {
		return c
	}
	_, _, _, a := c.RGBA()
	return Alpha{uint8(a >> 8)}
}

// ---------------------------------------------------------------- RGBA (premultiplicado)
type RGBA struct {
	R, G, B, A uint8
}

func (c RGBA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	r *= uint32(c.A)
	r /= 0xff

	g = uint32(c.G)
	g |= g << 8
	g *= uint32(c.A)
	g /= 0xff

	b = uint32(c.B)
	b |= b << 8
	b *= uint32(c.A)
	b /= 0xff

	a = uint32(c.A)
	a |= a << 8
	return
}

// ---------------------------------------------------------------- NRGBA (sin premultiplicar)
type NRGBA struct {
	R, G, B, A uint8
}

func (c NRGBA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	r *= uint32(c.A)
	r /= 0xff

	g = uint32(c.G)
	g |= g << 8
	g *= uint32(c.A)
	g /= 0xff

	b = uint32(c.B)
	b |= b << 8
	b *= uint32(c.A)
	b /= 0xff

	a = uint32(c.A)
	a |= a << 8
	return
}

// ---------------------------------------------------------------- otros modelos que usa el motor
type Gray struct{ Y uint8 }

func (c Gray) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y)
	y |= y << 8
	return y, y, y, 0xffff
}

type Alpha struct{ A uint8 }

func (c Alpha) RGBA() (r, g, b, a uint32) {
	a = uint32(c.A)
	a |= a << 8
	return a, a, a, a
}

type Gray16 struct{ Y uint16 }

func (c Gray16) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y)
	return y, y, y, 0xffff
}

type Alpha16 struct{ A uint16 }

func (c Alpha16) RGBA() (r, g, b, a uint32) {
	a = uint32(c.A)
	return a, a, a, a
}

type RGBA64 struct {
	R, G, B, A uint16
}

func (c RGBA64) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r *= uint32(c.A)
	r /= 0xffff

	g = uint32(c.G)
	g *= uint32(c.A)
	g /= 0xffff

	b = uint32(c.B)
	b *= uint32(c.A)
	b /= 0xffff

	a = uint32(c.A)
	return
}

type NRGBA64 struct {
	R, G, B, A uint16
}

func (c NRGBA64) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r *= uint32(c.A)
	r /= 0xffff

	g = uint32(c.G)
	g *= uint32(c.A)
	g /= 0xffff

	b = uint32(c.B)
	b *= uint32(c.A)
	b /= 0xffff

	a = uint32(c.A)
	return
}

// ---------------------------------------------------------------- las conversiones publicas de Go

// RGBAToYCbCr y YCbCrToRGB: la conversion de la television, tal cual la de Go.
func RGBAToYCbCr(r, g, b uint8) (yy, cb, cr uint8) {
	yy, cb, cr = rgbToYCbCr(r, g, b)
	return
}

// NRGBAAToNRGBA / RGBAToNRGBA: las conversiones entre premultiplicado y no.
func RGBAToNRGBA(c RGBA) NRGBA {
	if c.A == 0xff {
		return NRGBA{c.R, c.G, c.B, 0xff}
	}
	if c.A == 0 {
		return NRGBA{0, 0, 0, 0}
	}
	r := (uint32(c.R) * 0xffff) / uint32(c.A)
	g := (uint32(c.G) * 0xffff) / uint32(c.A)
	b := (uint32(c.B) * 0xffff) / uint32(c.A)
	return NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), c.A}
}

func toNRGBA(c Color) NRGBA {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return NRGBA{0, 0, 0, 0}
	}
	if a == 0xffff {
		return NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
	}
	r = (r * 0xffff) / a
	g = (g * 0xffff) / a
	b = (b * 0xffff) / a
	return NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

// ---------------------------------------------------------------- las paletas

// Palette es una paleta de colores: la usan los DC6 y los PL2 del juego.
type Palette []Color

func (p Palette) Convert(c Color) Color {
	if len(p) == 0 {
		return RGBA{}
	}
	return p[p.Index(c)]
}

// Index devuelve el indice del color mas parecido en la paleta (distancia al cuadrado en RGBA, igual
// que Go: no hay busqueda inteligente, es O(n) por color y para las paletas del juego alcanza).
func (p Palette) Index(c Color) int {
	cr, cg, cb, ca := c.RGBA()
	ret, best := 0, uint32(1<<31-1)
	for i, cc := range p {
		r, g, b, a := cc.RGBA()
		d := sq(int(cr) - int(r))
		d += sq(int(cg) - int(g))
		d += sq(int(cb) - int(b))
		d += sq(int(ca) - int(a))
		if uint32(d) < best {
			best = uint32(d)
			ret = i
		}
	}
	return ret
}

func sq(v int) int { return v * v }

// ---------------------------------------------------------------- formulas internas
func rgbToYCbCr(r, g, b uint8) (uint8, uint8, uint8) {
	yy := (19595*uint32(r) + 38470*uint32(g) + 7471*uint32(b) + 1<<15) >> 16
	// los negativos van en uint32 (complemento a dos), que es como lo hace Go internamente
	cb := ((1<<16-11056)*uint32(r) + (1<<16-21712)*uint32(g) + 32768*uint32(b) + 257<<15) >> 16
	cr := (32768*uint32(r) + (1<<16-27440)*uint32(g) + (1<<16-5328)*uint32(b) + 257<<15) >> 16
	return uint8(yy), uint8(cb), uint8(cr)
}
