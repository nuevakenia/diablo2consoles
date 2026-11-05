//go:build dreamcast

package ebiten

// DC-002.155 -- LA SUPERFICIE DEL MOTOR SABE PINTAR UNA IMAGEN DE INDICES (el camino de la cache del mapa).
//
// POR QUE EXISTE. `d2maprenderer` guarda la cache de tiles como indices de 8 bits cuando se lo pide
// (DC-002.101) y para pintarla le pide al destino el metodo `RenderIndicesTintado`. En la consola el destino
// del motor es ESTA superficie (la de `d2core/d2render/ebiten`, que abajo es el shim de
// `console/dc/engine/ebiten/v2`), y sin este metodo el renderer avisa "la Surface del cuadro no sabe dibujar
// indices" y NO PINTA: el cuadro del pueblo sale negro.
//
// LA CUENTA LA HACE EL SHIM (`(*ebiten.Image).PintarIndicesTintado`), que es quien tiene los pixeles. Aca
// solo se traduce el ESTADO de la superficie -- la traslacion y el color de `PushColor` -- al formato que el
// shim entiende. El color se resuelve como en el backend PVR (`pvr.aRGBA`): si el que dibuja paso un
// `color.RGBA` se usa tal cual (el alfa de las sombras es 160 y se lee derecho), y si paso otra
// implementacion de `color.Color` se desmultiplica, porque `RGBA()` devuelve el color con el alfa ya
// aplicado.
//
// EL ARCHIVO VA CON `//go:build dreamcast` Y ESO ES SOLO PARA EL `go build` DEL HOST (que asi lo saltea).
// El build de la consola NO lee esa linea: arma la lista de fuentes por NOMBRE
// (`scripts/console/palancas-del-build.sh`), y este archivo no lleva ninguno de los sufijos excluidos, asi
// que entra. Si algun dia se cambia de nombre, tiene que seguir sin sufijo de plataforma.

import (
	"image/color"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
)

// static check: el renderer del mapa pide exactamente esta firma (`lienzoDeIndices`, DC-002.101)
var _ interface {
	RenderIndicesTintado(indices []byte, ancho, alto int, paleta [][4]uint8)
} = &ebitenSurface{}

// RenderIndicesTintado: pinta `indices` (8 bits, `ancho` x `alto`) con `paleta` en esta superficie, en la
// posicion actual de la traslacion y con el tinte del estado actual.
func (s *ebitenSurface) RenderIndicesTintado(indices []byte, ancho, alto int, paleta [][4]uint8) {
	rojo, verde, azul, alfa := uint8(255), uint8(255), uint8(255), uint8(255)

	if c := s.stateCurrent.color; c != nil {
		rgba := comoRGBA(c)
		rojo, verde, azul, alfa = rgba.R, rgba.G, rgba.B, rgba.A
	}

	pintados := s.image.PintarIndicesTintado(indices, ancho, alto, paleta,
		s.stateCurrent.x, s.stateCurrent.y, rojo, verde, azul, alfa)

	if pintados == 0 {
		SuperficiesDeIndicesVacias++
	}
}

// SuperficiesDeIndicesVacias: cuantas veces se pidio pintar indices y no entro ni un pixel. Es el
// instrumento que separa "el renderer no dibujo" de "dibujo fuera del lienzo".
var SuperficiesDeIndicesVacias int

// comoRGBA: el color del estado en RGBA de 8 bits, con la misma regla que `pvr.aRGBA` (DC-002.101).
func comoRGBA(c color.Color) color.RGBA {
	switch v := c.(type) {
	case color.RGBA:
		return v
	case color.NRGBA:
		return color.RGBA{R: v.R, G: v.G, B: v.B, A: v.A}
	}

	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.RGBA{}
	}

	return color.RGBA{
		R: uint8(r * 255 / a),
		G: uint8(g * 255 / a),
		B: uint8(b * 255 / a),
		A: uint8(a >> 8),
	}
}

// ExpandirIndices expande los indices de 8 bits de un cuadro de animacion DIRECTAMENTE en el lienzo de esta
// superficie. (`superficieConLienzo` de `d2core/d2asset/cuadro_paletizado.go`.)
//
// POR QUE EXISTE (DC-002.159c, medido). Cargar un cuadro pedia `d2util.ImgIndexToRGBA`: un buffer de 4 x pixeles
// en el monton que se copiaba al lienzo con `ReplacePixels` y se tiraba. Con el compuesto del heroe son 7 capas
// por cuadro, y caminando el cuadro cambia casi todos los cuadros de video: basura a 7 buffers por cuadro
// justo cuando el recolector de la consola ya no tiene aire (semiespacio de 1.835.008 B, y el caminar del
// DC-002.159b moria con `fatal error: stack_alloc: out of memory`).
//
// La cuenta es exactamente la de `d2util.ImgIndexToRGBAEn` (el indice 0 transparente, el resto RGBA de su
// color), que es la MISMA que usaba `ImgIndexToRGBA`: aca el buffer que presta `d2util.ImgIndexToRGBA` es el
// lienzo de la superficie. El shim, que es el dueno de los pixeles, solo presta el slice
// (`(*ebiten.Image).LienzoDePixeles`, en `console/dc/engine/ebiten/v2/lienzo.go`).
func (s *ebitenSurface) ExpandirIndices(indices []byte, paleta d2interface.Palette) {
	if s == nil || s.image == nil || len(indices) == 0 {
		return
	}

	d2util.ImgIndexToRGBAEn(s.image.LienzoDePixeles(), indices, paleta)
}

// static check: la superficie del motor cumple la parte de la consola que carga cuadros sin buffer intermedio.
var _ interface {
	ExpandirIndices(indices []byte, paleta d2interface.Palette)
} = &ebitenSurface{}

// la superficie sigue cumpliendo el contrato del motor
var _ d2interface.Surface = &ebitenSurface{}
