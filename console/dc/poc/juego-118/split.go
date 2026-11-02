//go:build dreamcast

// DC-002.118 -- EL SPLIT DE TRES Y CUATRO: UNA REJILLA DE RECUADROS, UN LIENZO Y UNA CAMARA POR JUGADOR.
//
// POR QUE EXISTE. DC-002.116 dejo el split de DOS (dos lienzos de 320x480, dos camaras, la misma memoria del
// lienzo unico) y DC-002.117 dejo los CUATRO jugadores en UNA pantalla de 640x480 con el leash de grupo. Lo que
// no existia es el split de TRES y CUATRO: `prepararElSplit` rechazaba cualquier numero que no fuera 2 y lo decia.
// Esto es eso, y el reparto se DECIDE MIDIENDO (ver el comentario de `repartirElSplit`).
//
// QUE HACE, Y EN QUE ORDEN (el orden es la mitad del trabajo):
//
//  1. UN LIENZO POR JUGADOR, del tamano de SU recuadro de la pantalla, con sus pixeles en la VRAM del PVR
//     (`pvr.VramReservar`). La suma de los recuadros es SIEMPRE la pantalla entera, asi que la VRAM de los
//     lienzos es SIEMPRE 640*480*4 = 1.228.800 B, con uno, con dos, con tres o con cuatro: el split no pide
//     VRAM de mas, la reparte distinto. El lienzo de 640x480 de la pantalla no se reserva
//     (`UsarSuperficieDelCuadro`), asi que la VRAM que queda para la cache de tiles es la misma de siempre.
//  2. EL RENDERER DEL MAPA SE PONE EN EL TAMANO DEL RECUADRO que se va a dibujar (`FijarTamanoDelCuadro`): su
//     viewport y su `WorldToScreenF` pasan a ser los de ESE recuadro, asi que con la camara en la celda de un
//     jugador ese jugador cae en el centro de SU recuadro. La camara no cambia de forma: es la proyeccion del
//     viewport. Con la rejilla de cuatro hay DOS tamanos (320x240 y, en el reparto de tres, 320x480), y por eso
//     el tamano se pone por recuadro y no una vez al arrancar.
//  3. CUADRO A CUADRO, N PASADAS: se pone la camara del jugador i y se dibuja el mapa en el lienzo i. Cada
//     lienzo mide lo suyo, asi que el recorte de la costura NO es una cuenta aparte: lo que cae fuera del
//     lienzo lo descartan las mismas funciones de dibujado (`RenderIndicesTintado` y `copiarEn` miran los
//     bordes del destino). Por eso no puede haber un pixel de un recuadro escrito en otro.
//  4. LA PRESENTACION son N conversiones RGBA -> RGB565, cada una a SU region de la textura del PVR, y N quads
//     (uno por recuadro). Los pixeles convertidos son los mismos 307.200 de siempre.
//
// LA TEXTURA Y LA PANTALLA NO COINCIDEN, Y ES LO QUE HAY QUE ENTENDER DE ESTA TARJETA. La textura del PVR es
// 1024x512 y sus bloques son de 32x32: un lienzo se sube bloque a bloque, asi que donde EMPIEZA en la textura
// tiene que ser multiplo de 32. La fila de abajo de los cuatro cuadrantes empieza en la 240 de la PANTALLA, que
// no es multiplo de 32: sus pixeles van a la 256 de la TEXTURA y el quad los pinta en la 240. Por eso el
// presentador dice, recuadro por recuadro, de donde lee y donde pinta (`pvr.PintarEnBloque`), en vez de pintar
// la region (0,0)-(640,480) de la textura en toda la pantalla como hacia el split de dos.
//
// LO QUE NO ENTRA (y por eso no esta): el leash NO se aplica con el split (es la alternativa cuando el leash no
// alcanza: cada uno se va con su camara), y no se toca el camino del mapa ni el del jugador (`personaje.go` y
// `jugador.go` siguen siendo los del medidor del host, byte a byte).
//
// LAS PALANCAS: `split` en knobs.go ("si"/"no") es el A/B con la pantalla entera y con la rejilla;
// `repartoDeTres` ("grande" | "tiras") elige el reparto de TRES, que es el unico con dos formas razonables; y
// `celdaP2X`.../`celdaP4X`... son donde arrancan los jugadores, que es lo que pone la separacion del criterio.
package main

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

const (
	// maxLienzosDelSplit: cuatro, que son los cuatro puertos del bus maple y el tope del cooperativo local.
	maxLienzosDelSplit = 4

	// ladoDelBloqueDelPvr: el bloque de la textura del PVR (32x32). Las posiciones de la TEXTURA tienen que ser
	// multiplos de esto; las de la PANTALLA, no.
	ladoDelBloqueDelPvr = 32

	// anchoDeLaTextura / altoDeLaTextura: la potencia de dos que el backend pide para una pantalla de 640x480
	// (los dos lados potencia de dos, como exige el PVR). Estan aca para poder DECIR cuando un reparto no entra
	// en la textura, en vez de escribir afuera.
	anchoDeLaTextura = 1024
	altoDeLaTextura  = 512

	// celdasDelSplit: lo que pide la tarjeta -- la captura con los jugadores separados MAS de 20 celdas. Es el
	// numero contra el que se compara la separacion de cada cuadro, no una distancia que el POC imponga: los
	// jugadores arrancan donde los pone `celdaP2X`... y caminan con el guion.
	celdasDelSplit = 20.0
)

