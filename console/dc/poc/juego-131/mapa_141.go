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
//       - las BANDAS de los paneles (HUD, minimapa): son opacos y se vuelven a pintar en el mismo sitio, pero
//         el del cuadro anterior quedo corrido unos pixeles y su borde asomaria.
//  3. La presentacion convierte el cuadro entero, como siempre (el lienzo se movio todo). Lo que se ahorra
//     son los 60 ms del mapa, no la conversion.
//
// LO QUE NO CAMBIA, Y ES EL CRITERIO: el cuadro 1 se pinta ENTERO con el camino de siempre (los tres primeros
// cuadros lo hacen, ver `comoSeDibujaElMapa141`), asi que su crc32 tiene que seguir dando lo mismo
// (SOLO-MAPA -1654008076, CON-JUGADOR -1025148544). Y un cuadro caminando tiene que quedar IDENTICO al que
// daria un repintado entero: el corrimiento mueve los mismos pixeles y las franjas se pintan con el mismo
// bucle del mapa (solo cambia QUE pixeles se escriben).
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
	scroll141PxFranjas, scroll141PxCajas, scroll141PxPaneles                 int
	scroll141UsMover, scroll141UsFranjas                                     int
	scroll141Recortes, scroll141MaxSX, scroll141MaxSY                        int

	// scroll141Comparaciones: cuantas veces se comparo el cuadro de la copia contra el repintado entero.
	scroll141Comparaciones int
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

	tMover := d2cUs()

	if r := pvr.MoverLienzo(sx, sy); r != 0 {
		scroll141Fallos++

		println("JUEGO141: LA COPIA NO SE PUDO codigo=", r, "sx=", sx, "sy=", sy)

		return false
	}

	scroll141UsMover += int(d2cUs() - tMover)

	scroll141Caminando = true
	scroll141UltimoFueCopia = true

	tFranjas := d2cUs()

	pxFranjas, pxCajas, pxPaneles, recortes := armarLosRecortes141(malla, sx, sy)

	rendererDelMapa.Render(superficie)

	usFranjas := int(d2cUs() - tFranjas)

	scroll141UsFranjas += usFranjas

	malla.SinRecortes()

	// LA PRUEBA DE QUE EL CUADRO ES EL MISMO (palanca `COMPARAR_141=si`). Despues del cuadro caminando se pinta
	// el mapa ENTERO en el mismo sitio (el camino de DC-002.139, sin recortes) y se comparan los dos lienzos
	// bloque por bloque de 32x32: si el corrimiento y las franjas estan bien, no tiene que haber ni un bloque
	// distinto. Cuesta un repintado entero (60 ms) por cuadro comparado, asi que solo corre en las primeras
	// comparaciones y nunca en la corrida del criterio.
	if comparar141 == "si" && scroll141Comparaciones < 8 {
		scroll141Comparaciones++

		pixeles := malla.Imagen().Pixeles()

		// El crc32 se calcula ACA y no con `crcLienzo`, que devuelve 0 cuando el instrumento por cuadro esta
		// apagado (`medirCuadro=no`): la comparacion tiene que valer en el binario del criterio tambien.
		crcCopia := crc32.ChecksumIEEE(pixeles)

		var antes [bloquesDeLaComparacion141]uint32

		cuantosBloques := crcerLosBloques141(pixeles, &antes)

		// EL COSTO FIJO DEL RENDERER. Un render con un solo recorte de 1x1 no pinta practicamente nada: lo que
		// tarde es el aparato por tile, o sea recorrer los ~198 tiles del cuadro aunque no escriban un pixel. Es
		// el numero que dice si el cuadro caminando se va en los pixeles o en recorrer el mapa.
		malla.RecortarA(0, 0, 1, 1)

		tFijo := d2cUs()

		rendererDelMapa.Render(superficie)

		usFijo := int(d2cUs() - tFijo)

		malla.SinRecortes()

		tEntero := d2cUs()

		rendererDelMapa.Render(superficie)

		usEntero := int(d2cUs() - tEntero)

		crcEntero := crc32.ChecksumIEEE(pixeles)

		distintos, comparables, bx0, by0, bx1, by1, primero := contarBloquesDistintos141(pixeles, &antes, cuantosBloques)

		println("JUEGO141: COMPARACION cuadro=", cuadro, "sx=", sx, "sy=", sy, "crcCopia=", int(crcCopia),
			"crcEntero=", int(crcEntero), "igual=", crcCopia == crcEntero,
			"bloquesDistintos=", distintos, "deComparables=", comparables,
			"cajaDistinta=", bx0, by0, bx1, by1, "primerBloque=", primero,
			"pxFranjas=", pxFranjas, "pxCajas=", pxCajas, "pxPaneles=", pxPaneles, "recortes=", recortes,
			"usFranjas=", usFranjas, "usEntero=", usEntero, "usFijo=", usFijo,
			"pxPintados=", pvr.CensoDeLosPixeles141())
	}

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
	// LO QUE NO SE PINTA, PORQUE NO SE VE. Las tres cajas de los paneles son OPACAS y se vuelven a pintar
	// enteras en el mismo sitio (lo mismo que ya asume el camino quieto de DC-002.139, que no repinta el mapa
	// debajo de ellas): asi que de la franja solo hay que pintar la parte que el panel no tapa. En la practica
	// eso borra media franja -- la de arriba entera (el HUD ocupa 0..64 y el corrimiento vertical medido es de
	// 1..34 pixeles) y, en la de la derecha, todo lo que cae debajo del minimapa.
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

	// ---- 3. LAS BANDAS DE LOS PANELES -----------------------------------------------------------
	//
	// El HUD y el minimapa son OPACOS y se pintan otra vez en el mismo sitio de la pantalla, asi que el
	// interior del panel no hay que tocarlo. Lo que asoma es el borde del panel del cuadro anterior, corrido
	// (sx, sy): la banda que el panel de ahora NO tapa se repinta con el mapa (ver `agregarBanda141`).
	//
	// Las tres cajas son las mismas que marca DC-002.139 para la presentacion por bloques (medidas en la
	// captura de DC-002.140): la franja del HUD de arriba (0,0)-(640,64), la de abajo (0,400)-(640,480) y la
	// caja del minimapa (350,48)-(640,290), 279x225 pegada a la esquina de arriba a la derecha.
	paneles += agregarBanda141(malla, 0, 0, ancho, altoDelHUD141, sx, sy)
	paneles += agregarBanda141(malla, 0, alto-altoDelPie141, ancho, alto, sx, sy)
	paneles += agregarBanda141(malla, ancho-anchoDelMinimapa141, arribaDelMinimapa141,
		ancho, abajoDelMinimapa141, sx, sy)

	cuantos = malla.RecortesPuestos()

	return franjas, cajas, paneles, cuantos
}

