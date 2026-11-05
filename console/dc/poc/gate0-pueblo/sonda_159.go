//go:build dreamcast

package main

// DC-002.159b -- LA SONDA DEL COMPUESTO EN EL GATE 0.
//
// POR QUE EXISTE. El primer cuadro que dibuja un `Composite` de personaje en la consola salta a basura
// (`Unhandled exception`, PC dentro de datos, PR adentro de `runtime.mapassign`). Para saber cual de las
// llamadas por interfaz de `Composite.Render` es la que salta hay que ver, en el cuadro que muere, la capa y
// las DOS PALABRAS del valor de interfaz (itab y dato): eso es lo que entrega este gancho (`d2asset.SondaDeRender`,
// DC-002.84), que se llama una vez por capa y justo antes de `layer.RenderFromOrigin`.
//
// Es la misma sonda del POC `coop` (console/dc/poc/coop/sonda.go), sin la parte de la etapa: aca hay un solo
// heroe y un solo compuesto.

import (
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
)

// capasQueSeSondean159: las primeras capas de la corrida. Con dos cuadros alcanza y sobra (el cuadro que
// muere es el 1) y no ensucia el puerto serie, que es el mismo que se muere de callar (DC-002.87b).
const capasQueSeSondean159 = 24

var sondeadas159 int

func init() {
	d2asset.SondaDeRender = func(direccion, cuadro, direccionCof int, indice d2enum.CompositeType,
		capas []d2interface.Animation, capa d2interface.Animation, sombra bool) {
		sondeadas159++

		if sondeadas159 > capasQueSeSondean159 {
			return
		}

		palabras := *(*[2]uintptr)(unsafe.Pointer(&capa))

		println("SONDA159 n=", sondeadas159, "cuadro=", cuadro, "dir=", direccion, "dirCof=", direccionCof,
			"capa=", int(indice), "capas=", len(capas), "sombra=", sombra, "nil=", capa == nil,
			"itab=", int(palabras[0]), "dato=", int(palabras[1]))
	}
}
