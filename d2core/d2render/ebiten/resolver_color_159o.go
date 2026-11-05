package ebiten

// DC-002.159o -- EL COLOR SE RESUELVE AL EMPUJARLO, NO AL DIBUJARLO.
//
// EL DEFECTO QUE ESTO MATA. La corrida de cierre de DC-002.159m murio, una vez de dos, con
// `Unhandled exception` en `colorToColorM`: el PR caia justo despues del `jsr @r1` que despacha
// `color.Color.RGBA()` (PC en `.rodata`, o sea un salto por la tabla de metodos de una interfaz que ya no era la
// que se empujo). El estado de la superficie guardaba la INTERFAZ (`color.Color`, dos palabras: itab + dato) y la
// despachaba en el momento del dibujo, que puede ser miles de instrucciones y varias recolecciones despues.
//
// LA CURA. `PushColor` le pregunta el color a la interfaz UNA sola vez -- con la interfaz recien armada, que es el
// unico momento en que el que llama la tiene en la mano -- y guarda los CUATRO CANALES en `surfaceState`. El
// dibujo (`fillDrawImageOptions`, `prepareColorM`, `applyHSV`) usa esos canales y no vuelve a despachar nada: el
// `jsr @r1` que mataba deja de existir en el camino del dibujo. La matriz sale IDENTICA porque la cuenta es la
// misma (`colorMDeCanales159o` es `colorToColorM` escrita otra vez, a proposito: si compartieran el codigo, la
// comparacion de la prueba no probaria nada).
//
// LA PALANCA. `UsarColorAlEmpujar159o(false)` devuelve el camino viejo (guardar la interfaz y despacharla al
// dibujar). Es el MUTADOR: la prueba del host envenena el color del estado con una interfaz cuyo `RGBA()` revienta
// y con el arreglo prendido el dibujo no se entera; con el arreglo apagado, revienta. En la consola la palanca la
// mueve el arranque del Gate 0 (`console/dc/poc/gate0-pueblo/palancas_159o.go`).
//
// LA SEGUNDA PALANCA (instrumento). `ForzarColorDelTinte159o(c)` hace que un `PushColor(nil)` empuje `c` en vez de
// nada. En la corrida del pueblo el estado llega al dibujo SIN color ~11.677 veces y CON color una sola vez (medido
// en DC-002.159n), asi que el camino que murio no se ejercia. Con el color NEUTRO (`color.RGBA{255,255,255,255}`) la
// matriz que sale es la identidad, o sea que el cuadro no cambia ni un pixel, y el camino se ejerce miles de veces.

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// canalesDelColor159o: los cuatro canales de un color, tal cual los devuelve `color.Color.RGBA()` ([0,0xffff]),
// resueltos UNA vez. `listo` distingue "no hay color" de "hay color negro transparente", que en este camino no
// son lo mismo: sin color el dibujo no toca `ColorM`; con `color.RGBA{A: 0}` la matriz es toda cero.
type canalesDelColor159o struct {
	r, g, b, a uint32
	listo      bool

	// indice: los ocho bits de cada canal tal como los lee el camino de INDICES (`RenderIndicesTintado`, la cache
	// del mapa). No es lo mismo que los canales de arriba: ahi un `color.RGBA`/`color.NRGBA` se usa TAL CUAL (el
	// alfa 160 de las sombras se lee derecho) y cualquier otra implementacion se desmultiplica. Esa regla tambien
	// se resuelve al empujar (DC-002.159o): el mutador de la consola mostro que ESTE es el camino que pisa el
	// pueblo, y desatendido mataba el SH-4 igual que `colorToColorM`.
	indice color.RGBA
}

// resolverCanales159o le pregunta el color a la interfaz. ES LA UNICA VEZ QUE SE DESPACHA `RGBA()` en el camino
// del arreglo.
func resolverCanales159o(c color.Color) canalesDelColor159o {
	if c == nil {
		return canalesDelColor159o{}
	}

	r, g, b, a := c.RGBA()

	return canalesDelColor159o{r: r, g: g, b: b, a: a, listo: true, indice: canalesDeIndices159o(c)}
}