// recuadro: donde va UN lienzo. (X, Y, W, H) es el recuadro de la PANTALLA; (TX, TY) es donde viven sus pixeles
// en la TEXTURA del PVR, que no es el mismo lugar cuando la fila de la pantalla no es multiplo de 32.
type recuadro struct {
	X, Y, W, H int
	TX, TY     int
}

var (
	// rendererDelCiclo: el Renderer del PVR que corre el ciclo (main.go lo arma). Se guarda aca porque es a quien
	// se le pone la superficie propia y el presentador del split.
	rendererDelCiclo *pvr.Renderer

	// los recuadros de este reparto, sus lienzos y sus bloques de memoria
	recuadros     [maxLienzosDelSplit]recuadro
	lienzos       [maxLienzosDelSplit]*pvr.Surface
	bloquesLienzo [maxLienzosDelSplit][]byte

	// lienzosDelSplit: cuantos lienzos se armaron (uno por jugador).
	lienzosDelSplit int

	// camarasDelSplit: la posicion ORTOGONAL de la camara de cada jugador (la que consume `MoveCameraTo`).
	camarasDelSplit [maxLienzosDelSplit][2]float64

	// splitActivo: si esta tarjeta corrio (lo decide la palanca `split` al arrancar).
	splitActivo bool

	// de donde salieron los pixeles de los lienzos ("vram" o "monton") y cuanto ocupan: el numero de la tarjeta.
	origenDeLosLienzos = "vram"
	bytesDeLosLienzos  = 0

	// lo del ultimo cuadro dibujado, por recuadro
	crcDelRecuadro           [maxLienzosDelSplit]uint32
	crcConJugadorDelRecuadro [maxLienzosDelSplit]uint32
	llenosDelRecuadro        [maxLienzosDelSplit]int

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
	separacionMaxima         float64
	separacionMinima         = 1.0e9
	cuadrosSepMasDe20        int
	cuadrosVigilados         int
	cuadrosFueraDeSuRecuadro int

	// el cierre: el crc de cada recuadro en el cuadro del origen, para la prueba de repetibilidad
	crcOrigenDeRecuadro  [maxLienzosDelSplit]uint32
	crcConJugadorOrigen  [maxLienzosDelSplit]uint32
	crcCuadro1Recuadro   [maxLienzosDelSplit]uint32
	crcCuadro1ConJugador [maxLienzosDelSplit]uint32
)

// estaElSplitActivo: la pregunta que hace main.go (el split manda en la camara y apaga el leash).
func estaElSplitActivo() bool { return splitActivo }

// repartirElSplit: EL REPARTO DE LA PANTALLA, que es la decision de esta tarjeta.
//
// LOS TRES REPARTOS, y por que son estos:
//
//	1 jugador   640x480             el cuadro de siempre (es el A/B: con un lienzo del tamano de la pantalla el
//	                                camino tiene que ser el mismo de DC-002.115, y lo es).
//	2 jugadores 2 columnas 320x480   el de DC-002.116, sin tocar una linea: dos lienzos del alto completo.
//	4 jugadores 4 cuadrantes        320x240 cada uno. Es lo natural y lo que el propio juego hace: los cuatro
//	                                recuadros son IGUALES, que es lo unico simetrico para cuatro personas en la
//	                                misma pantalla, y cada uno ve 2x1,5 celdas de las 4x3 del cuadro unico.
//	3 jugadores 1 grande + 2 chicas  (por omision) 320x480 + dos de 320x240. El de tres TIENE dos formas
//	                                razonables y esta es la que se eligio MIDIENDO: la otra (tres tiras de
//	                                640x160) deja a cada uno 4x1 celdas de alto, o sea una franja donde no se
//	                                ve ni el piso de adelante ni el de atras. Las dos cuestan los MISMOS pixeles
//	                                (307.200) y los MISMOS ms de mapa -- el costo es por pixel y no por lienzo --,
//	                                asi que lo que decide es lo que ve cada jugador. La otra queda en la palanca
//	                                `repartoDeTres=tiras` para el A/B.
//
// Y LA VRAM: la suma de los recuadros es la pantalla entera en los cuatro casos, asi que los lienzos son w*h*4 B
// por recuadro y 1.228.800 B en total SIEMPRE. Ese es el numero que hace que el split de cuatro no pida ni un
// byte mas que el cuadro unico.
func repartirElSplit(n int) []recuadro {
	switch n {
	case 1:
		return []recuadro{{X: 0, Y: 0, W: ancho, H: alto}}
	case 2:
		return []recuadro{
			{X: 0, Y: 0, W: ancho / 2, H: alto},
			{X: ancho / 2, Y: 0, W: ancho / 2, H: alto},
		}
	case 3:
		if repartoDeTresKnob == "tiras" {
			return []recuadro{
				{X: 0, Y: 0, W: ancho, H: 160},
				{X: 0, Y: 160, W: ancho, H: 160},
				{X: 0, Y: 320, W: ancho, H: 160},
			}
		}

		return []recuadro{
			{X: 0, Y: 0, W: ancho / 2, H: alto},
			{X: ancho / 2, Y: 0, W: ancho / 2, H: alto / 2},
			{X: ancho / 2, Y: alto / 2, W: ancho / 2, H: alto / 2},
		}
	case 4:
		return []recuadro{
			{X: 0, Y: 0, W: ancho / 2, H: alto / 2},
			{X: ancho / 2, Y: 0, W: ancho / 2, H: alto / 2},
			{X: 0, Y: alto / 2, W: ancho / 2, H: alto / 2},
			{X: ancho / 2, Y: alto / 2, W: ancho / 2, H: alto / 2},
		}
	}

	return nil
}

