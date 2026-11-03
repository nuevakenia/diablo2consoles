//go:build dreamcast

package main

// DC-002.131 -- LA RECOLECCION EN EL BORDE DEL CUADRO: el camino que se midio y NO se uso.
//
// ESTA APAGADA POR OMISION (gcPorCuadro = "no"). Corre y salva la corrida, pero cuesta y corrompe el mapa de
// glifos del HUD (seccion "LO QUE COSTO"). Queda en el arbol porque es la unica forma de reproducir la medida,
// y porque el numero que dejo -- cuanto libera cada cuadro -- es el que explica POR QUE moria la corrida.
//
// LO MEDIDO, QUE ES LA RAZON DE ESTE ARCHIVO. Con los 22.848 B del HUD puestos en el split, la corrida completa
// (SPLIT=si JUGADORES=2 FASE=apagado CUADROS=24 HUD=si) moria en el cuadro 14: `GC to-space overflow`, con lo vivo
// en 1.796.448 B de los 1.835.008 del semiespacio. Con el arte del jugador y del bicho ya fuera del semiespacio
// (arte_131.go, 216.000 B) el vivo del arranque del split es 1.394.600 B, o sea que el problema NO era el arranque
// sino el cuadro: entre el cuadro 12 y el 14 el recolector copiaba 291.312 B mas (121.784 + 128.192 + 41.024) en
// tres recolecciones seguidas, todas disparadas por el umbral (95 %) EN MEDIO del dibujado del mapa.
//
// POR QUE EL UMBRAL NO ALCANZA Y UNA RECOLECCION EN EL BORDE SI. El recolector de libgodc es de copia y corta
// cuando lo ASIGNADO llega al umbral, que no distingue basura de vivo: al 95 % quedan ~90 KB de aire y el cuadro
// del split asigna bastante mas que eso entre dos recolecciones (el mapa de una mitad, el HUD, el inventario y la
// otra mitad), asi que la recoleccion termina cayendo adentro de un paso, con los buffers a medio armar todavia
// apuntados desde la pila -- y el barrido conservador los copia como si fueran vivos. Medido: esos 291.312 B
// copiados de mas NO eran estado (con una recoleccion en el borde del cuadro el vivo se queda clavado en 1.497.8xx
// y ya no sube en toda la corrida).
//
// LO QUE COSTO Y POR QUE QUEDA APAGADA (medido en esta tarjeta). Corre y SALVA la corrida (27 cuadros donde
// antes moria en el 14), pero:
//   - cuesta 86-92 ms por cuadro (2.391 ms sobre 27 cuadros) y el cuadro del mapa sube de 701 a 1.136 ms;
//   - y a partir del cuadro 5 el ancho del texto de la zona sale -2147483507 en las DOS mitades (141 - 2^31):
//     el mapa de glifos del HUD queda apuntando a basura despues de una recoleccion en ese punto de la pila.
//     El A/B es limpio (mismo arbol, mismas palancas, solo gcPorCuadro si/no): con "si" el numero se corrompe
//     desde el cuadro 5; con "no", 141 en las tres corridas. La causa raiz no quedo aislada -- queda el
//     sintoma medido y el A/B en console/dc/evidencia/dc-002.131-el-split-con-hud.txt.
// Por eso el arreglo que quedo es el de arte_131.go (sacar 216.000 B de arte del semiespacio), no recolectar
// mas seguido. La palanca se deja para poder volver a medirla, con el default en "no".
//
// LO QUE DEJO MEDIDO, y es lo que vale: lo que liberaba cada cuadro (101-240 KB) confirma que el recolector
// cortaba por umbral EN MEDIO del dibujado y copiaba basura como si fuera viva. Es el camino que el propio
// recolector recomienda cuando tira `GC to-space overflow`: "Call runtime.GC() more often to free garbage
// earlier" (libgodc/runtime/gc_heap.c). Lo que se agrega es CUANDO: en el borde del cuadro, que es el unico
// momento en que la pila no tiene buffers a medio armar.

import "runtime"

// los numeros del instrumento, para el cierre.
var (
	gcDeCuadroVeces     int // cuantos cuadros hicieron la recoleccion del borde
	gcDeCuadroMsTotal   int // lo que costaron, sumado
	gcDeCuadroMsMax     int // la mas cara
	gcDeCuadroLibrado   int // lo que se libero en la primera (la que mas tiene para liberar)
	gcDeCuadroPrimeraMs int
)

// gcDelBordeDelCuadro: la recoleccion del borde, con el umbral y el costo medidos. Devuelve los ms que costo (0 si
// no corrio).
//
// SE LLAMA UNA VEZ POR CUADRO, AL PRINCIPIO, ANTES DE TOCAR LOS LIENZOS. No se llama si la palanca esta apagada ni
// si el semiespacio esta holgado: con el heap al 70 % de vivo el recolector tiene su margen y una copia de 1,5 MB
// por cuadro seria plata tirada.
func gcDelBordeDelCuadro() int {
	if gcPorCuadro != "si" {
		return 0
	}

	total := int(d2cGcTotal())
	if total <= 0 {
		return 0
	}

	antes := int(d2cGcUsado())
	if antes*100/total < umbralDelGcDeCuadro {
		return 0
	}

	t0 := int64(d2cMs())

	runtime.GC()

	ms := int(int64(d2cMs()) - t0)
	despues := int(d2cGcUsado())

	gcDeCuadroVeces++
	gcDeCuadroMsTotal += ms

	if ms > gcDeCuadroMsMax {
		gcDeCuadroMsMax = ms
	}

	if gcDeCuadroVeces == 1 {
		gcDeCuadroPrimeraMs = ms
		gcDeCuadroLibrado = antes - despues
	}

	// el renglon del cuadro: se dice SIEMPRE que corre, con lo que habia, lo que quedo y lo que costo. En una
	// corrida de 27 cuadros son 27 renglones, que es el precio de poder decir el numero.
	println("JUEGO131: GC BORDE cuadro=", cuadro, "antes=", antes, "despues=", despues, "librado=", antes-despues,
		"total=", total, "pct=", antes*100/total, "ms=", ms)

	return ms
}

// resumenDelGcDeCuadro: el cierre de la recoleccion del borde (va en el resumen del split).
func resumenDelGcDeCuadro() {
	println("JUEGO131: GC BORDE RESUMEN palanca=", gcPorCuadro, "umbralPct=", umbralDelGcDeCuadro,
		"cuadros=", gcDeCuadroVeces, "msTotal=", gcDeCuadroMsTotal, "msPrimera=", gcDeCuadroPrimeraMs,
		"msMax=", gcDeCuadroMsMax, "libradoPrimera=", gcDeCuadroLibrado)
}
