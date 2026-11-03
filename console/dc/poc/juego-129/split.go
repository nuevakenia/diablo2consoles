//go:build dreamcast

// DC-002.116 -- EL SPLIT VERTICAL PARA DOS: DOS CAMARAS Y DOS LIENZOS DE 320x480 EN EL MISMO CUADRO.
//
// POR QUE EXISTE. DC-002.115 dejo el cooperativo en UNA pantalla de 640x480 con el leash: los dos jugadores
// entran mientras esten a menos de 5,25 celdas (la peor de las 8 direcciones de separacion, MEDIDA con la
// proyeccion del motor). Para separaciones mayores no hay leash que alcance -- 3 celdas en la diagonal de celdas
// ya son ~340 px de los 640 del lienzo -- y lo que hace el juego original es partir la pantalla. Esto es eso.
//
// QUE HACE, Y EN QUE ORDEN (el orden es la mitad del trabajo):
//
//  1. DOS LIENZOS DE 320x480, uno por jugador, con sus pixeles en la VRAM del PVR (`pvr.VramReservar`). Juntos
//     son 1.228.800 B, EXACTAMENTE los mismos que el lienzo de 640x480 de DC-002.115: el split no pide VRAM de
//     mas, la reparte distinto. El lienzo de 640x480 no se reserva (`UsarSuperficieDelCuadro`), asi que la VRAM
//     que queda para la cache de tiles es la misma de la corrida de referencia.
//  2. EL RENDERER DEL MAPA SE PONE EN 320x480 (`FijarTamanoDelCuadro`): su viewport, su rango de tiles y su
//     `WorldToScreenF` pasan a ser los de UNA mitad. La camara no cambia de forma: `viewport.go` corre la camara
//     media pantalla (160, 240), asi que con la camara en la celda de un jugador ese jugador cae en el CENTRO de
//     SU lienzo, en el ancla (160,283) -- el mismo 283 del cuadro de 640x480, que es lo que se autocomprueba.
//  3. CUADRO A CUADRO, DOS PASADAS: se pone la camara del jugador 1 y se dibuja el mapa en el lienzo 1; se pone
//     la del 2 y se dibuja en el lienzo 2. Cada lienzo mide 320x480, asi que el recorte de la costura NO es una
//     cuenta aparte: lo que cae fuera del lienzo lo descartan las mismas funciones de dibujado
//     (`RenderIndicesTintado` y `copiarEn` miran los bordes del destino). Por eso no puede haber un pixel de una
//     mitad escrito en la otra.
//  4. LA PRESENTACION son dos conversiones RGBA -> RGB565 de 320x480, cada una a su columna de la textura de la
//     pantalla (0 y 320) y un solo `PintarPantalla`. Los pixeles convertidos son los mismos 307.200 de siempre.
//
// LO QUE NO ENTRA (y por eso no esta): el leash NO se aplica con el split (es la alternativa cuando el leash no
// alcanza: cada uno se va con su camara), no hay 4 jugadores en split (DC-002.117) y no se toca el camino del
// mapa ni el del jugador (`personaje.go` y `jugador.go` siguen siendo los del medidor del host, byte a byte).
//
// LAS PALANCAS: `split` en knobs.go ("si"/"no") es el A/B de ms por cuadro con 1 lienzo (el de DC-002.115) y con
// DOS; `celdaP2X`/`celdaP2Y` son donde arranca el jugador 2, y es lo que pone la separacion de mas de 20 celdas
// del criterio.
package main

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

const (
	// lienzosDelSplit: dos, uno por jugador. Cuatro es DC-002.117.
	lienzosDelSplit = 2

	// mitadDelSplit: el ancho de cada lienzo, la mitad del de la pantalla (320 de 640).
	mitadDelSplit = ancho / 2

	// celdasDelSplit: lo que pide la tarjeta -- la captura con los dos separados MAS de 20 celdas. Es el numero
	// contra el que se compara la separacion de cada cuadro, no una distancia que el POC imponga: los jugadores
	// arrancan donde los pone `celdaP2X`/`celdaP2Y`.
	celdasDelSplit = 20.0
)

var (
	// rendererDelCiclo: el Renderer del PVR que corre el ciclo (main.go lo arma). Se guarda aca porque es a quien
	// se le pone la superficie propia y el presentador del split.
	rendererDelCiclo *pvr.Renderer

	// lienzos y sus bloques de memoria: los dos de 320x480.
	lienzos       [lienzosDelSplit]*pvr.Surface
	bloquesLienzo [lienzosDelSplit][]byte

	// camarasDelSplit: la posicion ORTOGONAL de la camara de cada jugador (la que consume `MoveCameraTo`).
	camarasDelSplit [lienzosDelSplit][2]float64

	// splitActivo: si esta tarjeta corrio (lo decide la palanca `split` al arrancar, mirando que haya dos
	// jugadores y que los dos lienzos se hayan podido pedir).
	splitActivo bool

	// de donde salieron los pixeles de los lienzos ("vram" o "monton") y cuanto ocupan: el numero de la tarjeta.
	origenDeLosLienzos = "vram"
	bytesDeUnLienzo    = 0
	bytesDeLosLienzos  = 0

	// lo del ultimo cuadro dibujado, por mitad
	crcDeLaMitad           [lienzosDelSplit]uint32
	crcConJugadorDeLaMitad [lienzosDelSplit]uint32
	llenosDeLaMitad        [lienzosDelSplit]int

	// lo que duro el cuadro anterior, de punta a punta (dibujar + presentar + la espera del PVR).
	msCuadroSplitAnterior int64
	msDelCuadroDelSplit   float64
	msPresentarDelCuadro  int
	primeroDelSplit       bool

	// los acumulados del resumen
	msMapaDelSplit      int
	msJugadorDelSplit   int
	msPresentarDelSplit int
	cuadrosDelSplit     int

	// la separacion, cuadro por cuadro (el criterio de la tarjeta)
	separacionMaxima    float64
	separacionMinima    = 1.0e9
	cuadrosSepMasDe20   int
	cuadrosVigilados    int
	cuadrosFueraDeMitad int

	// el cierre: el crc de cada mitad en el cuadro del origen, para la prueba de repetibilidad
	crcOrigenDeLaMitad   [lienzosDelSplit]uint32
	crcConJugadorOrigen  [lienzosDelSplit]uint32
	crcCuadro1Mitad      [lienzosDelSplit]uint32
	crcCuadro1ConJugador [lienzosDelSplit]uint32
)