// ubicarEnLaTextura: la posicion de la TEXTURA de cada recuadro, y la comprobacion de que la rejilla entra y no
// se pisa. La regla es una sola: donde empieza un lienzo en la textura es multiplo de 32 (los bloques del PVR) y
// los bloques de dos lienzos no se comparten -- porque la conversion escribe el bloque ENTERO (rellenando con
// cero lo que el lienzo no cubre) y dos lienzos en el mismo bloque se borrarian uno al otro.
//
// La fila de la PANTALLA se redondea PARA ARRIBA al multiplo de 32 siguiente (240 -> 256): redondear para abajo
// metaria la fila de abajo encima de la de arriba, y para arriba solo deja un hueco de 16 filas que no se pinta.
func ubicarEnLaTextura(rs []recuadro) (string, bool) {
	for i := range rs {
		tx := rs[i].X
		ty := rs[i].Y

		if tx%ladoDelBloqueDelPvr != 0 {
			tx += ladoDelBloqueDelPvr - tx%ladoDelBloqueDelPvr
		}

		if ty%ladoDelBloqueDelPvr != 0 {
			ty += ladoDelBloqueDelPvr - ty%ladoDelBloqueDelPvr
		}

		rs[i].TX, rs[i].TY = tx, ty

		if tx+rs[i].W > anchoDeLaTextura || ty+rs[i].H > altoDeLaTextura {
			return "el recuadro " + itoa(i) + " no entra en la textura (" + itoa(tx) + "," + itoa(ty) + " " +
				itoa(rs[i].W) + "x" + itoa(rs[i].H) + ")", false
		}
	}

	// los bloques de dos lienzos no se comparten (se compara en BLOQUES, no en pixeles: dos lienzos que comparten
	// un bloque se borrarian parte al subirse)
	for i := 0; i < len(rs); i++ {
		for j := i + 1; j < len(rs); j++ {
			if compartenBloques(rs[i], rs[j]) {
				return "los recuadros " + itoa(i) + " y " + itoa(j) + " comparten bloques de la textura", false
			}
		}
	}

	return "", true
}

// compartenBloques: si los dos recuadros tocan algun bloque de 32x32 de la TEXTURA.
func compartenBloques(a, b recuadro) bool {
	return a.TX < b.TX+b.W && b.TX < a.TX+a.W && a.TY < b.TY+b.H && b.TY < a.TY+a.H
}

