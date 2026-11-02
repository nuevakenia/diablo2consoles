//go:build dreamcast

package main

// DC-002.100 -- LA ARENA DEL DECODIFICADOR EN EL POC DEL PERSONAJE: los buffers de trabajo del .dcc salen del
// monton de C y los devuelve el que los pidio.
//
// POR QUE EXISTE. `d2dcc.PedirDeArena` (DC-002.98) es un gancho opt-in: si no es nil, el decodificador le pide
// al que llama los buffers grandes (los pixeles de cada cuadro, el lienzo de la direccion, las celdas y la
// tabla de celdas) en vez de pedirlos con `make`. En la consola eso es la diferencia entre entrar y no entrar:
// el semiespacio del recolector son 2 MB y una capa de la amazona en una direccion necesita ~1,4 MB de celdas
// mas los pixeles de los cuadros (medido en DC-002.98 sobre `aas2litnuhth.dcc`: 36,2 MB pedidos en 1.738
// objetos para 737 KB de pixeles).
//
// EL CONTRATO (el mismo del paquete, ver d2dcc/arena.go) y lo que hay que respetar aca:
//   - `pedir(bytes)` devuelve un tramo de AL MENOS `bytes`, alineado a 8 (los `DCCPixelBufferEntry` llevan dos
//     `int`), con la vida de lo decodificado: los `PixelData` de los cuadros APUNTAN adentro y NO se copian.
//   - `liberar()` devuelve todo al monton de C. DESPUES DE ESO LO DECODIFICADO NO SE PUEDE TOCAR: por eso el
//     POC compone los cuadros ANTES de liberar (los pixeles se pegan en el lienzo del `Personaje`).
//   - Los tramos son de 256 KB a proposito, muy por encima del umbral externo de libgodc (64 KB): asi el bloque
//     es `malloc` pelado (`gc_external_alloc`), no vive en el semiespacio y devolverlo con el `free` del shim es
//     lo que corresponde (es lo mismo que hace `runtime.FreeExternal`).
//   - Un tramo de arena NO puede llevar punteros adentro: `DCCCell` y `DCCPixelBufferEntry` no los llevan, y
//     por eso el gancho solo pide esos tres tipos (ver el comentario de d2dcc/arena.go).

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"

// tamanoTramoArena: el tramo con que se le pide memoria al monton de C. Por encima de 64 KB a proposito (ver
// arriba): el bloque tiene que ser externo para que el recolector no lo vea y para poder devolverlo a mano.
const tamanoTramoArena = 256 * 1024

// conArena: instala el gancho del decodificador. Con `sinArena` encendido NO lo instala y el decodificador pide
// con `make` (el semiespacio del recolector): es la BASELINE de la tarjeta, con el mismo codigo y la misma
// corrida, para que la unica diferencia entre las dos medidas sea la arena.
func conArena(a *arenaDCC) {
	if sinArena || a == nil {
		d2dcc.PedirDeArena = nil

		return
	}

	d2dcc.PedirDeArena = a.pedir
}

// arenaDCC es la arena de una decodificacion (una capa, o un ciclo entero). Es la duena de sus tramos.
type arenaDCC struct {
	tramos    [][]byte
	cursor    int
	pedidos   int
	entregado int
	liberados int
}

// pedir: el gancho que se instala en d2dcc.PedirDeArena. Devuelve el tramo pedido (o nil si el pedido no entra
// en un tramo, en cuyo caso el decodificador se degrada solo al monton de Go: no revienta).
func (a *arenaDCC) pedir(bytes int) []byte {
	if bytes <= 0 {
		return nil
	}

	if len(a.tramos) == 0 || a.cursor+bytes > len(a.tramos[len(a.tramos)-1]) {
		nuevo := tamanoTramoArena
		if bytes > nuevo {
			nuevo = bytes
		}

		a.tramos = append(a.tramos, make([]byte, nuevo))
		a.cursor = 0
	}

	t := a.tramos[len(a.tramos)-1]
	b := t[a.cursor : a.cursor+bytes]
	a.cursor += bytes
	a.pedidos++
	a.entregado += bytes

	return b
}

// liberar: devuelve TODOS los tramos al monton de C. Idempotente (llamarla dos veces no libera dos veces).
func (a *arenaDCC) liberar() {
	for _, t := range a.tramos {
		// el tramo siempre paso el umbral (tamanoTramoArena >> 64 KB), pero el corte se deja escrito: es la
		// regla de libgodc, no una casualidad de esta tarjeta
		if len(t) > umbralGrande {
			d2cFreeExternal(uintptrOf(t))
			a.liberados++
		}
	}

	a.tramos = nil
	a.cursor = 0
}

// capacidad: los bytes que la arena tiene pedidos AHORA (para la evidencia).
func (a *arenaDCC) capacidad() int {
	total := 0

	for _, t := range a.tramos {
		total += len(t)
	}

	return total
}