// estaElSplitActivo: la pregunta que hace main.go (el split manda en la camara y apaga el leash).
func estaElSplitActivo() bool { return splitActivo }

// prepararElSplit: los dos lienzos en la VRAM, el renderer en 320x480 y el presentador propio del ciclo.
//
// DEVUELVE false (y lo DICE) si falta algo: con un solo jugador no hay split que hacer, y si la VRAM no da para
// los dos bloques los lienzos van al monton de Go -- que es peor pero se mide igual, y el log dice cual de los
// dos caminos tomo.
func prepararElSplit() bool {
	if splitKnob != "si" {
		println("JUEGO121: SPLIT apagado (palanca split=", splitKnob, "): el cuadro es el de DC-002.115")
		return false
	}

	// DC-002.117: el split es de DOS y nada mas. Con tres y cuatro el lienzo no da (cuatro mitades de 160x480 no
	// son el cooperativo de nadie) y esta tarjeta mide esos casos en UNA pantalla con el leash de grupo.
	if len(jugadores) != lienzosDelSplit {
		println("JUEGO121: SPLIT no corre con jugadores=", len(jugadores),
			": el split es de 2 (uno por lienzo); el de 3 y 4 es otra tarjeta y aca se miden en UNA pantalla")
		return false
	}

	if rendererDelMapa == nil || rendererDelCiclo == nil {
		println("JUEGO121: SPLIT no corre sin renderer")
		return false
	}

	bytesDeUnLienzo = mitadDelSplit * alto * 4

	vivo("split-antes-de-los-lienzos")

	for i := 0; i < lienzosDelSplit; i++ {
		bloque := pvr.VramReservar(bytesDeUnLienzo)
		if bloque == nil {
			println("JUEGO121: SPLIT la VRAM no dio para el lienzo", i, "(", bytesDeUnLienzo, "B): va al monton")
			origenDeLosLienzos = "monton"

			lienzos[i] = pvr.NuevaSurface(mitadDelSplit, alto)
		} else {
			bloquesLienzo[i] = bloque
			lienzos[i] = pvr.SuperficieEnBloque(bloque, mitadDelSplit, alto)
		}

		if lienzos[i] == nil {
			println("JUEGO121: SPLIT no se pudo armar el lienzo", i)
			return false
		}

		// el lienzo arranca NEGRO TRANSPARENTE, como el de la pantalla de DC-002.106: el alfa no llega a la
		// pantalla y el cero es lo que hace comparables los crcs contra un cuadro de un solo jugador.
		lienzos[i].ClearRGBA(0, 0, 0, 0)

		bytesDeLosLienzos += bytesDeUnLienzo

		// DC-002.131: el monton de Go despues de CADA lienzo. Es la medida que descarta (o confirma) la hipotesis
		// de la tarjeta: si los pixeles estan en la VRAM, este numero NO se mueve.
		vivo("split-despues-del-lienzo-" + itoa(i))
	}

	// EL RENDERER, EN UNA MITAD: su viewport, su rango de tiles y su WorldToScreenF pasan a ser los de 320x480.
	rendererDelMapa.FijarTamanoDelCuadro(mitadDelSplit, alto)

	espacio := rendererDelMapa.TamanoDelCuadro()

	// EL CICLO PRESENTA LOS DOS LIENZOS, NO EL DE LA PANTALLA: el lienzo de 640x480 no se reserva.
	rendererDelCiclo.UsarSuperficieDelCuadro(lienzos[0])
	rendererDelCiclo.UsarPresentador(presentarElSplit)

	splitActivo = true

	println("JUEGO121: SPLIT LIENZOS n=", lienzosDelSplit, "tamano=", mitadDelSplit, alto,
		"bytesPorLienzo=", bytesDeUnLienzo, "bytesTotales=", bytesDeLosLienzos, "origen=", origenDeLosLienzos,
		"rendererCuadro=", espacio.W, espacio.H, "elLienzoDe640x480NoSeReserva=", true)

	vram("split-despues-de-los-lienzos")
	ponerLasCamarasDelSplit()

	println("JUEGO121: SPLIT CAMARAS camaraP1=", camarasDelSplit[0][0], camarasDelSplit[0][1],
		"camaraP2=", camarasDelSplit[1][0], camarasDelSplit[1][1],
		"centroDeCadaLienzo=", mitadDelSplit/2, alto/2, "celdaP1=", celdaX, celdaY,
		"celdaP2=", celdaX+celdaP2X, celdaY+celdaP2Y)

	return true
}

// ponerLasCamarasDelSplit: la camara de cada jugador, en ortogonal, calculada con el MISMO `WorldToOrtho` del
// motor con el que se dibuja. No hay cuenta nueva: es la proyeccion del viewport.
func ponerLasCamarasDelSplit() {
	for i := 0; i < len(jugadores) && i < lienzosDelSplit; i++ {
		cx, cy := jugadores[i].Mov.Est.Celda()

		rx, ry := rendererDelMapa.WorldToOrtho(cx, cy)
		camarasDelSplit[i][0], camarasDelSplit[i][1] = rx, ry
	}
}

