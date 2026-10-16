// Package draw: el dibujado de imagenes sobre imagenes, para la consola.
//
// Es el image/draw de Go, con lo que el motor usa: Draw y DrawMask con las operaciones Src y Over (las
// dos de siempre) y los tres puntos rapidos (Drawer, RGBA64Image, los rectangulos). El motor lo usa en
// d2common/d2display/window_frame.go para armar el marco de la ventana.
//
// Limite declarado: no estan las operaciones de mascara por color (Xor, Saturate) ni el dibujado
// concurrente de Go. Si el motor las pide, se ven al medir y se agregan.
package draw

import (
	"image"
	"image/color"
)

// Op es la operacion de dibujado.
type Op int

const (
	// Over: lo que se dibuja se pone encima, con su transparencia (el caso normal).
	Over Op = iota
	// Src: lo que se dibuja reemplaza al destino.
	Src
)

// Drawer es un destino al que se le puede dibujar (la interfaz de Go).
type Drawer interface {
	Draw(r image.Rectangle, src image.Image, sp image.Point)
}

// RGBA64Image: la interfaz que en Go permite dibujar sin convertir a 8 bits.
type RGBA64Image interface {
	image.Image
	RGBA64At(x, y int) color.RGBA64
}

// Draw dibuja src sobre dst, tomando de src el rectangulo sp y poniendolo en r.
func Draw(dst Image, r image.Rectangle, src image.Image, sp image.Point, op Op) {
	DrawMask(dst, r, src, sp, nil, image.Point{}, op)
}

// Image: el destino (en Go es una interfaz con Set/At; aca el motor usa *image.RGBA).
type Image interface {
	image.Image
	Set(x, y int, c color.Color)
}

// DrawMask: igual que Draw, pero con una mascara (el alfa que decide cuanto se ve de lo dibujado).
// mascara nil = todo opaco, que es el caso de Draw.
func DrawMask(dst Image, r image.Rectangle, src image.Image, sp image.Point, mascara image.Image, mp image.Point, op Op) {
	// el rectangulo no puede salirse del destino
	dr := image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: boundsMax(dst, image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 1 << 30, Y: 1 << 30}}).Max.X, Y: boundsMax(dst, image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 1 << 30, Y: 1 << 30}}).Max.Y}}
	r = recortar(r, dr)
	if r.Empty() {
		return
	}

	// los limites del origen
	sb := src.Bounds()
	sp = image.Point{X: sp.X + (r.Min.X - r.Min.X), Y: sp.Y + (r.Min.Y - r.Min.Y)}

	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			sx := sp.X + (x - r.Min.X)
			sy := sp.Y + (y - r.Min.Y)
			if sx < sb.Min.X || sy < sb.Min.Y || sx >= sb.Max.X || sy >= sb.Max.Y {
				continue
			}
			c := src.At(sx, sy)
			if mascara != nil {
				_, _, _, ma := mascara.At(mp.X+(x-r.Min.X), mp.Y+(y-r.Min.Y)).RGBA()
				c = multiplicarAlfa(c, ma)
			}
			if op == Src {
				dst.Set(x, y, c)
				continue
			}
			// Over: mezcla con lo que habia
			or, og, ob, oa := dst.At(x, y).RGBA()
			sr, sg, sb2, sa := c.RGBA()
			if sa == 0xffff {
				dst.Set(x, y, c)
				continue
			}
			if sa == 0 {
				continue
			}
			// fuente-encima: out = src + dst*(1 - src_a)
			r8 := uint8((sr*0xffff + or*(0xffff-sa)) / 0xffff >> 8)
			g8 := uint8((sg*0xffff + og*(0xffff-sa)) / 0xffff >> 8)
			b8 := uint8((sb2*0xffff + ob*(0xffff-sa)) / 0xffff >> 8)
			a8 := uint8((sa*0xffff + oa*(0xffff-sa)) / 0xffff >> 8)
			dst.Set(x, y, color.RGBA{R: r8, G: g8, B: b8, A: a8})
		}
	}
}

// multiplicarAlfa aplica el alfa de la mascara a un color.
func multiplicarAlfa(c color.Color, ma uint32) color.Color {
	r, g, b, a := c.RGBA()
	nr := uint8((r * ma) / 0xffff >> 8)
	ng := uint8((g * ma) / 0xffff >> 8)
	nb := uint8((b * ma) / 0xffff >> 8)
	na := uint8((a * ma) / 0xffff >> 8)
	return color.RGBA{R: nr, G: ng, B: nb, A: na}
}

// recortar devuelve la parte de r que entra en limite.
func recortar(r, limite image.Rectangle) image.Rectangle {
	rect := image.Rectangle{Min: r.Min, Max: r.Max}
	if rect.Min.X < limite.Min.X {
		rect.Min.X = limite.Min.X
	}
	if rect.Min.Y < limite.Min.Y {
		rect.Min.Y = limite.Min.Y
	}
	if rect.Max.X > limite.Max.X {
		rect.Max.X = limite.Max.X
	}
	if rect.Max.Y > limite.Max.Y {
		rect.Max.Y = limite.Max.Y
	}
	if rect.Max.X < rect.Min.X {
		rect.Max.X = rect.Min.X
	}
	if rect.Max.Y < rect.Min.Y {
		rect.Max.Y = rect.Min.Y
	}
	return rect
}

// boundsMax: el rectangulo de una imagen (para saber hasta donde se puede dibujar).
func boundsMax(img image.Image, def image.Rectangle) image.Rectangle {
	if img == nil {
		return def
	}
	return img.Bounds()
}