// prepararElSplit: los N lienzos en la VRAM, el presentador propio del ciclo y las camaras.
//
// DEVUELVE false (y lo DICE) si falta algo: con el split apagado no hay nada que hacer, y si la VRAM no da para
// los bloques los lienzos van al monton de Go -- que es peor pero se mide igual, y el log dice cual de los dos
// caminos tomo.
func prepararElSplit() bool {
	if splitKnob != "si" {
		println("JUEGO118: SPLIT apagado (palanca split=", splitKnob, "): el cuadro es el de DC-002.115/117")
		return false
	}

	n := len(jugadores)
	if n < 1 || n > maxLienzosDelSplit {
		println("JUEGO118: SPLIT no corre con jugadores=", n, ": el tope son los cuatro puertos maple")
		return false
	}

	if rendererDelMapa == nil || rendererDelCiclo == nil {
		println("JUEGO118: SPLIT no corre sin renderer")
		return false
	}

	nuevos := repartirElSplit(n)
	if nuevos == nil {
		println("JUEGO118: SPLIT no hay reparto para jugadores=", n)
		return false
	}

	if problema, ok := ubicarEnLaTextura(nuevos); !ok {
		println("JUEGO118: SPLIT REPARTO IMPOSIBLE:", problema)
		return false
	}

	lienzosDelSplit = n

	println("JUEGO118: SPLIT REPARTO jugadores=", n, "pantalla=", ancho, alto, "textura=", anchoDeLaTextura,
		altoDeLaTextura, "repartoDeTres=", repartoDeTresKnob)

	bytesDeLosLienzos = 0

	for i := 0; i < lienzosDelSplit; i++ {
		recuadros[i] = nuevos[i]

		bytes := recuadros[i].W * recuadros[i].H * 4

		bloque := pvr.VramReservar(bytes)
		if bloque == nil {
			println("JUEGO118: SPLIT la VRAM no dio para el lienzo", i, "(", bytes, "B): va al monton")
			origenDeLosLienzos = "monton"

			lienzos[i] = pvr.NuevaSurface(recuadros[i].W, recuadros[i].H)
		} else {
			bloquesLienzo[i] = bloque
			lienzos[i] = pvr.SuperficieEnBloque(bloque, recuadros[i].W, recuadros[i].H)
		}

		if lienzos[i] == nil {
			println("JUEGO118: SPLIT no se pudo armar el lienzo", i)
			return false
		}

		// el lienzo arranca NEGRO TRANSPARENTE, como el de la pantalla de DC-002.106: el alfa no llega a la
		// pantalla y el cero es lo que hace comparables los crcs contra un cuadro de un solo jugador.
		lienzos[i].ClearRGBA(0, 0, 0, 0)

		bytesDeLosLienzos += bytes

		println("JUEGO118: SPLIT LIENZO i=", i, "pantalla=", recuadros[i].X, recuadros[i].Y, recuadros[i].W,
			recuadros[i].H, "textura=", recuadros[i].TX, recuadros[i].TY, "bytes=", bytes,
			"enVram=", lienzos[i].Imagen().EnVram(), "vramDisponible=", pvr.VramDisponible())
	}

	// EL RENDERER, EN EL TAMANO DEL PRIMER RECUADRO: despues se pone uno por uno, antes de dibujar cada uno.
	rendererDelMapa.FijarTamanoDelCuadro(recuadros[0].W, recuadros[0].H)

	// EL CICLO PRESENTA LOS LIENZOS, NO EL DE LA PANTALLA: el lienzo de 640x480 no se reserva.
	rendererDelCiclo.UsarSuperficieDelCuadro(lienzos[0])
	rendererDelCiclo.UsarPresentador(presentarElSplit)

	splitActivo = true

	println("JUEGO118: SPLIT LIENZOS n=", lienzosDelSplit, "bytesTotales=", bytesDeLosLienzos, "origen=",
		origenDeLosLienzos, "bytesDeLaPantallaEntera=", ancho*alto*4,
		"mismosBytesQueUnLienzoUnico=", bytesDeLosLienzos == ancho*alto*4, "elLienzoDeLaPantallaNoSeReserva=",
		true)

	vram("split-despues-de-los-lienzos")
	ponerLasCamarasDelSplit()

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

// ponerLaCamara: la camara del motor en la posicion del jugador `i`. Es lo unico que se toca entre un recuadro y
// el otro: el viewport, la cache de tiles y el mapa son los mismos (lo que es por jugador es la camara).
func ponerLaCamara(i int) {
	if i < 0 || i >= lienzosDelSplit {
		return
	}

	pos := d2vector.NewPosition(camarasDelSplit[i][0], camarasDelSplit[i][1])

	rendererDelMapa.MoveCameraTo(&pos)
}

// ponerElTamanoDelRecuadro: el renderer del mapa en el tamano del recuadro `i`. Es lo que hace que el jugador de
// ESE recuadro caiga en el centro de SU lienzo (el viewport proyecta media pantalla). Con la rejilla de cuatro
// hay dos tamanos distintos, y `Viewport.Resize` no cuesta nada cuando el tamano ya es el que esta.
func ponerElTamanoDelRecuadro(i int) {
	espacio := rendererDelMapa.TamanoDelCuadro()

	if espacio.W != recuadros[i].W || espacio.H != recuadros[i].H {
		rendererDelMapa.FijarTamanoDelCuadro(recuadros[i].W, recuadros[i].H)
	}
}

// presentarElSplit: la pantalla repartida. Cada lienzo a SU region de la textura y un quad por recuadro.
//
// ES EL PRESENTADOR QUE EL CICLO LLAMA EN LUGAR DE `PresentarEnPantalla` (ver `pvr.Renderer.UsarPresentador`):
// lo que se sube son N lienzos, no uno de 640x480, y la cuenta de pixeles es la misma (307.200).
func presentarElSplit() error {
	t0 := d2cMs()

	for i := 0; i < lienzosDelSplit; i++ {
		if lienzos[i] == nil {
			continue
		}

		if err := pvr.PresentarEnBloque(lienzos[i].Imagen(), recuadros[i].TX, recuadros[i].TY); err != nil {
			println("JUEGO118: SPLIT no se pudo presentar el lienzo", i, ":", err.Error())
			return err
		}
	}

	for i := 0; i < lienzosDelSplit; i++ {
		pvr.PintarEnBloque(recuadros[i].X, recuadros[i].Y, recuadros[i].W, recuadros[i].H,
			recuadros[i].TX, recuadros[i].TY)
	}

	msPresentarDelCuadro = int(d2cMs() - t0)
	msPresentarDelSplit += msPresentarDelCuadro

	return nil
}

// dibujarElSplit: EL CUADRO DE LA REJILLA. N camaras, N lienzos, y el mapa de cada jugador dibujado en el suyo.
//
// LOS CUADROS DEL CIERRE SON LOS MISMOS DE DC-002.113/.115 (origen, repetibilidad y vuelta al final), pero
// medidos POR RECUADRO: el crc de cada lienzo en el cuadro del origen tiene que ser el mismo en el siguiente.
func dibujarElSplit(superficie d2interface.Surface) error {
	cuadro++

	if cuadro > cuadros+3 {
		return nil
	}

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
		// DC-002.118: la camara de cada recuadro se vuelve a poner ACA. `volverAlOrigen`/`volverAlFinal` mueven a
		// los jugadores DENTRO del cuadro (los que llama main.go en `actualizar` corren antes), asi que sin esto
		// los dos cuadros del cierre se dibujarian con las camaras del cuadro anterior y los cuatro saldrian
		// fuera de su recuadro (medido: cuadrosFuera=4 en el cuadro 7 y 3 en el 9, y la repetibilidad fallando).
		ponerLasCamarasDelSplit()
	case cuadros + 3:
		volverAlFinal()
		ponerLasCamarasDelSplit()
	}

	if cuadro == 1 {
		println("JUEGO118: SPLIT PRIMER-CUADRO cuadro=1 lienzos=", lienzosDelSplit, "jugadores=", len(jugadores),
			"repartoDeTres=", repartoDeTresKnob)
	}

	msMapaDelCuadro := 0
	msJugadorDelCuadro := 0

	for i := 0; i < lienzosDelSplit; i++ {
		msMapaUno, msJugadorUno := dibujarUnRecuadro(i)

		msMapaDelCuadro += msMapaUno
		msJugadorDelCuadro += msJugadorUno
	}

	msMapaDelSplit += msMapaDelCuadro
	msJugadorDelSplit += msJugadorDelCuadro
	cuadrosDelSplit++

	// LA SEPARACION DE ESTE CUADRO (el criterio de la tarjeta) y la vigilancia de cada recuadro.
	leerLaSeparacionDelSplit(cuadro)

	// EL CUADRO COMPLETO: el crc de los N lienzos leidos en orden (del 1 al N).
	crcCompleto := crcDeLosLienzos()

	println("JUEGO118: SPLIT CUADRO", cuadro, "lienzos=", lienzosDelSplit, "crc32=", int(crcCompleto),
		"pixelesNoVacios=", llenosDeLosLienzos(), "de", ancho*alto, "msMapa=", msMapaDelCuadro,
		"msJugadores=", msJugadorDelCuadro, "msPresentar=", msPresentarDelCuadro,
		"msCuadro=", msDelCuadroDelSplit, "celdas=", separacionDelSplit(), "jugadores=", len(jugadores),
		"repartoDeTres=", repartoDeTresKnob)

	// EL CIERRE: el origen se guarda y el cuadro siguiente tiene que dar lo mismo (por recuadro).
	switch cuadro {
	case cuadros + 1:
		for i := 0; i < lienzosDelSplit; i++ {
			crcOrigenDeRecuadro[i] = crcDelRecuadro[i]
			crcConJugadorOrigen[i] = crcConJugadorDelRecuadro[i]
		}

		println("JUEGO118: SPLIT ORIGEN cuadro=", cuadro, "crcP1=", int(crcDelRecuadro[0]),
			"crcP2=", int(crcDelRecuadro[1]), "crcP3=", int(crcDelRecuadro[2]), "crcP4=",
			int(crcDelRecuadro[3]))
	case cuadros + 2:
		iguales := true
		igualesConJugador := true

		for i := 0; i < lienzosDelSplit; i++ {
			if crcDelRecuadro[i] != crcOrigenDeRecuadro[i] {
				iguales = false
			}

			if crcConJugadorDelRecuadro[i] != crcConJugadorOrigen[i] {
				igualesConJugador = false
			}
		}

		println("JUEGO118: SPLIT REPETIBILIDAD cuadro=", cuadro, "lienzos=", lienzosDelSplit,
			"igualesPorRecuadro=", iguales, "igualesConJugador=", igualesConJugador, "crcP1=",
			int(crcDelRecuadro[0]), "crcP1DelCuadroAnterior=", int(crcOrigenDeRecuadro[0]), "crcP1DelCuadro1=",
			int(crcCuadro1Recuadro[0]), "igualAlCuadro1P1=", crcDelRecuadro[0] == crcCuadro1Recuadro[0])
	case cuadros + 3:
		resumenDelSplit()
	}

	return nil
}