// ponerLaCameraEnLaCelda: la camara del motor en una celda del mapa, con la proyeccion del renderer.
func ponerLaCameraEnLaCelda(i int, cx, cy float64) {
	rx, ry := rendererDelMapa.WorldToOrtho(cx, cy)
	camarasDelSplit[i][0], camarasDelSplit[i][1] = rx, ry

	ponerLaCamara(i)
}

// ponerLaCamara: la camara del motor en la posicion del jugador `i`. Es lo unico que se toca entre una mitad y
// la otra: el viewport, la cache de tiles y el mapa son los mismos (lo que es por jugador es la camara).
func ponerLaCamara(i int) {
	if i < 0 || i >= lienzosDelSplit {
		return
	}

	pos := d2vector.NewPosition(camarasDelSplit[i][0], camarasDelSplit[i][1])

	rendererDelMapa.MoveCameraTo(&pos)
}

// presentarElSplit: la pantalla del cuadro partido. Cada lienzo a SU columna de la textura y un solo quad.
//
// ES EL PRESENTADOR QUE EL CICLO LLAMA EN LUGAR DE `PresentarEnPantalla` (ver `pvr.Renderer.UsarPresentador`):
// lo que se sube son dos lienzos de 320x480, no uno de 640x480, y la cuenta de pixeles es la misma.
func presentarElSplit() error {
	t0 := d2cMs()

	for i := 0; i < lienzosDelSplit; i++ {
		if lienzos[i] == nil {
			continue
		}

		if err := pvr.PresentarEnColumna(lienzos[i].Imagen(), i*mitadDelSplit); err != nil {
			println("JUEGO121: SPLIT no se pudo presentar el lienzo", i, ":", err.Error())
			return err
		}
	}

	pvr.PintarPantalla()

	msPresentarDelCuadro = int(d2cMs() - t0)
	msPresentarDelSplit += msPresentarDelCuadro

	return nil
}

// dibujarElSplit: EL CUADRO DEL SPLIT. Dos camaras, dos lienzos, y el mapa de cada jugador dibujado en el suyo.
//
// LOS CUADROS DEL CIERRE SON LOS MISMOS DE DC-002.113/.115 (origen, repetibilidad y vuelta al final), pero
// medidos POR MITAD: el crc de cada lienzo en el cuadro del origen tiene que ser el mismo en el siguiente.
func dibujarElSplit(superficie d2interface.Surface) error {
	cuadro++

	if cuadro > cuadros+3 {
		return nil
	}

	// DC-002.131: LA RECOLECCION DEL BORDE. Va ACA, antes de tocar los lienzos: es el unico momento del cuadro en
	// que la pila no tiene buffers a medio armar, y por eso es la recoleccion que NO copia basura. Solo corre si el
	// semiespacio esta apretado (ver gc_del_cuadro_131.go).
	gcDelBordeDelCuadro()

	// el cuadro de punta a punta: lo que duro el anterior (dibujar + presentar + la espera del PVR)
	ahora := int64(d2cMs())
	if primeroDelSplit {
		msDelCuadroDelSplit = float64(ahora-msCuadroSplitAnterior) / 1000.0
	} else {
		primeroDelSplit = true
	}

	msCuadroSplitAnterior = ahora

	switch cuadro {
	case cuadros + 1:
		volverAlOrigen()
	case cuadros + 3:
		volverAlFinal()
	}

	if cuadro == 1 {
		println("JUEGO121: SPLIT PRIMER-CUADRO cuadro=1 lienzos=", lienzosDelSplit, "tamano=", mitadDelSplit, alto)
	}

	msMapaDelCuadro := 0
	msJugadorDelCuadro := 0

	for i := 0; i < lienzosDelSplit; i++ {
		msMapaUno, msJugadorUno := dibujarUnaMitad(i)

		msMapaDelCuadro += msMapaUno
		msJugadorDelCuadro += msJugadorUno
	}

	msMapaDelSplit += msMapaDelCuadro
	msJugadorDelSplit += msJugadorDelCuadro
	cuadrosDelSplit++

	// LA SEPARACION DE ESTE CUADRO (el criterio de la tarjeta) y la vigilancia de cada mitad.
	leerLaSeparacionDelSplit(cuadro)

	// EL CUADRO COMPLETO: el crc de los dos lienzos leidos en orden (izquierda y despues derecha).
	crcCompleto := crcDeLosDosLienzos()

	println("JUEGO121: SPLIT CUADRO", cuadro, "lienzos=", lienzosDelSplit, "crc32=", int(crcCompleto),
		"pixelesNoVacios=", llenosDeLosDosLienzos(), "de", ancho*alto, "msMapa=", msMapaDelCuadro,
		"msJugadores=", msJugadorDelCuadro, "msPresentar=", msPresentarDelCuadro,
		"msCuadro=", msDelCuadroDelSplit, "celdas=", separacionDelSplit(), "jugadores=", len(jugadores))

	// EL CIERRE: el origen se guarda y el cuadro siguiente tiene que dar lo mismo (por mitad).
	switch cuadro {
	case cuadros + 1:
		for i := 0; i < lienzosDelSplit; i++ {
			crcOrigenDeLaMitad[i] = crcDeLaMitad[i]
			crcConJugadorOrigen[i] = crcConJugadorDeLaMitad[i]
		}

		println("JUEGO121: SPLIT ORIGEN cuadro=", cuadro, "crcP1=", int(crcDeLaMitad[0]),
			"crcP2=", int(crcDeLaMitad[1]))
	case cuadros + 2:
		iguales := crcDeLaMitad[0] == crcOrigenDeLaMitad[0] && crcDeLaMitad[1] == crcOrigenDeLaMitad[1]
		igualesConJugador := crcConJugadorDeLaMitad[0] == crcConJugadorOrigen[0] &&
			crcConJugadorDeLaMitad[1] == crcConJugadorOrigen[1]

		println("JUEGO121: SPLIT REPETIBILIDAD cuadro=", cuadro, "crcP1=", int(crcDeLaMitad[0]),
			"crcP1DelCuadroAnterior=", int(crcOrigenDeLaMitad[0]), "crcP2=", int(crcDeLaMitad[1]),
			"crcP2DelCuadroAnterior=", int(crcOrigenDeLaMitad[1]), "igualesPorMitad=", iguales,
			"igualesConJugador=", igualesConJugador, "crcP1DelCuadro1=", int(crcCuadro1Mitad[0]),
			"igualAlCuadro1P1=", crcDeLaMitad[0] == crcCuadro1Mitad[0],
			"crcP2DelCuadro1=", int(crcCuadro1Mitad[1]),
			"igualAlCuadro1P2=", crcDeLaMitad[1] == crcCuadro1Mitad[1])
	case cuadros + 3:
		resumenDelSplit()
		resumenDelHUD() // DC-002.125: el HUD de cada mitad, con sus valores
	}

	return nil
}

