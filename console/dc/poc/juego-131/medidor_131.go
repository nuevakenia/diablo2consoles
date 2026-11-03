//go:build dreamcast

package main

// DC-002.131 -- EL MEDIDOR DE LO VIVO: donde se van los bytes del split.
//
// POR QUE EXISTE. `semiespacio()` imprime `gc_stats`->`used`, que en libgodc es
// `gc_heap.alloc_ptr - espacio_activo`: lo ASIGNADO desde la ultima recoleccion, no lo que sobrevive. Con eso no se
// puede contestar la pregunta de esta tarjeta ("donde se van los 1,7 MB del split"), porque basura y vivo se suman
// en el mismo numero. Lo que decide si el semiespacio alcanza es lo VIVO, o sea lo que el recolector tiene que
// COPIAR: se mide forzando una recoleccion completa (`runtime.GC()`) y leyendo el mismo `used`, que despues de una
// recoleccion es exactamente lo copiado (`gc-vivo` en gc_copy.c).
//
// EL NUMERO QUE MANDA. El recolector de libgodc es de copia y tira `fatal error: GC to-space overflow` cuando lo
// vivo no entra en el semiespacio de destino (1.835.008 B): lo medido en la consola son vivos de 1.650.000 B que
// revientan, porque a la copia se le suma lo que el barrido conservador de la pila todavia ve apuntado. Por eso el
// presupuesto se mira con margen: lo que se busca es BAJAR EL PICO, no bailar en el borde.

import (
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// vivo: fuerza una recoleccion y dice lo que quedo VIVO, con el monton de C y la VRAM del momento.
//
// OJO CON EL COSTO: una recoleccion completa copia ~1,7 MB por la cache-through del SH-4, asi que esto es un
// instrumento de hitos y de los primeros cuadros, no algo que se llame en cada cuadro de una corrida larga.
func vivo(etapa string) {
	if medirVivo != "si" {
		return
	}

	antes := int(d2cGcColecciones())

	runtime.GC()

	println("JUEGO131: VIVO", etapa, "vivo=", int(d2cGcUsado()), "total=", int(d2cGcTotal()),
		"reasignado=", int(d2cGcUsado())*100/int(d2cGcTotal()), "colecciones=", int(d2cGcColecciones()),
		"nuevas=", int(d2cGcColecciones())-antes,
		"montonUsado=", d2cMontonUsado(), "montonLibre=", d2cMontonLibre(),
		"vramLibre=", pvr.VramDisponible())
}

// picoVivo: lo mas grande que se vio en un `vivo()` de esta corrida (para el cierre).
var picoVivo int

// vivoYAnotar: `vivo()` que ademas lleva el pico, para que el resumen diga el peor momento de la corrida.
func vivoYAnotar(etapa string) {
	vivo(etapa)

	if v := int(d2cGcUsado()); v > picoVivo {
		picoVivo = v
	}
}

// vivoDeUnPaso: el instrumento FINO de esta tarjeta. Un `vivo()` (recoleccion + lo copiado) en cada paso del
// dibujado de una mitad, y SOLO en los cuadros del borde (`cuadroDesdeElBorde`..`cuadroHastaElBorde`), porque una
// recoleccion completa cuesta ~1,7 MB de copia y en una corrida larga seria el cuadro entero.
//
// POR QUE ACOTADO A UNOS CUADROS Y NO A TODOS: lo que se busca es DONDE sube el pico, y el pico tiene cuadro. Con
// esto el log dice, paso por paso del cuadro sospechoso, cuanto quedo vivo y como estaba la cache de tiles.
func vivoDeUnPaso(i int, paso string) {
	if medirVivo != "si" || cuadro < cuadroDesdeElBorde || cuadro > cuadroHastaElBorde {
		return
	}

	vivo("paso-" + itoa(cuadro) + "-mitad-" + itoa(i) + "-" + paso)

	cache := rendererDelMapa.CensoDelTramo()

	println("JUEGO131: PICO PASO cuadro=", cuadro, "mitad=", i, "paso=", paso,
		"vivo=", int(d2cGcUsado()), "montonLibre=", d2cMontonLibre(), "vramLibre=", pvr.VramDisponible(),
		"cacheBytes=", cache.Bytes, "cacheUsado=", cache.Usado, "cacheImagenes=", cache.Imagenes,
		"cacheDerrames=", cache.Derrames)
}