// canalesDeIndices159o: el color en RGBA de 8 bits con la MISMA regla que `comoRGBA` (DC-002.101, la que copio
// del backend PVR): un `color.RGBA` o un `color.NRGBA` se usan tal cual -- el alfa 160 de la sombra del `.dt1` se
// lee derecho, sin desmultiplicar -- y cualquier otra implementacion de `color.Color` se desmultiplica, porque su
// `RGBA()` devuelve el color con el alfa ya aplicado.
//
// Vive en este archivo (y no en `indices_consola.go`) para que la pruebe el host: `indices_consola.go` lleva
// `//go:build dreamcast` y el `go test` de la PC no lo ve.
func canalesDeIndices159o(c color.Color) color.RGBA {
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

// colorDeIndicesDelDibujo159o: el color de 8 bits del camino de indices. Con el arreglo prendido son los canales
// resueltos al empujar; con el arreglo apagado se le pregunta a la interfaz (el camino viejo).
func (s *ebitenSurface) colorDeIndicesDelDibujo159o() (color.RGBA, bool) {
	if colorAlEmpujar159o {
		c := s.stateCurrent.canales

		return c.indice, c.listo
	}

	if s.stateCurrent.color == nil {
		return color.RGBA{}, false
	}

	return canalesDeIndices159o(s.stateCurrent.color), true
}

// colorAlEmpujar159o: el arreglo. true = los canales se resuelven en `PushColor` y el dibujo no despacha la
// interfaz (lo de esta tarjeta). false = el camino viejo (guardar la interfaz y preguntarle `RGBA()` al dibujar),
// que es el mutador de la prueba.
//
//nolint:gochecknoglobals // palanca del instrumento: la mueve el arranque de la consola y la prueba del host
var colorAlEmpujar159o = true

// UsarColorAlEmpujar159o prende (true) o apaga (false) el arreglo. Apagado queda el camino de DC-002.159m, que es
// el que hay que poder volver a pisar para saber que la prueba caza su regresion.
func UsarColorAlEmpujar159o(si bool) { colorAlEmpujar159o = si }

// ColorAlEmpujar159o: si el arreglo esta prendido.
func ColorAlEmpujar159o() bool { return colorAlEmpujar159o }

// colorForzadoDelTinte159o: si no es nil, todo `PushColor(nil)` empuja ESTE color. Es el instrumento que fuerza el
// camino del tinte en la corrida del pueblo (ver la cabecera). nil = apagado, el camino de siempre.
//
//nolint:gochecknoglobals // palanca del instrumento
var colorForzadoDelTinte159o color.Color

// ForzarColorDelTinte159o fuerza (o apaga, con nil) el color que se empuja cuando el que dibuja no empuja ninguno.
func ForzarColorDelTinte159o(c color.Color) { colorForzadoDelTinte159o = c }

// ColorForzadoDelTinte159o: el color forzado (nil = apagado).
func ColorForzadoDelTinte159o() color.Color { return colorForzadoDelTinte159o }

// venenoDelColor159o: EL MUTADOR DEL ARREGLO, EN LA CONSOLA. Con esto prendido, `PushColor` guarda el color y
// despues ESCRIBE ENCIMA de las dos palabras de la interfaz del estado, que es lo que el recolector le dejaba a la
// superficie en DC-002.159m (el volcado del cuadro 1.245: una direccion del monton en el lugar de la tabla de
// metodos). El dibujo que despacha `color.Color.RGBA()` (camino viejo) muere en `colorToColorM`; el que usa los
// canales resueltos al empujar no la toca y sigue.
//
// Sirve para lo que la tarjeta pide: con el arreglo apagado la corrida vuelve a morir donde murio, y con el arreglo
// prendido la misma corrida pasa. Apagado (nil) no hace nada: el camino de la corrida de cierre.
//
//nolint:gochecknoglobals // palanca del instrumento
var venenoDelColor159o bool

// tablaDeMetodosPodrida159o: las dos palabras del volcado de DC-002.159m (`PC 8c3eaa28`, `R1 8c3eaa1c`): el
// despacho de la interfaz salta a una direccion que NO es codigo (en 159m caia en `.rodata`). No se usa una
// direccion del monton a proposito: un puntero falso dentro del semiespacio confunde al recolector y la corrida
// se va por otro lado (medido: el primer intento con 0x8c7c4940 termino en un ciclo de `Out of memory` en vez de
// en la excepcion del SH-4).
const tablaDeMetodosPodrida159o = 0x8c3eaa28

// EnvenenarElColor159o prende (o apaga) el mutador del color podrido.
func EnvenenarElColor159o(si bool) { venenoDelColor159o = si }

// VenenoDelColor159o: si el mutador esta prendido.
func VenenoDelColor159o() bool { return venenoDelColor159o }

// colorMDeCanales159o convierte un color YA RESUELTO en la matriz de dibujo. Es la misma cuenta que
// `colorToColorM` -- mismos desplazamientos, misma cache por superficie, mismo caso de alfa cero -- pero sin
// despachar ninguna interfaz. Las dos versiones existen a proposito: la prueba del host las compara elemento por
// elemento sobre un juego de colores.
func (s *ebitenSurface) colorMDeCanales159o(c canalesDelColor159o) ebiten.ColorM {
	// RGBA() esta en [0 - 0xffff]. Se ajusta a [0 - 0xff], igual que el camino viejo.
	cr, cg, cb, ca := c.r>>8, c.g>>8, c.b>>8, c.a>>8

	if ca == 0 {
		emptyColorM := ebiten.ColorM{}
		emptyColorM.Scale(0, 0, 0, 0)

		return emptyColorM
	}

	// nolint:gomnd // byte values
	key := colorMCacheKey(cr | (cg << 8) | (cb << 16) | (ca << 24))

	if e, ok := s.colorMCache[key]; ok {
		e.atime = s.now()

		return e.colorMatrix
	}

	if len(s.colorMCache) > cacheLimit {
		oldest := int64(math.MaxInt64)
		oldestKey := colorMCacheKey(0)

		for k, e := range s.colorMCache {
			if e.atime < oldest {
				oldestKey = k
				oldest = e.atime
			}
		}

		delete(s.colorMCache, oldestKey)
	}

	cm := ebiten.ColorM{}
	rf := float64(cr) / float64(ca)
	gf := float64(cg) / float64(ca)
	bf := float64(cb) / float64(ca)
	af := float64(ca) / maxAlpha
	cm.Scale(rf, gf, bf, af)

	s.colorMCache[key] = &colorMCacheEntry{colorMatrix: cm, atime: s.now()}

	return s.colorMCache[key].colorMatrix
}

// canalesDelColorDelDibujo159o: los canales del color que el estado tiene ahora, para el dibujo. Con el arreglo
// prendido son los que se resolvieron al empujar; con el arreglo apagado se le pregunta a la interfaz (el camino
// viejo, el que despacha `RGBA()` al dibujar).
func (s *ebitenSurface) canalesDelColorDelDibujo159o() (r, g, b, a uint32, hay bool) {
	if colorAlEmpujar159o {
		c := s.stateCurrent.canales

		return c.r, c.g, c.b, c.a, c.listo
	}

	if s.stateCurrent.color == nil {
		return 0, 0, 0, 0, false
	}

	r, g, b, a = s.stateCurrent.color.RGBA()

	return r, g, b, a, true
}