// dibujarUnaMitad: el mapa y el jugador de UNA mitad, en SU lienzo. Devuelve los ms del mapa y los del jugador.
func dibujarUnaMitad(i int) (int, int) {
	p := jugadores[i]
	anchoLienzo := mitadDelSplit

	ponerLaCamara(i)

	vivoDeUnPaso(i, "antes-del-mapa")

	t0 := d2cMs()
	rendererDelMapa.Render(lienzos[i])
	msMapa := int(d2cMs() - t0)

	vivoDeUnPaso(i, "despues-del-mapa")

	pixeles := lienzos[i].Imagen().Pixeles()

	llenos := contarLlenos(pixeles)
	crcMapa := crc32.ChecksumIEEE(pixeles)

	vivoDeUnPaso(i, "despues-del-crc")

	crcDeLaMitad[i] = crcMapa
	llenosDeLaMitad[i] = llenos

	if cuadro == 1 {
		crcCuadro1Mitad[i] = crcMapa
	}

	// el jugador de ESTA mitad, con la camara de ESTA mitad (el ancla sale del `WorldToScreenF` que acaba de
	// usar el mapa: son la misma cuenta).
	msJugador := 0
	crcConJugador := crcMapa
	pintado := false

	cu, listo := prepararElCuadroDeUnaMitad(p, pixeles, anchoLienzo)
	if jugadorOK && listo {
		t1 := d2cMs()
		PintarJugador(lienzos[i], cu, p.AnclaX, p.AnclaY, p.Pal[:])
		msJugador = int(d2cMs() - t1)

		p.MsDibujo = msJugador
		p.Cambiados = pixelesCambiadosEnLaMitad(pixeles, p, anchoLienzo)

		crcConJugador = crc32.ChecksumIEEE(pixeles)
		pintado = true

		restaurarCajaEnLaMitad(p, pixeles, anchoLienzo)

		if crc32.ChecksumIEEE(pixeles) != crcMapa {
			println("JUEGO121: SPLIT MITAD", i, "MAPA-RESTAURADO DISTINTO cuadro=", cuadro,
				"crc32=", int(crc32.ChecksumIEEE(pixeles)), "mapaSolo=", int(crcMapa))
		}
	}

	crcConJugadorDeLaMitad[i] = crcConJugador

	if cuadro == 1 {
		crcCuadro1ConJugador[i] = crcConJugador
	}

	// DC-002.126: LOS MONSTRUOS DE ESTA MITAD. Va DESPUES del jugador (el mismo orden que en la pantalla de uno) y
	// con la camara de ESTA mitad todavia puesta. Antes de esta tarjeta el camino del split NO dibujaba monstruos:
	// con dos o mas jugadores el bicho -- vivo o cadaver -- era invisible, y de ahi salio el "0 pixeles" con el que
	// DC-002.122 creyo que el `.cof` del DD venia vacio.
	msBicho, pxBicho := medirLosBichosEnLaMitad(i, pixeles)

	cx, cy := p.Mov.Est.Celda()
	camX, camY := camarasDeLaMitadEnCeldas(i)

	mitad := "IZQUIERDA"
	if i == 1 {
		mitad = "DERECHA"
	}

	println("JUEGO121: SPLIT MITAD cuadro=", cuadro, "mitad=", mitad, "jugador=", p.Nombre, "lienzo=",
		mitadDelSplit, alto, "celda=", cx, cy, "camara=", camX, camY, "ancla=", p.AnclaX, p.AnclaY,
		"caja=", p.Caja.X0, p.Caja.Y0, p.Caja.X1, p.Caja.Y1, "tamano=", p.Caja.Ancho(), "x", p.Caja.Alto(),
		"cajaDentro=", siNo(p.Dentro), "pixelesNoVacios=", llenos, "de", mitadDelSplit*alto,
		"crcMapa=", int(crcMapa), "crcConJugador=", int(crcConJugador), "pixelesDelJugador=", p.Cambiados,
		"msMapa=", msMapa, "msJugador=", msJugador, "cuadroDelCiclo=", p.CuadroDelCiclo,
		"pixelesDelBicho=", pxBicho, "msBicho=", msBicho)

	// el cuadro que queda en pantalla: se vuelve a pintar el jugador sobre el mapa restaurado, y encima los bichos
	if pintado {
		PintarJugador(lienzos[i], cu, p.AnclaX, p.AnclaY, p.Pal[:])
	}

	// DC-002.126: y los monstruos, que tambien se quedan en pantalla (es lo que deja ver el cadaver en el split).
	pintarLosBichosEnLaMitad(i)

	// DC-002.125: EL HUD DE ESTA MITAD. Cada mitad lleva el SUYO, con SUS valores (la vida y el mana del jugador
	// de ESTA mitad, sacados de `charstats.txt` con su nivel): es el criterio de la tarjeta en el split. Va ultimo,
	// encima del mapa, del jugador y de los bichos, y DESPUES de que los crc32 de la mitad ya se midieron, asi que
	// no ensucia ninguna comparacion contra el host.
	if hudKnob == "si" && elHUD != nil {
		// DC-002.131: antes y despues del HUD de la mitad, en los primeros cuadros, para saber si el HUD suma
		// bytes al monton de Go (la tarjeta supone que no: su arte va al monton de C).
		if cuadro <= 2 {
			vivo("split-cuadro-" + itoa(cuadro) + "-mitad-" + itoa(i) + "-antes-del-hud")
		}

		pxDelHUDDeLaMitad[i], msDelHUDDeLaMitad[i] = elHUD.dibujar(lienzos[i], i, mitadDelSplit, alto)

		if cuadro <= 2 {
			vivo("split-cuadro-" + itoa(cuadro) + "-mitad-" + itoa(i) + "-despues-del-hud")
		}

		println("JUEGO125: HUD MITAD cuadro=", cuadro, "mitad=", i, "jugador=", p.Nombre,
			"pixeles=", pxDelHUDDeLaMitad[i], "ms=", msDelHUDDeLaMitad[i],
			"lienzo=", mitadDelSplit, "x", alto)
	}

	// EL INVENTARIO DE ESTA MITAD (DC-002.124): cada mitad pinta el suyo, con su grilla y su objeto, dentro de su
	// lienzo. El costo se mide igual que en el modo de un jugador, con `d2cMs` alrededor del panel.
	if inventarioKnob == "si" && pintado {
		msPanel := dibujarElInventarioEnLaMitad(i, lienzos[i])
		if msPanel > 0 {
			msDelPanelDeLaMitad[i] = msPanel
		}
	}

	return msMapa, msJugador
}

