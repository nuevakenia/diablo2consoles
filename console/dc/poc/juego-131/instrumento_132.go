//go:build dreamcast

package main

// DC-002.132 -- EL INSTRUMENTO DE LOS CUADROS POR SEGUNDO Y DEL DESGLOSE DEL CUADRO.
//
// QUE CONTESTA. El POC ya media el costo de CADA pieza (mapa, jugadores, bichos, HUD, minimapa) pero no decia
// lo unico que el jugador ve: CUANTOS CUADROS POR SEGUNDO se pintan, ni cuanto de ese cuadro es el mapa y
// cuanto la presentacion. Sin esos dos numeros no se puede elegir que atacar, y ademas el propio instrumento
// (crc32 del lienzo entero 16 veces por cuadro + contarLlenos) es parte del costo: medido en DC-002.131, el
// crc32 de 1.228.800 B por cuadro es de decenas de ms.
//
// LAS DOS PALANCAS QUE AGREGA ESTE ARCHIVO (knobs.go):
//
//	medirCuadro = "si" | "no"  -- "no" apaga el crc32 y el conteo de pixeles POR CUADRO (el corpus del cuadro 1
//	                              se sigue midiendo igual). Es la unica forma de medir el costo de DIBUJAR y
//	                              no el de medirse.
//	blitRapido  = "si" | "no"  -- el camino nuevo de `RenderIndicesTintado`/`RenderIndices` (recorte calculado
//	                              una vez por tile y camino sin tinte). "no" es el A/B: el mismo binario con
//	                              el bucle viejo, que es el que midio 675-931 ms por cuadro.
//
// Y EL RESUMEN `JUEGO132`: cada `cuadrosDelInforme132` cuadros deja en la consola serie el promedio del
// cuadro, los cuadros de animacion QUE SE DIBUJARON de la amazona y del Returned (o sea lo que el ojo ve, no
// lo que el reloj del motor dice) y el desglose de la ultima pasada del mapa.

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// cuadrosDelInforme132: cada cuantos cuadros se deja el resumen. 30 cuadros son medio segundo a 60 cuadros/s
// y 30 s a 1 cuadro/s: el promedio dice el ritmo real sin inundar el puerto serie.
const cuadrosDelInforme132 = 30

// Estado del resumen (se reinicia en cada informe).
var (
	msDelCuadro132       int64 // ms acumulados de los cuadros del tramo
	cuadrosDelTramo      int
	animsHeroe132        int // cuadros de animacion DIBUJADOS de la amazona
	animsBicho132        int // cuadros de animacion DIBUJADOS del Returned
	ultimoAnimHeroe      int
	ultimoAnimBicho      int
	composiciones132     int // cuantas veces se volvio a componer el mapa en el tramo
	msDeLasComposiciones int64
)

// crcLienzo: el crc32 del lienzo, o 0 si el instrumento por cuadro esta apagado.
//
// OJO: con `medirCuadro = "no"` los numeros de las lineas de comparacion contra el host salen 0 y las lineas
// lo dicen (`instrumento=no`). La comparacion que vale -- la del cuadro 1 contra el host -- se mide igual,
// porque el guion del POC la pide antes de este cambio de palanca.
func crcLienzo(pixeles []byte) uint32 {
	if medirCuadro != "si" && cuadro > 3 {
		return 0
	}

	return crc32.ChecksumIEEE(pixeles)
}

// llenosLienzo: los pixeles no vacios del lienzo, o 0 si el instrumento por cuadro esta apagado.
//
// LOS TRES PRIMEROS CUADROS SE MIDEN SIEMPRE, aunque el instrumento este apagado: ahi vive la comparacion
// contra el host (el cuadro 1 es el del corpus, crc32 1437022778), y apagarla por velocidad dejaria la tarjeta
// sin la prueba de que el mundo se sigue dibujando igual.
func llenosLienzo(pixeles []byte) int {
	if medirCuadro != "si" && cuadro > 3 {
		return 0
	}

	return contarLlenos(pixeles)
}