// Las tres cajas de los paneles: las mismas que marca DC-002.139 para la presentacion por bloques (medidas en
// la captura de DC-002.140: la franja del HUD de arriba, la de abajo y la caja del minimapa, 279x225 pegada a
// la esquina de arriba a la derecha).
const (
	altoDelHUD141        = 64
	altoDelPie141        = 80
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

// enLosPaneles141: si el bloque de 32x32 en (x0,y0) cae en alguna de las tres cajas de los paneles.
//
// POR QUE LA COMPARACION LOS SALTEA. Los paneles se pintan DESPUES del mapa y siempre en el mismo sitio: en un
// cuadro que camina, el mapa que acaba de repintarse adentro de la caja todavia no los tiene, pero lo que si
// tiene adentro es el fantasma corrido del panel del cuadro anterior (lo tapa el panel de este cuadro, que
// todavia no se dibujo). El repintado entero, en cambio, deja mapa limpio ahi. O sea que adentro de las cajas
// los dos caminos SON distintos por construccion, y no es un error: por eso la comparacion mira solo el mapa
// que se ve, que es el resto del lienzo.
func enLosPaneles141(x0, y0 int) bool {
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

// contarBloquesDistintos141: cuantos bloques del lienzo (fuera de los paneles) cambiaron respecto de `antes`, la
// caja que los contiene (en pixeles) y el indice del primero. Es el diagnostico que dice DONDE se rompio el
// cuadro caminando: una caja pegada a un borde es una franja que no se pinto, una caja alrededor del jugador es
// un fantasma.
func contarBloquesDistintos141(pixeles []byte, antes *[bloquesDeLaComparacion141]uint32, cuantos int) (distintos, comparables, bx0, by0, bx1, by1, primero int) {
	columnas := ancho / ladoDelBloque141

	bx0, by0, bx1, by1, primero = -1, -1, -1, -1, -1

	k := 0

	for by := 0; by+ladoDelBloque141 <= alto; by += ladoDelBloque141 {
		for bx := 0; bx+ladoDelBloque141 <= ancho; bx += ladoDelBloque141 {
			if k >= cuantos {
				return distintos, comparables, bx0, by0, bx1, by1, primero
			}

			if enLosPaneles141(bx, by) {
				k++

				continue
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

// informeDelCuadroQueCamina141: el censo del tramo, pegado a la linea del mapa estatico (cada 30 cuadros).
func informeDelCuadroQueCamina141() {
	if cuadro%cuadrosDelInforme132 != 0 {
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
	}

	println("JUEGO141: LIENZO-CAMINA cuadros=", cuadro, "desplazados=", desp, "enteros=", ent, "quietos=", qui,
		"fallos=", scroll141Fallos, "palanca=", scroll141,
		"usMover=", scroll141UsMover, "usFranjas=", scroll141UsFranjas,
		"pxFranjas=", scroll141PxFranjas, "pxCajas=", scroll141PxCajas, "pxPaneles=", scroll141PxPaneles,
		"recortes=", scroll141Recortes, "maxSX=", scroll141MaxSX, "maxSY=", scroll141MaxSY)

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