// msDelHUDDeLaMitad / pxDelHUDDeLaMitad: lo que costo y lo que pinto el HUD de cada mitad en el ultimo cuadro.
var msDelHUDDeLaMitad [lienzosDelSplit + 1]int

var pxDelHUDDeLaMitad [lienzosDelSplit + 1]int

// msDelPanelDeLaMitad: lo que costo pintar el inventario de cada mitad en el ultimo cuadro.
var msDelPanelDeLaMitad [lienzosDelSplit + 1]int

// camarasDeLaMitadEnCeldas: la posicion ortogonal de la camara, de vuelta a CELDAS (la inversa de
// `WorldToOrtho`: orthoX = (x-y)*80, orthoY = (x+y)*40), para que el log diga en que celda quedo cada camara.
func camarasDeLaMitadEnCeldas(i int) (float64, float64) {
	ox, oy := camarasDelSplit[i][0], camarasDelSplit[i][1]

	x := oy/80.0 + ox/160.0
	y := oy/80.0 - ox/160.0

	return x, y
}

// leerLaSeparacionDelSplit: la separacion en CELDAS de este cuadro (el criterio: mas de 20) y la vigilancia de
// que cada jugador este DENTRO DE SU lienzo de 320x480.
func leerLaSeparacionDelSplit(cuadro int) {
	if len(jugadores) < lienzosDelSplit {
		return
	}

	cuadrosVigilados++

	x1, y1 := jugadores[0].Mov.Est.Celda()
	x2, y2 := jugadores[1].Mov.Est.Celda()

	d := distanciaEnCeldas(x1, y1, x2, y2)

	if d > separacionMaxima {
		separacionMaxima = d
	}

	if d < separacionMinima {
		separacionMinima = d
	}

	if d > celdasDelSplit {
		cuadrosSepMasDe20++
	}

	fuera := 0

	for i := 0; i < lienzosDelSplit; i++ {
		if jugadores[i].Dentro {
			continue
		}

		fuera++

		println("JUEGO121: SPLIT FUERA cuadro=", cuadro, "jugador=", jugadores[i].Nombre,
			"caja=", jugadores[i].CajaCruda.X0, jugadores[i].CajaCruda.Y0, jugadores[i].CajaCruda.X1,
			jugadores[i].CajaCruda.Y1, "lienzo=", mitadDelSplit, alto)
	}

	cuadrosFueraDeMitad += fuera

	if cuadro == 1 || cuadro%10 == 0 || cuadro > cuadros {
		println("JUEGO121: SPLIT SEPARACION cuadro=", cuadro, "jugadores=", len(jugadores), "celdas=", d,
			"masDe20=", siNo(d > celdasDelSplit), "p1=", x1, y1, "p2=", x2, y2,
			"maxCeldas=", separacionMaxima, "cuadrosMasDe20=", cuadrosSepMasDe20, "cuadrosFuera=", fuera)
	}
}

// separacionDelSplit: la separacion del cuadro que se acaba de dibujar (para la linea del cuadro).
func separacionDelSplit() float64 {
	if len(jugadores) < lienzosDelSplit {
		return 0
	}

	x1, y1 := jugadores[0].Mov.Est.Celda()
	x2, y2 := jugadores[1].Mov.Est.Celda()

	return distanciaEnCeldas(x1, y1, x2, y2)
}

