//go:build dreamcast

package main

import "github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"

// DC-002.134 -- EL CUADRO FUERA DEL MAPA: EL CRONOMETRO POR ETAPA.
//
// QUE CONTESTA. En DC-002.132 el cuadro quedo desglosado en tres: esperar + trabajar + presentar. El
// trabajo (1.113 ms) se partio en mapa (367 ms) y "el resto" (746 ms), y de ese resto no se sabia NADA:
// el POC dibuja al jugador, a los bichos, el HUD y el minimapa sobre el lienzo, pero ademas IMPRIME entre
// 1 y 2 KB de lineas de evidencia por cuadro y hace varios crc32 del lienzo entero. Esta tarjeta existe
// para no adivinar cual de esas cosas pesa: cada etapa del dibujado tiene su cronometro y cada 30 cuadros
// sale UNA linea con el promedio de todas.
//
// LA LINEA (promedio por cuadro, en microsegundos):
//
//	JUEGO134: ETAPAS cuadros= delTramo= actualizar= mapa= jugadores= bichos= items= npcs= inventario=
//	          hud= minimapa= dialogo= medir= sinEtapa= dibujar= cuadroTotal=
//
//	actualizar -- lo que el POC hace por cuadro ademas de dibujar (control, movimiento, camara, prints)
//	mapa       -- rendererDelMapa.Render (la pieza que DC-002.132 bajo a 367 ms)
//	jugadores  -- dibujarAJugadores + volverAPintarAJugadores (se pinta DOS veces: medir y presentar)
//	bichos     -- dibujarLosBichos + volverAPintarLosBichos (idem)
//	hud        -- elHUD.dibujar (globos, cinturon, zona)
//	minimapa   -- elMinimapa.dibujarElMinimapa (las celdas del modelo y el marcador)
//	medir      -- los crc32 del lienzo (1.228.800 B cada uno) y el conteo de pixeles: se acumula DENTRO de
//	              `crcLienzo`/`crc32De`/`llenosLienzo`, asi que esta INCLUIDO en las etapas de arriba
//	              (el mapa y el jugador pagan su propio crc): sirve para saber cuanto del cuadro es medirse
//	sinEtapa   -- la diferencia entre `dibujar` y la suma de las etapas: las lineas de evidencia (println)
//	              y el resto del camino que no tiene cronometro propio. Si este numero es grande, el que
//	              pesa es el instrumento impreso, no el dibujado.
//	dibujar    -- el `dibujar` entero del POC
//	cuadroTotal-- dibujar + actualizar (lo que el PVR llama "trabajo")
//
// LA PALANCA (knobs.go):
//
//	etapas      = "si" | "no"  -- "no" no suma nada (el cronometro no toca el cuadro). Con "si" el costo
//	                              es el de dos llamadas a d2c_us por etapa, o sea nada al lado de 746 ms.
//	verbosoEtapas = "si" | "no"-- "no" apaga las lineas de evidencia POR CUADRO del POC (las de
//	                              comparacion, que solo hacen falta en la corrida de la evidencia) y deja
//	                              las de los tres primeros cuadros y las de los resumenes del cierre. Es
//	                              el A/B que dice cuanto del cuadro es el puerto serie.

// d2cUs: microsegundos desde el arranque (csrc/shim.c, `timer_us_gettime64`). Es el mismo reloj que usa la
// presentacion del PVR, asi que los numeros de las dos puntas se pueden restar.
//
//extern d2c_us
func d2cUs() uint

// Las etapas del cuadro. El orden es el orden en que el POC las paga dentro del cuadro.
const (
	e134Mapa = iota
	e134Jugadores
	e134Bichos
	e134Items
	e134NPCs
	e134Inventario
	e134HUD
	e134Minimapa
	e134Dialogo
	e134Actualizar
	e134Instrumento
	e134Cuenta
)

var nombres134 = [e134Cuenta]string{
	"mapa", "jugadores", "bichos", "items", "npcs", "inventario", "hud", "minimapa", "dialogo", "actualizar",
	"medir",
}

var (
	usEtapas134     [e134Cuenta]int64 // microsegundos acumulados por etapa del DIBUJADO en el tramo
	usActualizar134 int64             // microsegundos acumulados de `actualizar` en el tramo
	usDibujar134    int64             // el `dibujar` entero del tramo
	cuadrosEtapas   int

	// pixeles134: los pixeles que el backend PVR escribio por camino en el tramo, contados por el mapa (el
	// contador se reinicia justo antes de `rendererDelMapa.Render` y se lee justo despues). Contesta lo que los
	// milisegundos solos no dicen: DE LOS 384 ms DEL MAPA, cuantos pixeles van por el camino de indices (los
	// pisos) y cuantos por el de copiar superficies (las paredes de arriba y los techos).
	pixeles134 [3]uint64
)

// anotarLosPixeles134: los pixeles del cuadro del mapa por camino (indices, copiar, sombras).
func anotarLosPixeles134(indices, copiar, sombras uint64) {
	pixeles134[0] += indices
	pixeles134[1] += copiar
	pixeles134[2] += sombras
}

// cronometro134: el arranque de una etapa (microsegundos).
func cronometro134() int64 { return int64(d2cUs()) }

// anotarEtapa134: cierra una etapa abierta con `cronometro134`.
func anotarEtapa134(etapa int, desde int64) {
	if etapasKnob != "si" || desde <= 0 {
		return
	}

	us := int64(d2cUs()) - desde
	if us < 0 { // el contador de 32 bits da la vuelta cada ~71 minutos: no se resta un negativo
		return
	}

	usEtapas134[etapa] += us
}

