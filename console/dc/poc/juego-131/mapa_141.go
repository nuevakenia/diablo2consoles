//go:build dreamcast

package main

import (
	"hash/crc32"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// DC-002.141 -- EL CUADRO CAMINANDO: EL MUNDO SE CORRE Y SE REPINTA SOLO LO QUE ENTRA.
//
// EL NUMERO QUE MANDA (DC-002.139, Flycast, la misma corrida de la demo). Con las tres palancas del mapa
// estatico la corrida da fpsMedio=30, pero el desglose por ventanas de 30 cuadros dice que son DOS casos:
//
//	parado     23-30 ms/cuadro = 30-43 fps   (el mapa NO se repinta: 2,6 ms)
//	caminando  77-92 ms/cuadro = 11-13 fps   (la camara se movio: el mapa se repinta ENTERO, 60,4 ms)
//
// Y repintar entero, caminando, es tirar el trabajo: entre dos cuadros que caminan el mapa es EL MISMO MAPA
// CORRIDO unos pixeles. Los 60,4 ms son 248 tiles, 1.359.235 lecturas de indices contra la VRAM y 307.200
// escrituras de pixel, tambien contra la VRAM (~50 ns cada viaje).
//
// LO QUE HACE ESTA TARJETA (la palanca `SCROLL_141=si`):
//
//  1. CORRE EL LIENZO con UNA copia en el lugar (`pvr.MoverLienzo`, en C, 64 bits por vuelta): el contenido se
//     mueve (sx, sy) pixeles, que es exactamente lo que la camara se movio. Eso es todo el mapa.
//  2. REPINTA DEL MAPA SOLO LO QUE LA COPIA NO TRAJO, con la lista de recortes del backend
//     (`pvr.Surface.RecortarA`, ver recortes_141.go):
//       - las FRANJAS que entran por el borde (|sx| columnas y/o |sy| filas);
//       - las CAJAS de los entes del cuadro anterior, corridas con la copia (si no, queda el fantasma del
//         sprite viejo donde estaba el jugador, el bicho, el NPC o el objeto);
//       - las CAJAS de los paneles (HUD, minimapa) ENTERAS: sus agujeros dejan ver el mundo (no son opacos:
//         `hudPiezas=esenciales` no carga el panel de fondo, y el arte de un globo tiene 1376 pixeles
//         transparentes de 6400), asi que el mapa tiene que estar limpio debajo; y ademas la banda que asoma del
//         panel del cuadro anterior, corrido con la copia, que si no deja su fantasma al costado.
//  3. La presentacion convierte el cuadro entero, como siempre (el lienzo se movio todo). Lo que se ahorra
//     son los 60 ms del mapa, no la conversion.
//
// LO QUE NO CAMBIA, Y ES EL CRITERIO: el cuadro 1 se pinta ENTERO con el camino de siempre (los tres primeros
// cuadros lo hacen, ver `comoSeDibujaElMapa141`), asi que su crc32 tiene que seguir dando lo mismo
// (SOLO-MAPA -1654008076, CON-JUGADOR -1025148544). Y un cuadro caminando tiene que quedar IDENTICO al que
// daria un repintado entero: el corrimiento mueve los mismos pixeles y las franjas se pintan con el mismo
// bucle del mapa (solo cambia QUE pixeles se escriben). Eso NO se afirma: se MIDE con `COMPARAR_141=si`, y la
// comparacion es sobre el cuadro TERMINADO (mapa + entes + HUD + minimapa), bloque por bloque de 32x32, los 300
// del lienzo: ver `compararElCuadroTerminado141`. (La version anterior comparaba el mapa ANTES de los paneles y
// tenia que saltear los bloques de las cajas de los paneles: 120 de 300 bloques comparados, 180 sin prueba.)
//
// CUANDO NO SE PUEDE CAMINAR Y SE REPINTA ENTERO (y por que):
//   - los tres primeros cuadros (ahi vive la comparacion contra el host);
//   - cambio el nivel, se abrio/cerro el panel, o el nivel tiene tiles animados (el mapa ya no es estatico);
//   - el salto de la camara es mayor que media pantalla (un teletransporte: no hay nada que copiar);
//   - el cuadro anterior se resolvio con la COPIA y este esta QUIETO: el camino quieto devuelve la caja
//     guardada de cada ente (DC-002.139), y esa copia es de una camara vieja -- devolverla dejaria el mapa del
//     cuadro de antes. Se repinta entero una vez y de ahi siguen los cuadros quietos que ya andaban;
//   - `SPLIT=si` (dos lienzos de 320x480: la copia y los recortes son de UN lienzo) o si el backend no esta
//     en los caminos rapidos (el de bytes no sabe de recortes por rectangulo, ver recortes_141.go).

const (
	// Los tres caminos del cuadro. `mapa141Entero` y `mapa141Quieto` son los de DC-002.139, bit a bit.
	mapa141Entero     = 0
	mapa141Quieto     = 1
	mapa141Desplazado = 2
)

var (
	// scroll141SX/SY: lo que se movio el contenido del lienzo en el cuadro que camina.
	scroll141SX, scroll141SY int

	// scroll141Caminando: este cuadro se resolvio con la copia. Lo lee `midiendoElCuadro134`: en un cuadro que
	// camina NO se guardan las cajas de los entes (la copia las pisa y el mundo se repinta ahi mismo), y eso
	// son los 2,7 ms de los jugadores + 4-6 de los bichos + 1,7 de los NPCs que DC-002.139 pagaba por cuadro.
	scroll141Caminando bool

	// scroll141UltimoFueCopia: el cuadro anterior se resolvio con la copia (ver el punto 4 de la cabecera).
	scroll141UltimoFueCopia bool

	// El censo del tramo, para la evidencia.
	scroll141Desplazados, scroll141Enteros, scroll141Quietos, scroll141Fallos int
	scroll141PxFranjas, scroll141PxCajas, scroll141PxPaneles                  int
	scroll141UsMover, scroll141UsFranjas                                      int
	scroll141Recortes, scroll141MaxSX, scroll141MaxSY                         int

	// scroll141RecortesIgnorados: cuantos rectangulos se descartaron por no entrar en la lista del lienzo
	// (`recortesMax141` = 32). Es acumulado de la corrida y 0 es lo esperado: si algun dia deja de serlo, el
	// lienzo queda mal y el log lo dice en vez de dejarlo pasar en silencio.
	scroll141RecortesIgnorados int

	// scroll141Comparaciones: cuantas veces se comparo el cuadro TERMINADO contra el mismo cuadro repintado
	// entero (palanca `COMPARAR_141=si`).
	scroll141Comparaciones int

	// scroll141ComparacionesSaltadas: cuadros que caminaban y no se compararon porque habia un panel abierto
	// (el panel abierto se pinta DESPUES y no es parte del camino del mapa: se sale de la prueba).
	scroll141ComparacionesSaltadas int

	// LA COBERTURA DE LOS CUATRO SIGNOS. Un cuadro caminando puede correrse para cualquiera de los cuatro
	// cuadrantes, y las ramas del corrimiento NO son simetricas en el codigo: `copiar_bloque` elige la direccion
	// del memmove por el signo, `agregarFranjaVertical141` tiene su rama de sx>0 y `agregarBanda141` las tiene de
	// sx>0 y de sy>0. Comparar los primeros cuadros de un tramo recto gasta las comparaciones en UN cuadrante y
	// deja los otros tres sin probar. Por eso la prueba compara hasta `comparacionesPorCuadrante141` cuadros de
	// CADA cuadrante (mascara de signos) y no un total suelto.
	scroll141Cuadrante   int // el codigo del ultimo cuadro caminando comparado (0 = ninguno todavia)
	scroll141EnCuadrante int // cuantas comparaciones se hicieron en ese cuadrante
	scroll141Cuadrantes  int // mascara de los cuadrantes ya comparados (un bit por codigo)

	// pintandoLaReferencia141: mientras dura el cuadro de REFERENCIA de la comparacion. Los pintores que se
	// saltean el trabajo cuando el instrumento por cuadro esta apagado (`volverAPintar*`, ver
	// `midiendoElCuadro134`) tienen que pintar igual: la referencia es el cuadro ENTERO.
	pintandoLaReferencia141 bool

	// barrasDelCuadro141: la vida y el mana con los que el HUD pinto ESTE cuadro, guardadas ANTES de que `dibujar`
	// las baje (`hudInstrumento` las baja cuadro a cuadro). La referencia de la comparacion tiene que pintar el
	// HUD con ESOS valores: si pintara con los de despues de la bajada, el globo quedaria un pixel mas bajo y la
	// comparacion del cuadro terminado acusaria una diferencia que no es del mapa.
	barrasDelCuadro141 [2][4]int
)

// comoSeDibujaElMapa141: cual de los tres caminos le toca a ESTE cuadro.
//
// Con la palanca apagada contesta lo mismo que `mapaSeRepinta139` (el camino de DC-002.139, sin una linea de
// diferencia). Con la palanca prendida usa su propia decision, porque el cuadro que camina NO necesita las
// cajas guardadas (`cajas139`) y no tiene por que depender de esa palanca.
func comoSeDibujaElMapa141() int {
	if rendererDelMapa == nil {
		return mapa141Entero
	}

	if scroll141 != "si" {
		if mapaSeRepinta139() {
			return mapa141Entero
		}

		return mapa141Quieto
	}

	// lo que obliga a repintar entero: los primeros cuadros (la comparacion contra el host), un lienzo que
	// todavia no se pinto, otro nivel, el panel abierto/cerrado, tiles animados, el split y los caminos lentos.
	if cuadro <= 3 || !mapa139Valido || demoNivelActual != mapa139Nivel || dialogoAbierto != mapa139Dialogo ||
		rendererDelMapa.TilesAnimados139() > 0 || splitKnob == "si" ||
		!pvr.BlitRapido() || !pvr.CaminoDeLaPalabra() {
		return mapa141Entero
	}

	x, y := rendererDelMapa.TraslacionDeLaPantalla139()
	sx, sy := x-mapa139X, y-mapa139Y

	if sx == 0 && sy == 0 {
		// quieto: el camino de DC-002.139, que devuelve el mapa debajo de la caja de cada ente. Necesita las
		// copias (`cajas139`) y que sean de ESTA camara.
		if cajas139 != "si" || scroll141UltimoFueCopia {
			return mapa141Entero
		}

		return mapa141Quieto
	}

	// un salto mayor que media pantalla no es caminar: no queda nada que copiar.
	if sx <= -ancho/2 || sx >= ancho/2 || sy <= -alto/2 || sy >= alto/2 {
		return mapa141Entero
	}

	scroll141SX, scroll141SY = sx, sy

	return mapa141Desplazado
}

// dibujarElCuadroQueCamina141: el cuadro caminando entero -- la copia y despues el mapa solo en los recortes.
// Devuelve false si no se pudo (y entonces el que llama repinta entero).
func dibujarElCuadroQueCamina141(malla *pvr.Surface, superficie d2interface.Surface) bool {
	sx, sy := scroll141SX, scroll141SY

	malla.SinRecortes()

	// DC-002.142: la prueba de la conversion, ANTES del corrimiento (ver `informe_142.go` y
	// `d2c_pvr_pantalla_prueba_conv142`): el mismo bucle cuesta 9 us por bloque en un cuadro quieto y 80 en uno
	// que camina, y hay que saber si el que cambia es el bucle o la memoria despues de la copia.
	usConvAntes142 := 0
	if pruebaConv142 == "si" {
		usConvAntes142 = pvr.PruebaDeLaConversion142(malla.Imagen(), bloquesDeLaPrueba142)
	}

	tMover := d2cUs()

	if r := pvr.MoverLienzo(sx, sy); r != 0 {
		scroll141Fallos++

		println("JUEGO141: LA COPIA NO SE PUDO codigo=", r, "sx=", sx, "sy=", sy)

		return false
	}

	scroll141UsMover += int(d2cUs() - tMover)

	scroll141Caminando = true
	scroll141UltimoFueCopia = true

	// DC-002.142: este cuadro camino; el informe por cuadro lo va a leer en el cuadro siguiente (los contadores
	// de la presentacion de ESTE cuadro no estan puestos todavia: se presentan despues de dibujar).
	caminoElCuadroAnterior142 = true
	sxDelCuadroAnterior142, syDelCuadroAnterior142 = sx, sy

	tFranjas := d2cUs()

	t143Recortes := cron143()
	pxFranjas, pxCajas, pxPaneles, recortes := armarLosRecortes141(malla, sx, sy)
	marca143(e143Recortes, t143Recortes)

	t143Render := cron143()
	rendererDelMapa.Render(superficie)
	marca143(e143RenderMapa, t143Render)

	usFranjas := int(d2cUs() - tFranjas)

	// y la misma prueba DESPUES del corrimiento y del repintado: si el numero sube, el que encarece la
	// conversion no es el bucle sino el estado de la VRAM que deja la copia.
	if pruebaConv142 == "si" {
		usConvDespues := pvr.PruebaDeLaConversion142(malla.Imagen(), bloquesDeLaPrueba142)

		println("JUEGO142: PRUEBA-CONV cuadro=", cuadro, "sx=", sx, "sy=", sy,
			"bloques=", bloquesDeLaPrueba142, "usAntes=", usConvAntes142, "usDespues=", usConvDespues,
			"usMover=", int(d2cUs()-tMover))
	}

	scroll141UsFranjas += usFranjas

	malla.SinRecortes()

	// LA PRUEBA NO SE HACE ACA. El cuadro caminando todavia no tiene los entes ni los paneles encima, y ahi
	// adentro de las cajas de los paneles los dos caminos SON distintos por construccion. La comparacion se hace
	// al final del cuadro, con el cuadro TERMINADO: `compararElCuadroTerminado141` (mas abajo), que la llama
	// `main.go` despues del HUD y del minimapa.

	scroll141PxFranjas += pxFranjas
	scroll141PxCajas += pxCajas
	scroll141PxPaneles += pxPaneles
	scroll141Recortes += recortes
	scroll141Desplazados++

	if sx > scroll141MaxSX {
		scroll141MaxSX = sx
	}
	if -sx > scroll141MaxSX {
		scroll141MaxSX = -sx
	}
	if sy > scroll141MaxSY {
		scroll141MaxSY = sy
	}
	if -sy > scroll141MaxSY {
		scroll141MaxSY = -sy
	}

	return true
}

// armarLosRecortes141: los rectangulos que hay que repintar del mapa en un cuadro que camina. Devuelve los
// pixeles (aproximados, ya recortados al lienzo) de cada grupo y cuantos rectangulos se pusieron.
func armarLosRecortes141(malla *pvr.Surface, sx, sy int) (franjas, cajas, paneles, cuantos int) {
	// ---- 1. LAS FRANJAS QUE ENTRAN ---------------------------------------------------------------
	//
	// El contenido se movio (sx, sy): lo que asoma por el borde que la copia dejo libre es exactamente lo que
	// hay que pintar. Con los dos ejes moviendose son dos rectangulos (una columna y una fila) y se pisan en
	// una esquina: pintarla dos veces da el mismo pixel (el mapa es opaco y se pinta igual), asi que no se
	// recorta la interseccion.
	//
	// LO QUE NO SE PINTA ACA. De la franja se saltea lo que los paneles tapan de verdad (su caja medida: el HUD
	// de arriba, el pie y la caja del minimapa): eso lo repinta el punto 3, mapa abajo incluido. Ojo, es un
	// ahorro de pixeles, no una suposicion de opacidad: los paneles NO son opacos (ver el punto 3).
	if sx > 0 {
		franjas += agregarFranjaVertical141(malla, 0, sx)
	} else if sx < 0 {
		franjas += agregarFranjaVertical141(malla, ancho+sx, ancho)
	}

	if sy > altoDelHUD141 {
		malla.RecortarA(0, altoDelHUD141, ancho, sy)
		franjas += ancho * (sy - altoDelHUD141)
	} else if -sy > altoDelPie141 {
		malla.RecortarA(0, alto+sy, ancho, alto-altoDelPie141)
		franjas += ancho * (-sy - altoDelPie141)
	}

	// ---- 2. LOS FANTASMAS DE LOS ENTES ----------------------------------------------------------
	//
	// La copia se llevo los pixeles del jugador/bicho/NPC/objeto a donde estaban MAS (sx, sy): ahi hay que
	// devolver el mapa. La caja que se usa es la del cuadro ANTERIOR, que es la que quedo en `Caja` (cada
	// dibujado la arma con su ancla; el mapa se pinta antes que los entes, asi que todavia es la de ayer).
	for _, p := range jugadores {
		cajas += agregarCaja141(malla, p.Caja, sx, sy)
	}

	for _, b := range bichos {
		cajas += agregarCaja141(malla, b.Caja, sx, sy)
	}

	for _, n := range npcsDelPueblo {
		cajas += agregarCaja141(malla, n.Caja, sx, sy)
	}

	for _, it := range suelo {
		cajas += agregarCaja141(malla, it.Caja, sx, sy)
	}

	// ---- 3. LAS BANDAS DE LOS PANELES (y el mapa DEBAJO de ellos) -------------------------------
	//
	// El HUD y el minimapa se pintan otra vez en el mismo sitio de la pantalla, asi que el interior de su caja
	// parece no haber que tocarlo. Lo que asoma es el borde del panel del cuadro anterior, corrido (sx, sy): la
	// banda que el panel de ahora NO tapa se repinta con el mapa (ver `agregarBanda141`).
	//
	// Y HAY QUE REPINTAR TAMBIEN EL INTERIOR, y esto se midio (revision de esta tarjeta): los paneles NO son
	// opacos. `hudPiezas=esenciales` no carga el panel de fondo, asi que de la banda del HUD lo unico que se pinta
	// son los globos (5024 de 6400 pixeles: 1376 agujeros), el cinturon (4000 de 4000, ese si opaco) y los textos
	// de la zona; el resto de la banda es MUNDO A LA VISTA. Con el corrimiento, los agujeros de un globo dejan ver
	// el globo del cuadro anterior corrido unos pixeles, y el texto de la zona deja su fantasma. Comparando el
	// cuadro terminado contra el repintado entero eso son 74 de 300 bloques distintos, TODOS adentro de las cajas
	// de los paneles. Repintando el mapa dentro de la caja, los agujeros muestran mapa limpio y el fantasma
	// desaparece: el cuadro queda igual al repintado entero (medido: 0 bloques distintos de 300).
	//
	// Las tres cajas son las mismas que marca DC-002.139 para la presentacion por bloques (medidas en la
	// captura de DC-002.140): la franja del HUD de arriba (0,0)-(640,64), la de abajo (0,400)-(640,480) y la
	// caja del minimapa (350,48)-(640,290), 279x225 pegada a la esquina de arriba a la derecha.
	paneles += agregarCajaDelPanel141(malla, 0, 0, ancho, altoDelHUD141, sx, sy)
	paneles += agregarCajaDelPanel141(malla, 0, alto-altoDelPie141, ancho, alto, sx, sy)
	paneles += agregarCajaDelPanel141(malla, ancho-anchoDelMinimapa141, arribaDelMinimapa141,
		ancho, abajoDelMinimapa141, sx, sy)

	cuantos = malla.RecortesPuestos()

	// la lista se guarda para el diagnostico: cuando la comparacion falla hay que saber que se repinto. Se copia
	// AHORA porque la referencia vacia la lista del lienzo.
	cuantosRecortesDelCaminando141 = cuantos

	for k := 0; k < cuantos && k < len(recortesDelCaminando141); k++ {
		x0, y0, x1, y1 := malla.RecorteCargado141(k)
		recortesDelCaminando141[k][0], recortesDelCaminando141[k][1] = x0, y0
		recortesDelCaminando141[k][2], recortesDelCaminando141[k][3] = x1, y1
	}

	// y cuantos hubo que descartar por no entrar en la lista (0 es lo esperado, ver `recortesMax141`): el numero
	// va a la linea del censo, porque un recorte de mas deja el lienzo mal sin romper nada visible en el log.
	scroll141RecortesIgnorados = malla.RecortesIgnorados141()

	return franjas, cajas, paneles, cuantos
}

// Las tres cajas de los paneles: las mismas que marca DC-002.139 para la presentacion por bloques (medidas en
// la captura de DC-002.140: la franja del HUD de arriba, la de abajo y la caja del minimapa, 279x225 pegada a
// la esquina de arriba a la derecha).
const (
	altoDelHUD141 = 64
	// altoDelPie141: la banda del pie, DESDE DONDE HAY QUE REPINTAR EL MAPA. NO son los 80 del globo: los globos
	// se pegan con `globoVidaDY`/`globoManaDY` = -13, asi que su caja arranca en 480-13-80 = 387 y sobresale 13
	// px POR ENCIMA de los 400. Medido (revision de esta tarjeta): con la banda en 400, esa franja de 13 px
	// quedaba con mapa sucio y el corrimiento la arrastraba a otro lado del lienzo -- 3 bloques distintos de 300,
	// justo el fantasma que la comparacion del cuadro terminado caza. 96 = 3 bloques de 32, arranca en 384.
	altoDelPie141        = 96
	anchoDelMinimapa141  = 290
	arribaDelMinimapa141 = 48
	abajoDelMinimapa141  = 290
	// la caja MEDIDA del panel del minimapa (DC-002.140), la que esta tapada seguro
	minimapa141X0 = 359
	minimapa141Y0 = 57
	minimapa141X1 = 638
	minimapa141Y1 = 282
	// ladoDelBloque141: el lado del bloque de la presentacion y de la comparacion (el mismo de `sucios_139.go`).
	ladoDelBloque141 = 32
)

// agregarFranjaVertical141: la franja [x0,x1) que dejo libre la copia, solo en la parte que los paneles tapan
// de verdad: entre el HUD de arriba (0..64) y el pie (400..480), y esquivando el minimapa.
//
// EL MINIMAPA VA CON SU CAJA MEDIDA (359,57)-(638,282), no con la caja con aire que usa DC-002.139 para marcar
// bloques (350,48)-(640,290): lo que se descuenta tiene que estar tapado SEGURO. El aire alrededor del panel se
// repinta igual, asi que un par de pixeles de error de medicion no dejan una costura a la vista.
//
// Son hasta tres rectangulos y no se pisan: el trozo a la izquierda del minimapa (de arriba abajo), el que queda
// debajo del minimapa y el borde derecho del panel.
func agregarFranjaVertical141(malla *pvr.Surface, x0, x1 int) int {
	px := 0

	if x0 < minimapa141X0 {
		px += agregarRecorte141(malla, x0, altoDelHUD141, minDel141(x1, minimapa141X0), alto-altoDelPie141)
	}

	if x1 > minimapa141X0 {
		px += agregarRecorte141(malla, maxDel141(x0, minimapa141X0), minimapa141Y1, x1, alto-altoDelPie141)
	}

	if x1 > minimapa141X1 {
		px += agregarRecorte141(malla, maxDel141(x0, minimapa141X1), altoDelHUD141, x1, minimapa141Y1)
	}

	return px
}

// minDel141/maxDel141: el minimo y el maximo, que en este archivo se usan para partir rectangulos.
func minDel141(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func maxDel141(a, b int) int {
	if a > b {
		return a
	}

	return b
}

// agregarCaja141: pide repintar el mapa bajo la caja del cuadro anterior corrida con la copia, con un pixel de
// aire a cada lado (un sprite puede tocar el borde de su caja por redondeo). Devuelve los pixeles pedidos.
func agregarCaja141(malla *pvr.Surface, c CajaDelJugador, sx, sy int) int {
	if c.Ancho() <= 0 || c.Alto() <= 0 {
		return 0
	}

	x0, y0, x1, y1 := c.X0+sx-1, c.Y0+sy-1, c.X1+sx+1, c.Y1+sy+1

	return agregarRecorte141(malla, x0, y0, x1, y1)
}

// agregarBanda141: la parte del panel (x0,y0,x1,y1) del cuadro anterior -- corrido (sx, sy) -- que el panel de
// ahora NO tapa. Son dos rectangulos: la banda horizontal que se corrio y la vertical.
//
// EJEMPLO: un panel de (0,0)-(640,64) con la camara corriendo el mundo (10, 5): el panel viejo quedo en
// (10,5)-(650,69), asi que asoma una franja de 5 pixeles abajo (x 10..640) y nada a la derecha (se sale del
// lienzo). Esa franja es la que hay que devolverle al mapa.
func agregarBanda141(malla *pvr.Surface, x0, y0, x1, y1, sx, sy int) int {
	px := 0

	if sy != 0 {
		if sy > 0 {
			px += agregarRecorte141(malla, x0+sx, y1, x1+sx, y1+sy)
		} else {
			px += agregarRecorte141(malla, x0+sx, y0+sy, x1+sx, y0)
		}
	}

	if sx != 0 {
		if sx > 0 {
			px += agregarRecorte141(malla, x1, y0, x1+sx, y1)
		} else {
			px += agregarRecorte141(malla, x0+sx, y0, x0, y1)
		}
	}

	return px
}

// agregarCajaDelPanel141: devuelve al mapa la caja del panel ENTERA -- porque los paneles NO son opacos: por sus
// agujeros (los pixeles de indice 0 del arte) se ve el mundo, y el mundo tiene que estar limpio -- MAS la banda
// que asoma del panel del cuadro anterior, corrido (sx, sy), que es lo que hace `agregarBanda141`.
func agregarCajaDelPanel141(malla *pvr.Surface, x0, y0, x1, y1, sx, sy int) int {
	px := agregarRecorte141(malla, x0, y0, x1, y1)

	return px + agregarBanda141(malla, x0, y0, x1, y1, sx, sy)
}

// agregarRecorte141: pide un rectangulo al lienzo y cuenta los pixeles que de verdad caen adentro.
func agregarRecorte141(malla *pvr.Surface, x0, y0, x1, y1 int) int {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > ancho {
		x1 = ancho
	}
	if y1 > alto {
		y1 = alto
	}

	if x0 >= x1 || y0 >= y1 {
		return 0
	}

	malla.RecortarA(x0, y0, x1, y1)

	return (x1 - x0) * (y1 - y0)
}

// bloquesDeLaComparacion141: la grilla de la comparacion (640x480 en bloques de 32x32 = 20x15 = 300; se deja
// aire para lienzos mas grandes, que es lo mismo que hace el presentador).
const bloquesDeLaComparacion141 = 400

// huellaDelBloque141: la huella de un bloque de 32x32, fila por fila, con un crc32 por fila mezclado a mano.
//
// POR QUE NO ES UN crc32 CORRIDO. gccgo (el compilador de este POC, para SH-4) no trae `crc32.Update`; con
// `crc32.ChecksumIEEE` por fila y una mezcla FNV entre filas alcanza y sobra: la huella solo tiene que decir
// "este bloque es igual a aquel", no detectar corrupcion en un cable.
func huellaDelBloque141(pixeles []byte, bx, by int) uint32 {
	suma := uint32(2166136261)

	for y := by; y < by+ladoDelBloque141; y++ {
		i := (y*ancho + bx) * 4
		suma = (suma ^ crc32.ChecksumIEEE(pixeles[i:i+ladoDelBloque141*4])) * 16777619
	}

	return suma
}

// crcerLosBloques141: la huella de cada bloque de 32x32 del lienzo, en el orden de la grilla (fila por fila).
//
// Se usa un crc32 por fila mezclado a mano (ver `huellaDelBloque141`) y no un objeto `hash` por bloque: en el
// semiespacio de 1,7 MB del POC no se pide memoria por bloque (300 bloques x 2 pasadas x 8 cuadros = 4.800
// pedidos, y el recolector no da abasto).
func crcerLosBloques141(pixeles []byte, crcs *[bloquesDeLaComparacion141]uint32) int {
	cuantos := 0

	for by := 0; by+ladoDelBloque141 <= alto; by += ladoDelBloque141 {
		for bx := 0; bx+ladoDelBloque141 <= ancho; bx += ladoDelBloque141 {
			suma := huellaDelBloque141(pixeles, bx, by)

			crcs[cuantos] = suma
			cuantos++
		}
	}

	return cuantos
}

// contarBloquesDistintos141: cuantos bloques del lienzo cambiaron respecto de `antes`, la caja que los contiene (en
// pixeles) y el indice del primero. Es el diagnostico que dice DONDE se rompio el cuadro caminando: una caja pegada
// a un borde es una franja que no se pinto, una caja alrededor del jugador es un fantasma.
//
// SE MIRAN LOS 300 BLOQUES, SIN EXCEPCIONES. La version anterior de esta prueba salteaba los bloques que tocaban
// las cajas de los paneles (120 comparables de 300, el 60 % del lienzo sin prueba) porque comparaba el mapa ANTES
// de los paneles. Ahora se compara el cuadro TERMINADO (ver `compararElCuadroTerminado141`), asi que adentro de los
// paneles los dos caminos tienen el MISMO panel encima y tambien se comparan: el numero que vale es 300 de 300.
func contarBloquesDistintos141(pixeles []byte, antes *[bloquesDeLaComparacion141]uint32, cuantos int) (distintos, comparables, bx0, by0, bx1, by1, primero int) {
	columnas := ancho / ladoDelBloque141

	bx0, by0, bx1, by1, primero = -1, -1, -1, -1, -1

	k := 0

	for by := 0; by+ladoDelBloque141 <= alto; by += ladoDelBloque141 {
		for bx := 0; bx+ladoDelBloque141 <= ancho; bx += ladoDelBloque141 {
			if k >= cuantos {
				return distintos, comparables, bx0, by0, bx1, by1, primero
			}

			comparables++

			suma := huellaDelBloque141(pixeles, bx, by)

			if suma != antes[k] {
				distintos++

				if primero < 0 {
					primero = by*columnas + bx
				}

				if bx0 < 0 || bx < bx0 {
					bx0 = bx
				}
				if by0 < 0 || by < by0 {
					by0 = by
				}
				if bx+ladoDelBloque141 > bx1 {
					bx1 = bx + ladoDelBloque141
				}
				if by+ladoDelBloque141 > by1 {
					by1 = by + ladoDelBloque141
				}
			}

			k++
		}
	}

	return distintos, comparables, bx0, by0, bx1, by1, primero
}

// ============================== LA PRUEBA: EL CUADRO TERMINADO ==============================

// comparacionesDelCuadroTerminado141: cuantas comparaciones por CORRIMIENTO (por cuadrante de signos) en la
// corrida de la prueba. Cada una cuesta un cuadro entero de referencia (el mapa ENTERO, los entes y los paneles:
// ~100 ms), asi que solo corre con `COMPARAR_141=si`, nunca en la corrida del criterio.
const comparacionesDelCuadroTerminado141 = 2

// cuadranteDelCorrimiento141: el codigo de signos de (sx, sy): 1 = +x, 2 = -x, 4 = +y, 8 = -y, sumados. Es lo
// que decide si un cuadro caminando prueba una rama que todavia nadie probo (ver `scroll141Cuadrante`).
func cuadranteDelCorrimiento141(sx, sy int) int {
	q := 0

	if sx > 0 {
		q |= 1
	} else if sx < 0 {
		q |= 2
	}

	if sy > 0 {
		q |= 4
	} else if sy < 0 {
		q |= 8
	}

	return q
}

// compararElCuadroTerminado141: LA PRUEBA DE QUE EL CUADRO CAMINANDO ES EL MISMO CUADRO.
//
// QUE SE COMPARA: el cuadro TERMINADO (mapa + entes + HUD + minimapa) contra el MISMO cuadro repintado entero, en
// el mismo lienzo y en el mismo cuadro del juego. Bloque por bloque de 32x32, los 300 (640x480), sin una sola
// excepcion: si el cuadro de la copia y el repintado entero dan lo mismo, `igual=` es true y `bloquesDistintos= 0`.
//
// POR QUE EL CUADRO TERMINADO Y NO EL MAPA PELADO (la correccion de la revision de DC-002.141). Comparando antes
// de los paneles habia que saltear los bloques que tocaban las cajas de los paneles -- ahi adentro el cuadro que
// camina todavia tiene el fantasma corrido del panel anterior, que el panel de este cuadro tapa. Eso dejaba 120
// de 300 bloques comparados y 180 sin ninguna prueba, justo donde la opacidad de los paneles era una medicion de
// captura y no una prueba. Con los paneles encima, adentro de las cajas los dos caminos tienen el mismo panel: se
// comparan los 300.
//
// LA REFERENCIA se arma con el MISMO orden del cuadro (mapa entero, entes, paneles) y en el mismo lienzo, asi que
// no hace falta ninguna copia del lienzo (que no entra: el semiespacio del POC son 1,7 MB y el lienzo 1,2 MB).
// Los contadores de los paneles se devuelven a como estaban: el cuadro de referencia NO es un cuadro de la corrida.
//
// CUANDO NO SE COMPARA: con un panel ABIERTO (el panel del dialogo o el inventario se pintan por su cuenta y no son
// parte del camino del mapa; se cuentan en `scroll141ComparacionesSaltadas`), y nunca en un cuadro que no camina.
func compararElCuadroTerminado141(malla *pvr.Surface, superficie d2interface.Surface) {
	if comparar141 != "si" || !scroll141Caminando || rendererDelMapa == nil {
		return
	}

	if dialogoAbierto || elInventarioEstaAbierto124() {
		scroll141ComparacionesSaltadas++

		return
	}

	// la cobertura de los signos: dos cuadros por cuadrante, y un cuadrante nuevo siempre entra
	cuadrante := cuadranteDelCorrimiento141(scroll141SX, scroll141SY)

	if cuadrante != scroll141Cuadrante {
		scroll141Cuadrante, scroll141EnCuadrante = cuadrante, 0
		scroll141Cuadrantes |= 1 << cuadrante
	}

	if scroll141EnCuadrante >= comparacionesDelCuadroTerminado141 {
		return
	}

	scroll141EnCuadrante++
	scroll141Comparaciones++

	pixeles := malla.Imagen().Pixeles()

	// El crc32 del cuadro terminado, ANTES de la referencia: `crcLienzo` devuelve 0 cuando el instrumento por
	// cuadro esta apagado (`medirCuadro=no`), y esta prueba tiene que valer en el binario del criterio tambien.
	crcCaminando := crc32.ChecksumIEEE(pixeles)

	var antes [bloquesDeLaComparacion141]uint32

	cuantosBloques := crcerLosBloques141(pixeles, &antes)

	sumarLosBloques141(pixeles)

	guardarLasSondas141(pixeles)

	// ---- LA REFERENCIA: el mismo cuadro, pintado entero ----
	malla.SinRecortes()

	tEntero := d2cUs()

	rendererDelMapa.Render(superficie)

	usMapaEntero := int(d2cUs() - tEntero)

	tEntes := d2cUs()

	pintandoLaReferencia141 = true

	volverAPintarAJugadores(malla)
	volverAPintarLosBichos(malla)
	volverAPintarLosItems(malla)
	volverAPintarLosNPCs(malla)

	pintandoLaReferencia141 = false

	usEntes := int(d2cUs() - tEntes)

	usPaneles := dibujarLosPanelesDeLaReferencia141(malla)

	malla.SinRecortes()

	crcEntero := crc32.ChecksumIEEE(pixeles)

	distintos, comparables, bx0, by0, bx1, by1, primero := contarBloquesDistintos141(pixeles, &antes, cuantosBloques)

	pxDistintos, filasDistintas, primerasColumnas := diagnosticoDeLaComparacion141(pixeles, &antes, cuantosBloques)

	println("JUEGO141: COMPARACION cuadro=", cuadro, "sx=", scroll141SX, "sy=", scroll141SY,
		"cuadrante=", scroll141Cuadrante,
		"crcCaminando=", int(crcCaminando), "crcEntero=", int(crcEntero), "igual=", crcCaminando == crcEntero,
		"bloquesDistintos=", distintos, "deComparables=", comparables,
		"cajaDistinta=", bx0, by0, bx1, by1, "primerBloque=", primero,
		"distintosPorFila=", pxDistintos, "distintosPorColumna=", filasDistintas,
		"enPaneles=", primerasColumnas,
		"usMapaEntero=", usMapaEntero, "usEntes=", usEntes, "usPaneles=", usPaneles)

	// cuando falla, QUE hay en la caja que cambio: los entes que la cruzan y cuanto cambia cada bloque. Es el
	// dato que dice si el fantasma es de un sprite, de un panel o del mapa.
	if distintos > 0 {
		println("JUEGO141: COMPARACION-DETALLE cuadro=", cuadro, "entes=", entesEnLaCaja141(bx0, by0, bx1, by1))

		println("JUEGO141: COMPARACION-BLOQUES cuadro=", cuadro, "sumas=", sumasDeLosBloques141(pixeles, &antes, cuantosBloques))

		println("JUEGO141: COMPARACION-SONDAS cuadro=", cuadro, "fila=", sondasY141,
			"pixeles=", lasSondasQueDifieren141(pixeles))

		println("JUEGO141: COMPARACION-RECORTES cuadro=", cuadro, "sx=", scroll141SX, "sy=", scroll141SY,
			"lista=", losRecortesDelCuadro141(malla))
	}
}

// dibujarLosPanelesDeLaReferencia141: el HUD y el minimapa del cuadro de referencia, en el orden del cuadro, y con
// los contadores devueltos a como estaban (el cuadro de referencia es evidencia, no un cuadro de la corrida: no
// tiene que mover ni un promedio). Devuelve los microsegundos que costo pintarlos.
func dibujarLosPanelesDeLaReferencia141(malla *pvr.Surface) int {
	t0 := d2cUs()

	if hudKnob == "si" && elHUD != nil {
		px, ms, cuadrosHUD := elHUD.pxPintados, elHUD.msDibujo, elHUD.cuadrosConHUD

		// el cuadro pinto el HUD con las barras de ANTES de bajarlas: la referencia pinta con esas mismas, y la
		// bajada del instrumento vuelve a dejarlas donde las dejo el cuadro (ver `guardarLasBarrasDelCuadro141`).
		restaurarLasBarras141(vidaActual, barrasDelCuadro141[0])
		restaurarLasBarras141(manaActual, barrasDelCuadro141[1])

		elHUD.dibujar(malla, 0, ancho, alto)

		elHUD.pxPintados, elHUD.msDibujo, elHUD.cuadrosConHUD = px, ms, cuadrosHUD
	}

	if minimapaKnob == "si" && elMinimapa != nil {
		antes := copiaDeLosContadoresDelMinimapa141(elMinimapa)

		// el camino "cambio" del minimapa (`minimapaCuando`) se salta el dibujo si el heroe no se movio en la
		// caja y el modelo no crecio: en la referencia hay que pintarlo igual, o la referencia quedaria con mapa
		// donde va el minimapa. Se apaga el disparador del salto (y se devuelve con los contadores).
		elMinimapa.hayDibujo = false

		elMinimapa.dibujarElMinimapa(malla, 0, ancho, alto)

		restaurarLosContadoresDelMinimapa141(elMinimapa, antes)
	}

	return int(d2cUs() - t0)
}

// copiaDeLasBarras141 / restaurarLasBarras141: la vida y el mana de todos los jugadores, para poder pintar el HUD
// dos veces en el mismo cuadro sin que la segunda salga con las barras un punto mas abajo.
func copiaDeLasBarras141(v []int) [4]int {
	var c [4]int

	for i := 0; i < len(v) && i < len(c); i++ {
		c[i] = v[i]
	}

	return c
}

func restaurarLasBarras141(v []int, c [4]int) {
	for i := 0; i < len(v) && i < len(c); i++ {
		v[i] = c[i]
	}
}

// guardarLasBarrasDelCuadro141: la vida y el mana de todos los jugadores ANTES de que el HUD de este cuadro los
// baje. Se llama en `main.go` justo antes de `elHUD.dibujar`; lo usa el cuadro de referencia de la comparacion.
func guardarLasBarrasDelCuadro141() {
	barrasDelCuadro141[0] = copiaDeLasBarras141(vidaActual)
	barrasDelCuadro141[1] = copiaDeLasBarras141(manaActual)
}

// contadoresDelMinimapa141: lo que un dibujo del minimapa mueve (los contadores del cuadro, los maximos, los
// promedios y los acumuladores del instrumento). Se guarda y se devuelve entero: la referencia no cuenta.
type contadoresDelMinimapa141 struct {
	cuadrosDibujados, celdasUltimas, celdasMax, pxUltimos, pxTotal int
	msUltimo, msTotal, msMax, saltados, sinArte                    int
	hayDibujo, ultimoHeroeX, ultimoHeroeY, ultimasCeldasDib        int
	usTabla, usCaja, usCeldas, usMarcador, usExtra                 int64
	delTramo, celdasDelTramo, pixelesDelTramo                      int
}

func copiaDeLosContadoresDelMinimapa141(m *minimapaDelJuego) contadoresDelMinimapa141 {
	c := contadoresDelMinimapa141{
		cuadrosDibujados: m.cuadrosDibujados, celdasUltimas: m.celdasUltimas, celdasMax: m.celdasMax,
		pxUltimos: m.pxUltimos, pxTotal: m.pxTotal, msUltimo: m.msUltimo, msTotal: m.msTotal, msMax: m.msMax,
		saltados: m.saltados, sinArte: m.SinArte,
		ultimoHeroeX: m.ultimoHeroeX, ultimoHeroeY: m.ultimoHeroeY, ultimasCeldasDib: m.ultimasCeldasDib,
		usTabla: usMiniTabla137, usCaja: usMiniCaja137, usCeldas: usMiniCeldas137, usMarcador: usMiniMarcador137,
		usExtra: usMiniExtra137, delTramo: cuadrosMini137, celdasDelTramo: celdasMini137,
		pixelesDelTramo: pixelesMini137,
	}

	if m.hayDibujo {
		c.hayDibujo = 1
	}

	return c
}

func restaurarLosContadoresDelMinimapa141(m *minimapaDelJuego, c contadoresDelMinimapa141) {
	m.cuadrosDibujados, m.celdasUltimas, m.celdasMax = c.cuadrosDibujados, c.celdasUltimas, c.celdasMax
	m.pxUltimos, m.pxTotal, m.msUltimo, m.msTotal, m.msMax = c.pxUltimos, c.pxTotal, c.msUltimo, c.msTotal, c.msMax
	m.saltados, m.SinArte = c.saltados, c.sinArte
	m.hayDibujo = c.hayDibujo != 0
	m.ultimoHeroeX, m.ultimoHeroeY, m.ultimasCeldasDib = c.ultimoHeroeX, c.ultimoHeroeY, c.ultimasCeldasDib

	usMiniTabla137, usMiniCaja137, usMiniCeldas137 = c.usTabla, c.usCaja, c.usCeldas
	usMiniMarcador137, usMiniExtra137 = c.usMarcador, c.usExtra
	cuadrosMini137, celdasMini137, pixelesMini137 = c.delTramo, c.celdasDelTramo, c.pixelesDelTramo
}

// diagnosticoDeLaComparacion141: DONDE estan los bloques distintos, para no tener que adivinar cuando la
// comparacion falla. Devuelve tres textos: los bloques distintos por fila de la grilla ("3:20 12:57 ..."), por
// columna, y cuantos de ellos caen en las cajas de los paneles (donde el cuadro que camina tiene el fantasma del
// panel anterior y el repintado entero tiene mapa limpio).
func diagnosticoDeLaComparacion141(pixeles []byte, antes *[bloquesDeLaComparacion141]uint32, cuantos int) (porFila, porColumna, enPaneles string) {
	columnas := ancho / ladoDelBloque141
	filas := alto / ladoDelBloque141

	var cuentaFilas, cuentaColumnas [24]int

	dentro := 0

	k := 0

	for by := 0; by+ladoDelBloque141 <= alto; by += ladoDelBloque141 {
		f := by / ladoDelBloque141

		for bx := 0; bx+ladoDelBloque141 <= ancho; bx += ladoDelBloque141 {
			if k >= cuantos {
				break
			}

			if huellaDelBloque141(pixeles, bx, by) != antes[k] {
				if f < len(cuentaFilas) {
					cuentaFilas[f]++
				}

				c := bx / ladoDelBloque141
				if c < len(cuentaColumnas) {
					cuentaColumnas[c]++
				}

				if enLaCajaDelPanel141(bx, by) {
					dentro++
				}
			}

			k++
		}
	}

	porFila = ""
	for f := 0; f < filas && f < len(cuentaFilas); f++ {
		if cuentaFilas[f] == 0 {
			continue
		}

		if porFila != "" {
			porFila += " "
		}

		porFila += itoa(f) + ":" + itoa(cuentaFilas[f])
	}

	porColumna = ""
	for c := 0; c < columnas && c < len(cuentaColumnas); c++ {
		if cuentaColumnas[c] == 0 {
			continue
		}

		if porColumna != "" {
			porColumna += " "
		}

		porColumna += itoa(c) + ":" + itoa(cuentaColumnas[c])
	}

	enPaneles = itoa(dentro) + "-de-" + itoa(k)

	return porFila, porColumna, enPaneles
}

// enLaCajaDelPanel141: si el bloque de 32x32 en (x0,y0) cae en alguna de las tres cajas de los paneles. Es SOLO
// diagnostico (la comparacion compara los 300 bloques): sirve para leer un fallo y saber si esta donde el cuadro
// que camina no repinta el mapa porque el panel de este cuadro lo tapa.
func enLaCajaDelPanel141(x0, y0 int) bool {
	x1, y1 := x0+ladoDelBloque141, y0+ladoDelBloque141

	if y0 < altoDelHUD141 {
		return true
	}

	if y1 > alto-altoDelPie141 {
		return true
	}

	if x1 > ancho-anchoDelMinimapa141 && y1 > arribaDelMinimapa141 && y0 < abajoDelMinimapa141 {
		return true
	}

	return false
}

// sumasDelCuadro141: la suma de los bytes de color de cada bloque de 32x32 del cuadro que camina. Es SOLO
// diagnostico (ver `sumasDeLosBloques141`): cuando la comparacion falla, cuanto cambio cada bloque dice si la
// diferencia es un pixel suelto o un pedazo grande. Vive en el `.bss`, no en el monton: son 1,6 KB que no tocan
// el semiespacio del recolector.
var sumasDelCuadro141 [bloquesDeLaComparacion141]uint32

// sumarLosBloques141: llena `sumasDelCuadro141` con el cuadro que camina, antes de pintar la referencia.
func sumarLosBloques141(pixeles []byte) {
	k := 0

	for by := 0; by+ladoDelBloque141 <= alto; by += ladoDelBloque141 {
		for bx := 0; bx+ladoDelBloque141 <= ancho; bx += ladoDelBloque141 {
			if k >= len(sumasDelCuadro141) {
				return
			}

			suma := uint32(0)

			for y := by; y < by+ladoDelBloque141; y++ {
				i := (y*ancho + bx) * 4

				for x := 0; x < ladoDelBloque141*4; x++ {
					suma += uint32(pixeles[i+x])
				}
			}

			sumasDelCuadro141[k] = suma
			k++
		}
	}
}

// sumasDeLosBloques141: los primeros bloques distintos con su suma en los dos cuadros ("(224,384)caminando:entero").
func sumasDeLosBloques141(pixeles []byte, antes *[bloquesDeLaComparacion141]uint32, cuantos int) string {
	s := ""

	columnas := ancho / ladoDelBloque141
	k := 0

	for by := 0; by+ladoDelBloque141 <= alto; by += ladoDelBloque141 {
		for bx := 0; bx+ladoDelBloque141 <= ancho; bx += ladoDelBloque141 {
			if k >= cuantos {
				return s
			}

			if huellaDelBloque141(pixeles, bx, by) != antes[k] {
				if s != "" {
					s += " "
				}

				entero := uint32(0)

				for y := by; y < by+ladoDelBloque141; y++ {
					i := (y*ancho + bx) * 4

					for x := 0; x < ladoDelBloque141*4; x++ {
						entero += uint32(pixeles[i+x])
					}
				}

				s += "(" + itoa(bx) + "," + itoa(by) + "," + itoa(by*columnas+bx) + ")" +
					itoa(int(sumasDelCuadro141[k])) + ":" + itoa(int(entero))
			}

			k++
		}
	}

	return s
}

// sondasDelCuadro141: una fila de bloques (32 px de alto, ancho entero) del cuadro que camina, copiada antes de
// pintar la referencia. Es SOLO diagnostico de `COMPARAR_141`: cuando la comparacion falla sirve para imprimir
// los pixeles que difieren, con su valor en los dos cuadros. 82 KB de `.bss` que no tocan el monton del
// recolector, y fuera de la palanca no se copia nada.
const sondasY141 = 256

var sondasDelCuadro141 [ladoDelBloque141 * 640]uint32

// guardarLasSondas141: copia la fila de bloques `sondasY141` del cuadro que camina.
func guardarLasSondas141(pixeles []byte) {
	i := sondasY141 * ancho * 4

	for k := 0; k < len(sondasDelCuadro141); k++ {
		sondasDelCuadro141[k] = uint32(pixeles[i]) | uint32(pixeles[i+1])<<8 |
			uint32(pixeles[i+2])<<16 | uint32(pixeles[i+3])<<24
		i += 4
	}
}

// lasSondasQueDifieren141: el resumen de la fila de sondas, linea por linea: cuantos pixeles cambiaron, desde
// donde hasta donde, y los dos primeros valores. Alcanza para saber si la diferencia es una linea de borde (un
// pixel de alto de punta a punta) o un pedazo grande.
func lasSondasQueDifieren141(pixeles []byte) string {
	s := ""
	i := sondasY141 * ancho * 4

	for fila := 0; fila < ladoDelBloque141; fila++ {
		cuantos := 0
		desde := -1
		hasta := -1
		ejemplo := ""

		for x := 0; x < ancho; x++ {
			ahora := uint32(pixeles[i]) | uint32(pixeles[i+1])<<8 | uint32(pixeles[i+2])<<16 | uint32(pixeles[i+3])<<24
			k := fila*ancho + x

			if ahora != sondasDelCuadro141[k] {
				cuantos++

				if desde < 0 {
					desde = x

					ejemplo = itoa(int(sondasDelCuadro141[k])) + ":" + itoa(int(ahora))
				}

				hasta = x
			}

			i += 4
		}

		if cuantos == 0 {
			continue
		}

		if s != "" {
			s += " | "
		}

		s += "fila" + itoa(fila) + "(y" + itoa(sondasY141+fila) + ")=" + itoa(cuantos) +
			"px x" + itoa(desde) + ".." + itoa(hasta) + " " + ejemplo
	}

	if s == "" {
		return "ninguno"
	}

	return s
}

// losRecortesDelCuadro141: la lista de recortes que se puso el cuadro que camina, como texto. SOLO diagnostico:
// se guarda una copia en `armarLosRecortes141` porque la referencia vacia la lista del lienzo.
var recortesDelCaminando141 [32][4]int
var cuantosRecortesDelCaminando141 int

func losRecortesDelCuadro141(malla *pvr.Surface) string {
	s := ""

	for k := 0; k < cuantosRecortesDelCaminando141 && k < len(recortesDelCaminando141); k++ {
		r := recortesDelCaminando141[k]

		if s != "" {
			s += " "
		}

		s += "(" + itoa(r[0]) + "," + itoa(r[1]) + "," + itoa(r[2]) + "," + itoa(r[3]) + ")"
	}

	if s == "" {
		return "vacia"
	}

	return s
}

// entesEnLaCaja141: las cajas de los entes que cruzan la caja (x0,y0)-(x1,y1). Es el diagnostico que dice si el
// bloque que cambio esta sobre un sprite (y entonces el fantasma es del sprite) o sobre el mapa pelado.
func entesEnLaCaja141(x0, y0, x1, y1 int) string {
	s := ""

	for i, p := range jugadores {
		if cajaCruza141(p.Caja, x0, y0, x1, y1) {
			s += "jugador" + itoa(i+1) + "(" + itoa(p.Caja.X0) + "," + itoa(p.Caja.Y0) + "," +
				itoa(p.Caja.X1) + "," + itoa(p.Caja.Y1) + ") "
		}
	}

	for i, b := range bichos {
		if cajaCruza141(b.Caja, x0, y0, x1, y1) {
			s += "bicho" + itoa(i) + "(" + itoa(b.Caja.X0) + "," + itoa(b.Caja.Y0) + "," +
				itoa(b.Caja.X1) + "," + itoa(b.Caja.Y1) + ") "
		}
	}

	for i, n := range npcsDelPueblo {
		if cajaCruza141(n.Caja, x0, y0, x1, y1) {
			s += "npc" + itoa(i) + "(" + itoa(n.Caja.X0) + "," + itoa(n.Caja.Y0) + "," +
				itoa(n.Caja.X1) + "," + itoa(n.Caja.Y1) + ") "
		}
	}

	for i, it := range suelo {
		if cajaCruza141(it.Caja, x0, y0, x1, y1) {
			s += "item" + itoa(i) + "(" + itoa(it.Caja.X0) + "," + itoa(it.Caja.Y0) + "," +
				itoa(it.Caja.X1) + "," + itoa(it.Caja.Y1) + ") "
		}
	}

	if s == "" {
		return "ninguno"
	}

	return s
}

// cajaCruza141: si la caja del ente toca el rectangulo (x0,y0)-(x1,y1).
func cajaCruza141(c CajaDelJugador, x0, y0, x1, y1 int) bool {
	if c.Ancho() <= 0 || c.Alto() <= 0 {
		return false
	}

	return c.X0 < x1 && c.X1 > x0 && c.Y0 < y1 && c.Y1 > y0
}

// informeDelCuadroQueCamina141: el censo del tramo, pegado a la linea del mapa estatico (cada `cadaInforme143`).
func informeDelCuadroQueCamina141() {
	if cuadro%cadaInforme143() != 0 {
		return
	}

	desp, ent, qui := scroll141Desplazados, scroll141Enteros, scroll141Quietos

	// el censo de los blits del renderer: cuantos tiles se piden por cuadro y cuantos se saltan por no tocar
	// ningun recorte. Si "pedidos" es 241x4 y "pintados" unos pocos, el costo esta en el aparato por tile.
	pedidos, saltados, pintados := pvr.CensoDeLosBlits141()
	pintadosPx := pvr.CensoDeLosPixeles141()

	// la conversion del lienzo: con la verificacion puesta, cuantos bloques se convirtieron por los DOS caminos
	// y cuantas palabras de 16 bits no coincidieron (cero es el unico resultado aceptable).
	bloquesConv, fallosConv := pvr.CensoDeLaConversion141()

	if comparar141 == "si" {
		println("JUEGO141: CONVERSION cuadros=", cuadro, "bloques=", bloquesConv, "palabrasDistintas=", fallosConv,
			"porFilas=", convFilas141)

		println("JUEGO141: COMPARACIONES cuadros=", cuadro, "hechas=", scroll141Comparaciones,
			"cuadrantes=", scroll141Cuadrantes, "saltadasPorPanel=", scroll141ComparacionesSaltadas,
			"porCuadrante=", comparacionesDelCuadroTerminado141)
	}

	println("JUEGO141: LIENZO-CAMINA cuadros=", cuadro, "desplazados=", desp, "enteros=", ent, "quietos=", qui,
		"fallos=", scroll141Fallos, "palanca=", scroll141,
		"usMover=", scroll141UsMover, "usFranjas=", scroll141UsFranjas,
		"pxFranjas=", scroll141PxFranjas, "pxCajas=", scroll141PxCajas, "pxPaneles=", scroll141PxPaneles,
		"recortes=", scroll141Recortes, "maxSX=", scroll141MaxSX, "maxSY=", scroll141MaxSY,
		"recortesIgnorados=", scroll141RecortesIgnorados)

	// DC-002.142: la presentacion de esos mismos cuadros caminando, por dentro (convertir / subir / pintar) y
	// con cuantas copias a la VRAM costo. Es el numero que dice si el anillo de textura hace falta.
	println("JUEGO142: PRESENTACION-CAMINANDO tramo=", cuadro, "grupo=", grupoDelBinario142(),
		promedioPresentar142())

	reiniciarPresentar142()

	// el promedio por cuadro caminando: es el numero que se compara contra los 60,4 ms del repintado entero
	if desp > 0 {
		println("JUEGO141: BLITS pedidos=", pedidos, "saltados=", saltados, "pintados=", pintados,
			"pixeles=", pintadosPx, "porCuadro=", pintadosPx/desp, "cuadrosDesplazados=", desp)

		println("JUEGO141: LIENZO-CAMINA PROMEDIO porCuadro mover_us=", scroll141UsMover/desp,
			"franjas_us=", scroll141UsFranjas/desp, "pxFranjas=", scroll141PxFranjas/desp,
			"pxCajas=", scroll141PxCajas/desp, "pxPaneles=", scroll141PxPaneles/desp,
			"recortes=", scroll141Recortes/desp)
	}

	scroll141Desplazados, scroll141Enteros, scroll141Quietos = 0, 0, 0
	scroll141PxFranjas, scroll141PxCajas, scroll141PxPaneles = 0, 0, 0
	scroll141UsMover, scroll141UsFranjas = 0, 0
	scroll141Recortes = 0
}