// prepararElCuadroDeUnaMitad: lo mismo que `prepararElCuadroDeUnJugador` (coop.go), pero con el ancho del LIENZO
// en la mano: el ancla, la caja, si entra y la copia de lo que hay debajo se miden contra 320x480, no 640x480.
//
// LA CUENTA DEL ANCLA ES LA MISMA DEL MOTOR (`AnclaDelJugador` con el `WorldToScreenF` del renderer, que en el
// split es el de 320x480): no hay una formula nueva por estar en una mitad.
func prepararElCuadroDeUnaMitad(p *participante, pixeles []byte, anchoLienzo int) (*CuadroDelJugador, bool) {
	ciclo := cicloDelJugador(p.Mov.Direccion, p.Mov.Caminando)
	if ciclo == nil {
		return nil, false
	}

	cu := p.Mov.CuadroDeLaAnimacion(ciclo)
	if cu == nil {
		return nil, false
	}

	p.CuadroDelCiclo = indiceDelCuadro(ciclo, cu)

	cx, cy := p.Mov.Est.Celda()

	p.AnclaX, p.AnclaY = AnclaDelJugador(rendererDelMapa.WorldToScreenF, cx, cy)

	// la caja SIN recortar es el numero del criterio (que el jugador entre en SU lienzo)
	p.CajaCruda = CajaDe(p.AnclaX, p.AnclaY, cu)
	p.Dentro = cajaDentroDelLienzoDe(p.CajaCruda, anchoLienzo, alto)

	p.Caja = recortarAlLienzo(p.CajaCruda, anchoLienzo, alto)
	if p.Caja.Ancho() <= 0 || p.Caja.Alto() <= 0 {
		return nil, false
	}

	guardarCajaEnLaMitad(p, pixeles, anchoLienzo)

	return cu, true
}

// cajaDentroDelLienzoDe: la caja entera adentro del lienzo de la mitad (los bordes incluidos: X1 e Y1 son
// exclusivos).
func cajaDentroDelLienzoDe(c CajaDelJugador, anchoLienzo, altoLienzo int) bool {
	return c.X0 >= 0 && c.Y0 >= 0 && c.X1 <= anchoLienzo && c.Y1 <= altoLienzo
}

// recortarAlLienzo: la caja adentro de la mitad (escribir afuera de un lienzo es corromper memoria ajena).
func recortarAlLienzo(c CajaDelJugador, anchoLienzo, altoLienzo int) CajaDelJugador {
	if c.X0 < 0 {
		c.X0 = 0
	}

	if c.Y0 < 0 {
		c.Y0 = 0
	}

	if c.X1 > anchoLienzo {
		c.X1 = anchoLienzo
	}

	if c.Y1 > altoLienzo {
		c.Y1 = altoLienzo
	}

	if c.X1 < c.X0 {
		c.X1 = c.X0
	}

	if c.Y1 < c.Y0 {
		c.Y1 = c.Y0
	}

	return c
}

// guardarCajaEnLaMitad: copia lo que hay en el lienzo de la mitad debajo de la caja del jugador. Es
// `guardarCajaDe` (coop.go) con el paso de fila del lienzo: 320, no 640.
func guardarCajaEnLaMitad(p *participante, pixeles []byte, anchoLienzo int) {
	necesario := p.Caja.Ancho() * p.Caja.Alto() * 4

	if p.CajaGuardada == nil || len(p.CajaGuardada) < necesario {
		p.CajaGuardada = make([]byte, necesario)
	}

	n := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*anchoLienzo + x) * 4
			copy(p.CajaGuardada[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}
}

// restaurarCajaEnLaMitad: devuelve la caja del jugador a lo que habia debajo (el mapa de SU mitad).
func restaurarCajaEnLaMitad(p *participante, pixeles []byte, anchoLienzo int) {
	n := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*anchoLienzo + x) * 4
			copy(pixeles[j:j+4], p.CajaGuardada[n:n+4])

			n += 4
		}
	}
}