// dibujarUnRecuadro: el mapa y el jugador de UN recuadro, en SU lienzo. Devuelve los ms del mapa y los del
// jugador.
func dibujarUnRecuadro(i int) (int, int) {
	p := jugadores[i]

	anchoLienzo := recuadros[i].W
	altoLienzo := recuadros[i].H

	ponerElTamanoDelRecuadro(i)
	ponerLaCamara(i)

	t0 := d2cMs()
	rendererDelMapa.Render(lienzos[i])
	msMapa := int(d2cMs() - t0)

	pixeles := lienzos[i].Imagen().Pixeles()

	llenos := contarLlenos(pixeles)
	crcMapa := crc32.ChecksumIEEE(pixeles)

	crcDelRecuadro[i] = crcMapa
	llenosDelRecuadro[i] = llenos

	if cuadro == 1 {
		crcCuadro1Recuadro[i] = crcMapa
	}

	// el jugador de ESTE recuadro, con la camara de ESTE recuadro (el ancla sale del `WorldToScreenF` que acaba de
	// usar el mapa: son la misma cuenta).
	msJugador := 0
	crcConJugador := crcMapa
	pintado := false

	cu, listo := prepararElCuadroDeUnRecuadro(p, pixeles, anchoLienzo, altoLienzo)
	if jugadorOK && listo {
		t1 := d2cMs()
		PintarJugador(lienzos[i], cu, p.AnclaX, p.AnclaY, p.Pal[:])
		msJugador = int(d2cMs() - t1)

		p.MsDibujo = msJugador
		p.Cambiados = pixelesCambiadosEnElRecuadro(pixeles, p, anchoLienzo)

		crcConJugador = crc32.ChecksumIEEE(pixeles)
		pintado = true

		restaurarCajaEnElRecuadro(p, pixeles, anchoLienzo)

		if crc32.ChecksumIEEE(pixeles) != crcMapa {
			println("JUEGO118: SPLIT RECUADRO", i, "MAPA-RESTAURADO DISTINTO cuadro=", cuadro,
				"crc32=", int(crc32.ChecksumIEEE(pixeles)), "mapaSolo=", int(crcMapa))
		}
	}

	crcConJugadorDelRecuadro[i] = crcConJugador

	if cuadro == 1 {
		crcCuadro1ConJugador[i] = crcConJugador
	}

	cx, cy := p.Mov.Est.Celda()
	camX, camY := camaraDelRecuadroEnCeldas(i)

	println("JUEGO118: SPLIT RECUADRO cuadro=", cuadro, "i=", i, "jugador=", p.Nombre, "pantalla=",
		recuadros[i].X, recuadros[i].Y, anchoLienzo, altoLienzo, "textura=", recuadros[i].TX, recuadros[i].TY,
		"celda=", cx, cy, "camara=", camX, camY, "ancla=", p.AnclaX, p.AnclaY,
		"caja=", p.Caja.X0, p.Caja.Y0, p.Caja.X1, p.Caja.Y1, "tamano=", p.Caja.Ancho(), "x", p.Caja.Alto(),
		"cajaDentro=", siNo(p.Dentro), "pixelesNoVacios=", llenos, "de", anchoLienzo*altoLienzo,
		"crcMapa=", int(crcMapa), "crcConJugador=", int(crcConJugador), "pixelesDelJugador=", p.Cambiados,
		"msMapa=", msMapa, "msJugador=", msJugador, "cuadroDelCiclo=", p.CuadroDelCiclo)

	// el cuadro que queda en pantalla: se vuelve a pintar el jugador sobre el mapa restaurado
	if pintado {
		PintarJugador(lienzos[i], cu, p.AnclaX, p.AnclaY, p.Pal[:])
	}

	return msMapa, msJugador
}

