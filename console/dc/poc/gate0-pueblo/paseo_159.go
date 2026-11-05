//go:build dreamcast

package main

// DC-002.159b -- EL PASEO DEL HEROE: LA CRUCETA APRETADA POR EL PROGRAMA.
//
// POR QUE EXISTE. La tarjeta pide ver al heroe CAMINAR (la celda cambiando en las cuatro direcciones y en las
// diagonales, sin dejar rastro de pixeles). Un worker corre solo y de noche: no hay dedos en la cruceta. Y
// apretar el teclado del host tampoco sirve de nada medido -- se probo con `ydotool` (ver
// `scripts/console/paseo-heroe-159.sh`) y el heroe no se movio ni una celda: Flycast no trae mapeo de teclado a
// la cruceta en su config (no hay seccion `[input]` en `~/.config/flycast/emu.cfg`), asi que los eventos del
// host no llegan a la Dreamcast.
//
// COMO. Se envuelve el MISMO servicio de entrada que usa el heroe (`d2input/maple`) y se le contesta la cruceta
// con un guion. Lo unico que cambia es de donde sale el "esta apretada": el resto del camino es el de verdad
// (`entrada.IsKeyPressed` -> `direccionDelPaso` -> `Player.Advance` -> `d2path`), y el mando real sigue sumando
// (si alguien aprieta, se respeta). El guion son los ocho sentidos de la PANTALLA, que es lo que la tarjeta
// pide: cuatro ejes y cuatro diagonales.
//
// CUANDO NO SE QUIERE. `paseoApagado159` (abajo) lo deja leyendo solo el mando.

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2input/maple"
)

// cuadrosPorTramo159: lo que dura cada sentido. Con ~24 cuadros por segundo medidos en este host son casi dos
// segundos: a la velocidad de caminar del motor son un par de celdas por tramo, suficiente para que la captura
// siguiente tenga al heroe en otro lado y para que el modo de caminar (`TW`) entre y se vea la animacion.
const cuadrosPorTramo159 = 24

// paseoApagado159 deja el paseo quieto (solo mando).
const paseoApagado159 = false

// sentido159: un tramo del guion, en las teclas de la PANTALLA.
type sentido159 struct {
	arriba, abajo, izquierda, derecha bool
	nombre                            string
}

// guion159: cuatro ejes y cuatro diagonales, en el orden en que se recorren.
var guion159 = []sentido159{
	// ida y vuelta por cada eje y cada diagonal: con la camara quieta (el mapa se pinto una sola vez y no se
	// vuelve a pintar) el heroe que camina siempre para el mismo lado se va del lienzo en una celda y media.
	{nombre: "derecha", derecha: true},
	{nombre: "izquierda", izquierda: true},
	{nombre: "abajo", abajo: true},
	{nombre: "arriba", arriba: true},
	{nombre: "abajo-derecha", abajo: true, derecha: true},
	{nombre: "arriba-izquierda", arriba: true, izquierda: true},
	{nombre: "arriba-derecha", arriba: true, derecha: true},
	{nombre: "abajo-izquierda", abajo: true, izquierda: true},
}

// paseo159: el servicio de entrada con el guion encima.
type paseo159 struct {
	*maple.Servicio

	cuadro int // cuadros desde que arranco: lo mueve `ActualizarDesdeElControl`, el mismo que refresca el mando
}

func nuevoPaseo159(base *maple.Servicio) *paseo159 {
	return &paseo159{Servicio: base}
}

// ActualizarDesdeElControl refresca el mando (lo que hacia el servicio pelado) y cuenta el cuadro del guion.
func (p *paseo159) ActualizarDesdeElControl() {
	p.Servicio.ActualizarDesdeElControl()
	p.cuadro++

	// DC-002.159n: EL LATIDO DE LA SONDA DEL COLOR. Sin esta linea, una corrida con cero eventos de la sonda
	// no distingue "no paso nada" de "la sonda no corrio": los numeros son la prueba de que el camino se
	// ejercio (llamadas > 0, conColor > 0) y de cuantas veces cada clase se llevo el color por delante.
	if p.cuadro%300 == 0 {
		LatidoDeLaSonda159n(p.cuadro)
	}
}

// sentidoDeAhora: el tramo del guion que toca. Despues del ultimo, quieto (asi la corrida termina con el heroe
// parado en el ultimo lugar y la captura lo muestra sin que se siga yendo).
func (p *paseo159) sentidoDeAhora() (sentido159, bool) {
	if paseoApagado159 || p.cuadro < cuadrosDeEspera159 {
		return sentido159{}, false
	}

	tramo := (p.cuadro - cuadrosDeEspera159) / cuadrosPorTramo159
	if tramo >= len(guion159)*vueltas159 {
		return sentido159{}, false
	}

	return guion159[tramo%len(guion159)], true
}

// cuadrosDeEspera159: los cuadros que se dejan pasar antes de la primera direccion. El cuadro 1 es el que
// compone el pueblo y crea al heroe (~4 s medidos): apretar antes no rompe nada, pero el paseo arrancaria
// escondido en la carga.
const cuadrosDeEspera159 = 30

// vueltas159: cuantas veces se recorre el guion. Una vuelta son 8 tramos de 24 cuadros = 192 cuadros de caminar,
// y el criterio de DC-002.159d pide medir 200 cuadros caminando: con dos vueltas el heroe camina 384 cuadros y
// vuelve al punto de partida (el guion es ida y vuelta por cada eje, asi que repetirlo no lo saca del lienzo).
const vueltas159 = 2

// IsKeyPressed: la cruceta del guion, o la del mando si el guion ya termino.
func (p *paseo159) IsKeyPressed(tecla d2enum.Key) bool {
	if p.Servicio.IsKeyPressed(tecla) {
		return true
	}

	sentido, hay := p.sentidoDeAhora()
	if !hay {
		return false
	}

	switch tecla {
	case d2enum.KeyUp:
		return sentido.arriba
	case d2enum.KeyDown:
		return sentido.abajo
	case d2enum.KeyLeft:
		return sentido.izquierda
	case d2enum.KeyRight:
		return sentido.derecha
	}

	return false
}

// Tramo159: en que tramo del guion va, para el log de la corrida (lo imprime la pantalla junto al estado del
// heroe: asi la captura y la linea se pueden cruzar sin adivinar).
func (p *paseo159) Tramo159() (nombre string, cuadro int) {
	sentido, hay := p.sentidoDeAhora()
	if !hay {
		return "quieto", p.cuadro
	}

	return sentido.nombre, p.cuadro
}
