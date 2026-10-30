//go:build dreamcast

package pvr

import (
	"image/color"
)

// Sprite es un sprite PALETIZADO del motor: un byte por pixel (indice) y una paleta de 256 colores.
//
// POR QUE ASI Y NO RGBA: es el formato en el que vienen los DC6 (las imagenes de la interfaz y del suelo) y
// los COF (las capas del personaje y de los monstruos). Un frame de 118x52 en RGBA son 24.544 B; en indices,
// 6.136 B. La paleta es la UNICA de la imagen (los DC6 y los COF de Diablo II la traen aparte, en el .dat
// del acto o en el MPQ), y por eso hay un solo banco de paleta.
//
// EL INDICE 0 ES TRANSPARENTE, y no es una convencion nuestra: asi vienen los DC6/COF del juego y asi lo
// trata el hardware (el PVR con el banco 0 y TXRALPHA_ENABLE). Igual se puede cambiar con Opaco0.
type Sprite struct {
	ancho   int
	alto    int
	indices []byte
	paleta  []uint16

	// imagen: la expansion a RGBA, armada la primera vez que hace falta (no se recalcula por cuadro: la
	// paleta de un sprite no cambia en la partida).
	imagen *Imagen
	// opaco0: si el indice 0 se pinta como cualquier otro (por omision NO: es transparente).
	opaco0 bool
}

// NuevoSprite arma un sprite con sus indices y su paleta en formato del hardware (ARGB1555, 256 entradas).
//
// LA PALETA ES ARGB1555 Y NO RGBA a proposito: es el formato que come el PVR (pvr_set_pal_entry) y el que
// devuelve la conversion de un .dat/PL2 del motor. Convertir aca a RGBA y volver a ARGB1555 en el C seria
// dos pasadas por la misma tabla.
func NuevoSprite(ancho, alto int, indices []byte, paleta []uint16) *Sprite {
	if ancho <= 0 || alto <= 0 || len(indices) < ancho*alto || len(paleta) < 256 {
		return nil
	}

	return &Sprite{ancho: ancho, alto: alto, indices: indices, paleta: paleta}
}

// SpriteDesdePL2: el sprite a partir de una paleta PL2/DAT cruda (768 bytes: 256 colores de 3 canales, sin
// alfa). El motor las lee asi de los MPQ (d2dat.Load) y de ahi salen las de los actos.
func SpriteDesdePL2(ancho, alto int, indices, pl2 []byte) *Sprite {
	if len(pl2) < 768 {
		return nil
	}

	paleta := make([]uint16, 256)
	for i := 0; i < 256; i++ {
		r := uint16(pl2[i*3+0])
		g := uint16(pl2[i*3+1])
		b := uint16(pl2[i*3+2])
		paleta[i] = 0x8000 | ((r >> 3) << 10) | ((g >> 3) << 5) | (b >> 3)
	}

	// el indice 0 es transparente en los DC6/COF del juego, no negro opaco
	paleta[0] = 0

	return NuevoSprite(ancho, alto, indices, paleta)
}

// Ancho y Alto del sprite.
func (s *Sprite) Ancho() int { return s.ancho }

// Alto del sprite.
func (s *Sprite) Alto() int { return s.alto }

// Indices: los bytes de 8 bits por pixel (es lo que sube el PVR tal cual).
func (s *Sprite) Indices() []byte { return s.indices }

// Paleta: las 256 entradas ARGB1555 (lo que espera el banco de paleta del PVR).
func (s *Sprite) Paleta() []uint16 { return s.paleta }

// Opaco0: si el indice 0 se pinta opaco. Por omision es transparente (ver el comentario del tipo). Hay que
// llamarlo ANTES de dibujar para que no quede una expansion vieja en la cache.
func (s *Sprite) Opaco0(opaco bool) {
	if s.opaco0 == opaco {
		return
	}

	s.opaco0 = opaco
	s.imagen = nil
}

// Imagen: la expansion a RGBA del sprite (se arma una vez y se guarda).
func (s *Sprite) Imagen() *Imagen {
	if s.imagen != nil {
		return s.imagen
	}

	img := NuevaImagen(s.ancho, s.alto)
	pix := img.Pixeles()

	for i := 0; i < s.ancho*s.alto; i++ {
		idx := s.indices[i]
		if idx == 0 && !s.opaco0 {
			continue // transparente: el lienzo ya esta en ceros
		}

		v := s.paleta[idx]
		a := uint8(0)
		if v&0x8000 != 0 || s.opaco0 {
			a = 255
		}

		r := uint8(((v >> 10) & 0x1f) * 255 / 31)
		g := uint8(((v >> 5) & 0x1f) * 255 / 31)
		b := uint8((v & 0x1f) * 255 / 31)

		j := i * 4
		pix[j+0] = r
		pix[j+1] = g
		pix[j+2] = b
		pix[j+3] = a
	}

	s.imagen = img

	return img
}

// Dibujar pinta el sprite en el lienzo, con la esquina arriba-izquierda en (x, y) y una escala entera
// (1 = tamano original; el motor de Diablo II no escala sprites, solo mueve).
//
// ESTA ES LA GEOMETRIA DE CAJA DEL POC: el sprite ocupa exactamente (x, y) .. (x+ancho*escala,
// y+alton*escala), sin filtro. La esquina ISO (el punto del suelo bajo el personaje) la aplica el que
// llama, restandole a (x, y) la mitad del ancho y la altura de la caja; aca no se adivina.
func (s *Sprite) Dibujar(dst *Imagen, x, y int, escala int) {
	if s == nil || dst == nil {
		return
	}

	if escala <= 0 {
		escala = 1
	}

	m := identidad()
	if escala != 1 {
		m = m.escalar(float64(escala), float64(escala))
	}

	m = m.trasladar(float64(x), float64(y))
	copiarEn(dst, s.Imagen(), m, ajuste{}, sinForzar)
}

// At: el color RGBA de un pixel del sprite (0,0,0,0 si el indice es 0 y el 0 es transparente).
func (s *Sprite) At(x, y int) color.RGBA { return s.Imagen().Tomar(x, y) }