// camaraDelRecuadroEnCeldas: la posicion ortogonal de la camara, de vuelta a CELDAS (la inversa de
// `WorldToOrtho`: orthoX = (x-y)*80, orthoY = (x+y)*40), para que el log diga en que celda quedo cada camara.
func camaraDelRecuadroEnCeldas(i int) (float64, float64) {
	ox, oy := camarasDelSplit[i][0], camarasDelSplit[i][1]

	x := oy/80.0 + ox/160.0
	y := oy/80.0 - ox/160.0

	return x, y
}

// leerLaSeparacionDelSplit: la separacion en CELDAS del cuadro (el par mas lejano) y la vigilancia de que cada
// jugador este DENTRO DE SU recuadro.
func leerLaSeparacionDelSplit(cuadro int) {
	if len(jugadores) < 2 {
		return
	}

	cuadrosVigilados++

	d := separacionDelSplit()

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

		println("JUEGO118: SPLIT FUERA cuadro=", cuadro, "jugador=", jugadores[i].Nombre,
			"caja=", jugadores[i].CajaCruda.X0, jugadores[i].CajaCruda.Y0, jugadores[i].CajaCruda.X1,
			jugadores[i].CajaCruda.Y1, "recuadro=", recuadros[i].W, recuadros[i].H)
	}

	cuadrosFueraDeSuRecuadro += fuera

	if cuadro == 1 || cuadro%10 == 0 || cuadro > cuadros {
		println("JUEGO118: SPLIT SEPARACION cuadro=", cuadro, "jugadores=", len(jugadores), "celdas=", d,
			"masDe20=", siNo(d > celdasDelSplit), "maxCeldas=", separacionMaxima, "cuadrosMasDe20=",
			cuadrosSepMasDe20, "cuadrosFuera=", fuera)
	}
}