// pixelesCambiadosEnLaMitad: cuantos pixeles de la caja del jugador NO son los del mapa (los que pinto de verdad).
func pixelesCambiadosEnLaMitad(pixeles []byte, p *participante, anchoLienzo int) int {
	n := 0
	cambiados := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*anchoLienzo + x) * 4

			if pixeles[j] != p.CajaGuardada[n] || pixeles[j+1] != p.CajaGuardada[n+1] ||
				pixeles[j+2] != p.CajaGuardada[n+2] || pixeles[j+3] != p.CajaGuardada[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// crcDeLosDosLienzos: el crc32 de los dos lienzos LEIDOS EN ORDEN (izquierda y despues derecha). Es el numero
// estable del cuadro partido: no es el de un lienzo de 640x480 (esos pixeles no existen en el split), y se
// calcula sin pedir un buffer de 640x480 solo para hashear.
func crcDeLosDosLienzos() uint32 {
	d := crc32.NewIEEE()

	for i := 0; i < lienzosDelSplit; i++ {
		if lienzos[i] == nil {
			continue
		}

		_, _ = d.Write(lienzos[i].Imagen().Pixeles())
	}

	return d.Sum32()
}

// llenosDeLosDosLienzos: los pixeles no negros de los dos lienzos (el "cuadro" del split son los dos).
func llenosDeLosDosLienzos() int { return llenosDeLaMitad[0] + llenosDeLaMitad[1] }

// resumenDelSplit: los numeros del criterio de esta tarjeta, en cuatro lineas.
func resumenDelSplit() {
	memoria("split-fin")
	semiespacio("split-fin")
	resumenDelGcDeCuadro()

	cache := rendererDelMapa.CensoDelTramo()

	println("JUEGO121: SPLIT RESUMEN lienzos=", lienzosDelSplit, "tamano=", mitadDelSplit, alto,
		"bytesPorLienzo=", bytesDeUnLienzo, "bytesDeLienzos=", bytesDeLosLienzos, "origen=", origenDeLosLienzos,
		"cuadrosPintados=", cuadrosDelSplit, "msMapaTotal=", msMapaDelSplit, "msJugadorTotal=", msJugadorDelSplit,
		"msPresentarTotal=", msPresentarDelSplit, "msMapaPorCuadro=", porCuadro(msMapaDelSplit, cuadrosDelSplit),
		"msJugadorPorCuadro=", porCuadro(msJugadorDelSplit, cuadrosDelSplit),
		"msPresentarPorCuadro=", porCuadro(msPresentarDelSplit, cuadrosDelSplit))

	println("JUEGO121: SPLIT CRITERIO separacionMinCeldas=", separacionMinima, "separacionMaxCeldas=",
		separacionMaxima, "masDe20=", siNo(separacionMaxima > celdasDelSplit), "cuadrosMasDe20=",
		cuadrosSepMasDe20, "cuadrosVigilados=", cuadrosVigilados, "cuadrosFueraDeSuMitad=", cuadrosFueraDeMitad,
		"lienzo=", mitadDelSplit, alto, "dosMitades=", true)

	println("JUEGO121: SPLIT MEMORIA vramTotal=", pvr.VramTotal(), "vramDisponible=", pvr.VramDisponible(),
		"vramPedidaPorEstePoc=", pvr.FotoDeLaVram().Pedida, "lienzoDeLaPantalla=", pvr.VramDelLienzo(),
		"cacheDelTramoBytes=", cache.Bytes, "cacheUsado=", cache.Usado, "cacheImagenes=", cache.Imagenes,
		"cacheDerrames=", cache.Derrames, "montonLibre=", d2cMontonLibre(),
		"semiespacioUsado=", int(d2cGcUsado()), "semiespacioTotal=", int(d2cGcTotal()),
		"colecciones=", int(d2cGcColecciones()))

	println("JUEGO121: SPLIT FIN crcP1=", int(crcDeLaMitad[0]), "crcP2=", int(crcDeLaMitad[1]),
		"crcCuadro1P1=", int(crcCuadro1Mitad[0]), "crcCuadro1P2=", int(crcCuadro1Mitad[1]),
		"crcConJugadorP1=", int(crcConJugadorDeLaMitad[0]), "crcConJugadorP2=", int(crcConJugadorDeLaMitad[1]))
}

// porCuadro: el promedio de un acumulado sobre los cuadros pintados (sin fmt: la consola no lo trae).
func porCuadro(total, n int) float64 {
	if n <= 0 {
		return 0
	}

	return float64(total) / float64(n)
}

// ============================== LOS MONSTRUOS EN LA MITAD (DC-002.126) ==============================
//
// Hasta DC-002.124 el camino del split NO dibujaba monstruos: con dos, tres o cuatro jugadores el bicho era
// invisible, vivo o muerto. Esa es la causa medida del "0 pixeles pintados" con el que DC-002.122 concluyo que el
// `.cof` del DD venia vacio (el compuesto del DD da 819 pixeles y crc32 1212887229; lo que daba cero era el
// dibujo). Aca se pinta en cada mitad con la camara de esa mitad, con el MISMO pincel y la misma cuenta de caja que
// en la pantalla de uno: se guarda lo de abajo, se pinta, se miden los pixeles, se devuelve el mapa y se comprueba
// que el crc32 de la mitad volvio al del mapa solo.

// ponerElBichoEnLaMitad: el cuadro, el ancla y la caja del bicho con la camara de la mitad que este puesta, y la
// copia de lo que hay debajo de su caja en ESE lienzo.
func ponerElBichoEnLaMitad(b *bicho, pixeles []byte) (*CuadroDelJugador, bool) {
	if b == nil || b.Est == nil || !elCadaverSeDibuja(b) {
		b.Pintados = 0

		return nil, false
	}

	cu := b.cuadroDelBicho()
	if cu == nil {
		return nil, false
	}

	cx, cy := b.Est.Celda()

	// la MISMA proyeccion que el jugador (AnclaDelJugador: WorldToScreenF + RenderOffset del motor)
	b.AnclaX, b.AnclaY = AnclaDelJugador(rendererDelMapa.WorldToScreenF, cx, cy)

	b.Ancho, b.Alto = cu.Ancho, cu.Alto
	b.CajaCruda = CajaDe(b.AnclaX, b.AnclaY, cu)
	b.Caja = recortarAlLienzo(b.CajaCruda, mitadDelSplit, alto)

	if b.Caja.Ancho() <= 0 || b.Caja.Alto() <= 0 {
		b.Pintados = 0 // se fue del lienzo de esta mitad: este cuadro no pinta nada

		return nil, false
	}

	guardarCajaDelBichoEnLaMitad(b, pixeles)

	return cu, true
}

// guardarCajaDelBichoEnLaMitad: copia lo que hay en el lienzo de la mitad dentro de la caja del bicho.
func guardarCajaDelBichoEnLaMitad(b *bicho, pixeles []byte) {
	necesario := b.Caja.Ancho() * b.Caja.Alto() * 4

	if b.CajaGuardada == nil || len(b.CajaGuardada) < necesario {
		b.CajaGuardada = make([]byte, necesario)
	}

	n := 0

	for y := b.Caja.Y0; y < b.Caja.Y1; y++ {
		for x := b.Caja.X0; x < b.Caja.X1; x++ {
			j := (y*mitadDelSplit + x) * 4
			copy(b.CajaGuardada[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}
}

// restaurarCajaDelBichoEnLaMitad: devuelve la caja del bicho a lo que habia debajo (el mapa de la mitad).
func restaurarCajaDelBichoEnLaMitad(b *bicho, pixeles []byte) {
	n := 0

	for y := b.Caja.Y0; y < b.Caja.Y1; y++ {
		for x := b.Caja.X0; x < b.Caja.X1; x++ {
			j := (y*mitadDelSplit + x) * 4
			copy(pixeles[j:j+4], b.CajaGuardada[n:n+4])

			n += 4
		}
	}
}

// pixelesCambiadosDelBichoEnLaMitad: cuantos pixeles de la caja del bicho NO son los del mapa.
func pixelesCambiadosDelBichoEnLaMitad(b *bicho, pixeles []byte) int {
	n := 0
	cambiados := 0

	for y := b.Caja.Y0; y < b.Caja.Y1; y++ {
		for x := b.Caja.X0; x < b.Caja.X1; x++ {
			j := (y*mitadDelSplit + x) * 4

			if pixeles[j] != b.CajaGuardada[n] || pixeles[j+1] != b.CajaGuardada[n+1] ||
				pixeles[j+2] != b.CajaGuardada[n+2] || pixeles[j+3] != b.CajaGuardada[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// medirLosBichosEnLaMitad: pinta los bichos en el lienzo de la mitad `i`, mide lo que pinta cada uno, devuelve el
// mapa y comprueba que el crc32 volvio al de antes. Devuelve los ms y los pixeles pintados (sumados).
func medirLosBichosEnLaMitad(i int, pixeles []byte) (int, int) {
	if len(bichos) == 0 || i >= len(lienzos) || lienzos[i] == nil {
		return 0, 0
	}

	crcAntes := crc32.ChecksumIEEE(pixeles)
	msTotal, pintados := 0, 0

	pintadosCon := make([]*CuadroDelJugador, 0, len(bichos))
	pintadosBicho := make([]*bicho, 0, len(bichos))

	// la profundidad: el que tiene menor x+y se dibuja primero (la misma insercion que la pantalla de uno)
	listos := ordenarLosBichosPorProfundidad()

	for _, b := range listos {
		cu, ok := ponerElBichoEnLaMitad(b, pixeles)
		if !ok {
			continue
		}

		t1 := d2cMs()
		PintarJugador(lienzos[i], cu, b.AnclaX, b.AnclaY, palJugador[:])
		b.MsDibujo = int(d2cMs() - t1)

		msTotal += b.MsDibujo
		b.Pintados = pixelesCambiadosDelBichoEnLaMitad(b, pixeles)
		pintados += b.Pintados

		pintadosCon = append(pintadosCon, cu)
		pintadosBicho = append(pintadosBicho, b)
	}

	for k := len(listos) - 1; k >= 0; k-- {
		if listos[k].Caja.Ancho() > 0 && listos[k].Caja.Alto() > 0 {
			restaurarCajaDelBichoEnLaMitad(listos[k], pixeles)
		}
	}

	lienzoRestaurado := crc32.ChecksumIEEE(pixeles) == crcAntes
	if !lienzoRestaurado {
		println("JUEGO126: BICHO MITAD", i, "MAPA-RESTAURADO DISTINTO cuadro=", cuadro,
			"crc32=", int(crc32.ChecksumIEEE(pixeles)), "mapaSolo=", int(crcAntes))
	}

	for k := range pintadosCon {
		anotarElCuadroDeLaMuerte(pintadosBicho[k], pintadosCon[k], pintadosBicho[k].Pintados,
			pintadosBicho[k].MsDibujo, lienzoRestaurado)
	}

	if len(pintadosCon) > 0 {
		println("JUEGO126: BICHO MITAD", i, "cuadro=", cuadro, "bichos=", len(pintadosCon), "pixeles=", pintados,
			"ms=", msTotal, "lienzoRestaurado=", lienzoRestaurado)
	}

	return msTotal, pintados
}

// pintarLosBichosEnLaMitad: el cuadro que queda en pantalla en esa mitad (despues de devolver el mapa). No mide.
func pintarLosBichosEnLaMitad(i int) {
	if len(bichos) == 0 || i >= len(lienzos) || lienzos[i] == nil {
		return
	}

	for _, b := range ordenarLosBichosPorProfundidad() {
		if b.Caja.Ancho() <= 0 || b.Caja.Alto() <= 0 || !elCadaverSeDibuja(b) {
			continue
		}

		cu := b.cuadroDelBicho()
		if cu == nil {
			continue
		}

		PintarJugador(lienzos[i], cu, b.AnclaX, b.AnclaY, palJugador[:])
	}
}

// ordenarLosBichosPorProfundidad: los bichos ordenados por x+y (el de atras primero). Insercion, como en la
// pantalla de uno: la consola no lleva `sort` en este camino.
func ordenarLosBichosPorProfundidad() []*bicho {
	listos := make([]*bicho, 0, len(bichos))

	for _, b := range bichos {
		if b.Est != nil && elCadaverSeDibuja(b) {
			listos = append(listos, b)
		}
	}

	for i := 1; i < len(listos); i++ {
		for j := i; j > 0; j-- {
			xj, yj := listos[j].Est.Celda()
			xp, yp := listos[j-1].Est.Celda()

			if xj+yj >= xp+yp {
				break
			}

			listos[j], listos[j-1] = listos[j-1], listos[j]
		}
	}

	return listos
}