// anotarActualizar134: lo que costo `actualizar` en este cuadro (lo llama `actualizar` al salir). Va por
// fuera de la tabla de etapas porque se cierra ANTES de que empiece `dibujar`: si fuera una etapa mas, la
// resta de `sinEtapa` lo contaria dos veces.
func anotarActualizar134(us int64) {
	if etapasKnob != "si" || us < 0 {
		return
	}

	usActualizar134 += us
}

// anotarCuadro134: el total del cuadro (lo llama `dibujar` al final) y, cada `cuadrosDelInforme132`
// cuadros, el informe de las etapas.
func anotarCuadro134(usDibujar int64) {
	if etapasKnob != "si" || usDibujar < 0 {
		return
	}

	usDibujar134 += usDibujar
	cuadrosEtapas++

	if cuadro%cuadrosDelInforme132 == 0 {
		informeEtapas134()
	}
}

// informeEtapas134: la linea con el promedio por cuadro de cada etapa.
func informeEtapas134() {
	if cuadrosEtapas == 0 {
		return
	}

	n := int64(cuadrosEtapas)
	un := uint64(cuadrosEtapas)

	suma := int64(0)
	for _, us := range usEtapas134 {
		suma += us
	}

	// `sinEtapa` es lo que del `dibujar` no tiene cronometro propio: las lineas de evidencia (println), los
	// resumenes y el camino de siempre. Es el numero que dice si el que pesa es el puerto serie.
	//
	// OJO CON LA CUENTA: `medir` (los crc32 y el conteo de pixeles) se cronometra ADENTRO de las etapas, asi que
	// ya esta sumado en `mapa`, `jugadores` y `bichos`. Por eso se descuenta de la suma antes de restar: si no,
	// el cuadro "no explicado" sale negativo, que es lo que paso en la primera corrida de esta tarjeta.
	sumaDeEtapas := suma - usEtapas134[e134Instrumento]
	if sumaDeEtapas < 0 {
		sumaDeEtapas = 0
	}

	sinEtapa := usDibujar134 - sumaDeEtapas

	println("JUEGO134: ETAPAS cuadros=", cuadro, "delTramo=", cuadrosEtapas,
		"actualizar=", usActualizar134/n, "mapa=", usEtapas134[e134Mapa]/n,
		"jugadores=", usEtapas134[e134Jugadores]/n, "bichos=", usEtapas134[e134Bichos]/n,
		"items=", usEtapas134[e134Items]/n, "npcs=", usEtapas134[e134NPCs]/n,
		"inventario=", usEtapas134[e134Inventario]/n,
		"hud=", usEtapas134[e134HUD]/n, "minimapa=", usEtapas134[e134Minimapa]/n,
		"dialogo=", usEtapas134[e134Dialogo]/n,
		"medir=", usEtapas134[e134Instrumento]/n,
		"sinEtapa=", sinEtapa/n, "dibujar=", usDibujar134/n,
		"cuadroTotal=", (usDibujar134+usActualizar134)/n,
		"etapas=", etapasKnob, "verboso=", verbosoEtapas)
	println("JUEGO134: PIXELES cuadros=", cuadro, "delTramo=", cuadrosEtapas,
		"porCuadroIndices=", pixeles134[0]/un, "porCuadroCopiar=", pixeles134[1]/un,
		"porCuadroSombras=", pixeles134[2]/un,
		"blitPalabras=", blitPalabras134, "avisosDeAlineacion=", pvr.AvisosDeAlineacion())

	for i := 0; i < int(e134Cuenta); i++ {
		usEtapas134[i] = 0
	}

	pixeles134[0], pixeles134[1], pixeles134[2] = 0, 0, 0
	usActualizar134, usDibujar134, cuadrosEtapas = 0, 0, 0
}

// verboso134: si este cuadro lleva las lineas de evidencia por cuadro (las de comparacion contra el host
// y las de cada pieza). Los TRES primeros cuadros las llevan SIEMPRE: ahi vive el crc32 del mundo contra
// el host (1437022778), que es la prueba de que el dibujo no cambio.
func verboso134() bool {
	return verbosoEtapas == "si" || cuadro <= 3
}

// activarElBlitDePalabras: enciende o apaga el camino de 32 bits por pixel del backend PVR (DC-002.134).
func activarElBlitDePalabras() {
	pvr.FijarBlitPalabras(blitPalabras134 == "si")
}

// midiendoElCuadro134: si en ESTE cuadro corre el instrumento por cuadro (los crc32 del lienzo, el conteo de
// pixeles, la caja guardada y devuelta, el segundo pintado del sprite).
//
// ES LA MISMA REGLA QUE `crcLienzo`/`llenosLienzo`: con `medirCuadro=no` el instrumento corre solo en los tres
// primeros cuadros, que es donde vive la comparacion del mundo contra el host (crc32 1437022778) y los pixeles
// del jugador y del bicho. De ahi en adelante no hay nada que medir y si algo que pagar: la caja del sprite
// (unos 100 KB de VRAM copiados dos veces por cuadro) y el segundo pintado.
//
// OJO CON LA REGLA DE LA CASA: apagar el instrumento NO cambia el dibujo. El sprite se pinta en el mismo orden
// y en el mismo sitio; lo que se va es el guardar/medir/devolver que existia para el numero, no para el cuadro.
func midiendoElCuadro134() bool {
	return medirCuadro == "si" || cuadro <= 3 || pintarUnaVez != "si" || cajas139 == "si"
}