// separacionDelSplit: la separacion del cuadro (el par MAS LEJANO), en celdas. Con dos es la de siempre; con tres
// y cuatro es la que dice si el grupo se podria ver en un solo cuadro o si el split es la unica salida.
func separacionDelSplit() float64 {
	if len(jugadores) < 2 {
		return 0
	}

	maxima := 0.0

	for i := 0; i < len(jugadores); i++ {
		for j := i + 1; j < len(jugadores); j++ {
			xi, yi := jugadores[i].Mov.Est.Celda()
			xj, yj := jugadores[j].Mov.Est.Celda()

			if d := distanciaEnCeldas(xi, yi, xj, yj); d > maxima {
				maxima = d
			}
		}
	}

	return maxima
}

// prepararElCuadroDeUnRecuadro: lo mismo que `prepararElCuadroDeUnJugador` (coop.go), pero con el TAMANO DEL
// RECUADRO en la mano: el ancla, la caja, si entra y la copia de lo que hay debajo se miden contra w x h, no
// contra 640x480.
//
// LA CUENTA DEL ANCLA ES LA MISMA DEL MOTOR (`AnclaDelJugador` con el `WorldToScreenF` del renderer, que en el
// split es el del recuadro que se acaba de dibujar): no hay una formula nueva por estar en un recuadro.
func prepararElCuadroDeUnRecuadro(p *participante, pixeles []byte, anchoRecuadro, altoRecuadro int) (*CuadroDelJugador, bool) {
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

	// la caja SIN recortar es el numero del criterio (que el jugador entre en SU recuadro)
	p.CajaCruda = CajaDe(p.AnclaX, p.AnclaY, cu)
	p.Dentro = cajaDentroDelRecuadro(p.CajaCruda, anchoRecuadro, altoRecuadro)

	p.Caja = recortarAlRecuadro(p.CajaCruda, anchoRecuadro, altoRecuadro)
	if p.Caja.Ancho() <= 0 || p.Caja.Alto() <= 0 {
		return nil, false
	}

	guardarCajaEnElRecuadro(p, pixeles, anchoRecuadro)

	return cu, true
}

// cajaDentroDelRecuadro: la caja entera adentro del lienzo (los bordes incluidos: X1 e Y1 son exclusivos).
func cajaDentroDelRecuadro(c CajaDelJugador, anchoRecuadro, altoRecuadro int) bool {
	return c.X0 >= 0 && c.Y0 >= 0 && c.X1 <= anchoRecuadro && c.Y1 <= altoRecuadro
}

// recortarAlRecuadro: la caja adentro del recuadro (escribir afuera de un lienzo es corromper memoria ajena).
func recortarAlRecuadro(c CajaDelJugador, anchoRecuadro, altoRecuadro int) CajaDelJugador {
	if c.X0 < 0 {
		c.X0 = 0
	}

	if c.Y0 < 0 {
		c.Y0 = 0
	}

	if c.X1 > anchoRecuadro {
		c.X1 = anchoRecuadro
	}

	if c.Y1 > altoRecuadro {
		c.Y1 = altoRecuadro
	}

	if c.X1 < c.X0 {
		c.X1 = c.X0
	}

	if c.Y1 < c.Y0 {
		c.Y1 = c.Y0
	}

	return c
}

// guardarCajaEnElRecuadro: copia lo que hay en el lienzo debajo de la caja del jugador. Es `guardarCajaDe`
// (coop.go) con el paso de fila del LIENZO (su ancho), no el de la pantalla.
func guardarCajaEnElRecuadro(p *participante, pixeles []byte, anchoRecuadro int) {
	necesario := p.Caja.Ancho() * p.Caja.Alto() * 4

	if p.CajaGuardada == nil || len(p.CajaGuardada) < necesario {
		p.CajaGuardada = make([]byte, necesario)
	}

	n := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*anchoRecuadro + x) * 4
			copy(p.CajaGuardada[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}
}

// restaurarCajaEnElRecuadro: devuelve la caja del jugador a lo que habia debajo (el mapa de SU recuadro).
func restaurarCajaEnElRecuadro(p *participante, pixeles []byte, anchoRecuadro int) {
	n := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*anchoRecuadro + x) * 4
			copy(pixeles[j:j+4], p.CajaGuardada[n:n+4])

			n += 4
		}
	}
}