// cuadroAnimDelHeroe132: el cuadro del ciclo de la amazona que esta en pantalla (para contar la fluidez por lo
// DIBUJADO, no por lo que pide el reloj del motor).
func cuadroAnimDelHeroe132() int {
	if len(jugadores) == 0 {
		return 0
	}

	return jugadores[0].CuadroDelCiclo
}

// cuadroAnimDelBicho132: el cuadro del ciclo del Returned que esta en pantalla.
func cuadroAnimDelBicho132() int {
	if len(bichos) == 0 {
		return 0
	}

	return bichos[0].CuadroCiclo
}

// activarElBlitRapido: enciende o apaga el camino nuevo de dibujado de tiles del backend PVR.
func activarElBlitRapido() {
	pvr.FijarBlitRapido(blitRapido == "si")
}

// anotarElCuadro132: un cuadro dibujado, con sus ms y los cuadros de animacion que quedaron en pantalla.
//
// LOS CUADROS DE ANIMACION SE CUENTAN POR LO DIBUJADO, no por lo que pide el reloj: la animacion del motor
// avanza con el reloj (MsAnim), asi que a 1 cuadro por segundo se salta cuadros y el ojo ve saltos; lo que
// se mide aca es cuantos cuadros DISTINTOS llegaron al lienzo en el tramo, que es exactamente la fluidez.
func anotarElCuadro132(ms int64, animHeroe, animBicho int) {
	msDelCuadro132 += ms
	cuadrosDelTramo++

	if animHeroe != ultimoAnimHeroe {
		animsHeroe132++
		ultimoAnimHeroe = animHeroe
	}

	if animBicho != ultimoAnimBicho {
		animsBicho132++
		ultimoAnimBicho = animBicho
	}
}

// anotarLaComposicion132: el mapa se volvio a componer (es la pieza mas cara del cuadro).
func anotarLaComposicion132(ms int64) {
	composiciones132++
	msDeLasComposiciones += ms
}

// resumenDelCuadro132: el informe del tramo (cada 30 cuadros).
func resumenDelCuadro132(cuadrosTotales int) {
	if cuadrosDelTramo == 0 {
		return
	}

	n := int64(cuadrosDelTramo)
	msPorCuadro := msDelCuadro132 / n
	// DC-002.132: los cuadros por segundo van en CENTESIMAS y no en enteros. Medido en la Dreamcast: con el
	// mapa del pueblo a ~600 ms por cuadro, 1000/600 da 1 y 1000/1010 da 0 -- el informe decia "fps= 0" en las
	// dos corridas y no distinguia la buena de la mala. La cuenta es 100.000/ms, que en centesimas es exacto.
	fpsX100 := int64(0)
	if msPorCuadro > 0 {
		fpsX100 = 100000 / msPorCuadro
	}

	msMapaPorCuadro := int64(0)
	if cuadrosDelTramo > 0 {
		msMapaPorCuadro = msDeLasComposiciones / n
	}

	// los cuadros de animacion por segundo: si el tramo duro 0 ms no se divide (una corrida con el reloj
	// detenido no es un numero, es un defecto del instrumento y se informa como 0).
	animsHeroePorSeg, animsBichoPorSeg := int64(0), int64(0)
	if msDelCuadro132 > 0 {
		animsHeroePorSeg = int64(animsHeroe132) * 1000 / msDelCuadro132
		animsBichoPorSeg = int64(animsBicho132) * 1000 / msDelCuadro132
	}

	println("JUEGO132: FPS cuadros=", cuadrosTotales, "delTramo=", cuadrosDelTramo,
		"msPorCuadro=", msPorCuadro, "fpsX100=", int(fpsX100),
		"animsHeroePorSeg=", animsHeroePorSeg, "animsBichoPorSeg=", animsBichoPorSeg,
		"animsHeroe=", animsHeroe132, "animsBicho=", animsBicho132,
		"composiciones=", composiciones132, "msMapaPorCuadro=", msMapaPorCuadro,
		"medirCuadro=", medirCuadro, "blitRapido=", blitRapido)

	msDelCuadro132, cuadrosDelTramo = 0, 0
	animsHeroe132, animsBicho132 = 0, 0
	composiciones132, msDeLasComposiciones = 0, 0
}
