//go:build dreamcast

package main

// DC-002.84 -- LA SONDA DEL PINTADO: que capa, en que direccion y con que valor de interfaz.
//
// POR QUE. En DC-002.76 el primer cuadro que pinta un `Composite` mata el SH-4 con `PC 3fd17b4f`,
// `PC == PR` (salto a basura) y R3 = 0xac0000e0. La pila no dice cual de las llamadas por interfaz de
// `Composite.Render` salta, asi que se mira desde arriba: `d2asset.SondaDeRender` se llama UNA vez por capa
// justo antes de `layer.RenderFromOrigin`, y aca se imprime la direccion, el cuadro de animacion, el indice
// de capa y las DOS palabras de la interfaz (itab y dato). Con eso el cuadro que muere queda nombrado: si el
// indice es legal y la interfaz tiene un itab raro, el problema es el itab; si el indice se sale, es el COF.
//
// Las dos palabras se leen con `unsafe` a proposito: es un instrumento, no codigo del motor, y la
// representacion de una interfaz no vacia en libgodc es `Iface { itab, data }` (runtime/runtime.h).
//
// SE CALLA sola a los `cuadrosConSonda` cuadros de cada etapa: sin eso son 12 lineas por cuadro.

import (
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

const cuadrosConSonda = 2

// capaUnica: cuando `pintarSoloLaCapa` esta prendido, la PRIMERA capa que la sonda ve en el cuadro 1 de
// cada etapa (la que el COF pone primera en la prioridad) es la unica que se pinta.
var capaUnica d2interface.Animation

func init() {
	d2asset.SondaDeRender = func(direccion, cuadro, direccionCof int, indice d2enum.CompositeType,
		capas []d2interface.Animation, capa d2interface.Animation, sombra bool) {
		if capaUnica == nil && capa != nil {
			capaUnica = capa
		}

		if cuadros > cuadrosConSonda {
			return
		}

		palabras := *(*[2]uintptr)(unsafe.Pointer(&capa))

		println("COOP276: SONDA n=", nEtapa, "cuadro=", cuadros, "dir=", direccion, "dirCof=", direccionCof,
			"capa=", int(indice), "capas=", len(capas), "sombra=", sombra, "nil=", capa == nil,
			"itab=", int(palabras[0]), "dato=", int(palabras[1]))
	}
}