// pixelesCambiadosEnElRecuadro: cuantos pixeles de la caja del jugador NO son los del mapa (los que pinto de
// verdad).
func pixelesCambiadosEnElRecuadro(pixeles []byte, p *participante, anchoRecuadro int) int {
	n := 0
	cambiados := 0

	for y := p.Caja.Y0; y < p.Caja.Y1; y++ {
		for x := p.Caja.X0; x < p.Caja.X1; x++ {
			j := (y*anchoRecuadro + x) * 4

			if pixeles[j] != p.CajaGuardada[n] || pixeles[j+1] != p.CajaGuardada[n+1] ||
				pixeles[j+2] != p.CajaGuardada[n+2] || pixeles[j+3] != p.CajaGuardada[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// crcDeLosLienzos: el crc32 de los N lienzos LEIDOS EN ORDEN (del 1 al N). Es el numero estable del cuadro
// partido: no es el de un lienzo de 640x480 (esos pixeles no existen en el split), y se calcula sin pedir un
// buffer de 640x480 solo para hashear.
func crcDeLosLienzos() uint32 {
	d := crc32.NewIEEE()

	for i := 0; i < lienzosDelSplit; i++ {
		if lienzos[i] == nil {
			continue
		}

		_, _ = d.Write(lienzos[i].Imagen().Pixeles())
	}

	return d.Sum32()
}

// llenosDeLosLienzos: los pixeles no negros de todos los lienzos (el "cuadro" del split son todos).
func llenosDeLosLienzos() int {
	total := 0

	for i := 0; i < lienzosDelSplit; i++ {
		total += llenosDelRecuadro[i]
	}

	return total
}

// resumenDelSplit: los numeros del criterio de esta tarjeta, en cuatro lineas.
func resumenDelSplit() {
	memoria("split-fin")
	semiespacio("split-fin")

	cache := rendererDelMapa.CensoDelTramo()

	println("JUEGO118: SPLIT RESUMEN lienzos=", lienzosDelSplit, "jugadores=", len(jugadores), "repartoDeTres=",
		repartoDeTresKnob, "bytesDeLienzos=", bytesDeLosLienzos, "origen=", origenDeLosLienzos,
		"cuadrosPintados=", cuadrosDelSplit, "msMapaTotal=", msMapaDelSplit, "msJugadorTotal=",
		msJugadorDelSplit, "msPresentarTotal=", msPresentarDelSplit, "msMapaPorCuadro=",
		porCuadro(msMapaDelSplit, cuadrosDelSplit), "msJugadorPorCuadro=",
		porCuadro(msJugadorDelSplit, cuadrosDelSplit), "msPresentarPorCuadro=",
		porCuadro(msPresentarDelSplit, cuadrosDelSplit), "msDelCuadroUltimo=", msDelCuadroDelSplit)

	println("JUEGO118: SPLIT CRITERIO separacionMinCeldas=", separacionMinima, "separacionMaxCeldas=",
		separacionMaxima, "masDe20=", siNo(separacionMaxima > celdasDelSplit), "cuadrosMasDe20=",
		cuadrosSepMasDe20, "cuadrosVigilados=", cuadrosVigilados, "cuadrosFueraDeSuRecuadro=",
		cuadrosFueraDeSuRecuadro, "lienzos=", lienzosDelSplit)

	println("JUEGO118: SPLIT MEMORIA vramTotal=", pvr.VramTotal(), "vramDisponible=", pvr.VramDisponible(),
		"vramPedidaPorEstePoc=", pvr.FotoDeLaVram().Pedida, "lienzoDeLaPantalla=", pvr.VramDelLienzo(),
		"cacheDelTramoBytes=", cache.Bytes, "cacheUsado=", cache.Usado, "cacheImagenes=", cache.Imagenes,
		"cacheDerrames=", cache.Derrames, "montonLibre=", d2cMontonLibre(), "semiespacioUsado=",
		int(d2cGcUsado()), "semiespacioTotal=", int(d2cGcTotal()), "colecciones=", int(d2cGcColecciones()))

	println("JUEGO118: SPLIT FIN lienzos=", lienzosDelSplit, "crcP1=", int(crcDelRecuadro[0]), "crcP2=",
		int(crcDelRecuadro[1]), "crcP3=", int(crcDelRecuadro[2]), "crcP4=", int(crcDelRecuadro[3]),
		"crcConJugadorP1=", int(crcConJugadorDelRecuadro[0]), "crcConJugadorP2=",
		int(crcConJugadorDelRecuadro[1]), "crcConJugadorP3=", int(crcConJugadorDelRecuadro[2]),
		"crcConJugadorP4=", int(crcConJugadorDelRecuadro[3]))
}

// porCuadro: el promedio de un acumulado sobre los cuadros pintados (sin fmt: la consola no lo trae).
func porCuadro(total, n int) float64 {
	if n <= 0 {
		return 0
	}

	return float64(total) / float64(n)
}
