package main

import "hash/crc32"

// DC-002.142 -- EL TEXTO DEL PANEL, CONTADO: cuantos glifos se pintaron y cuanta tinta cae en el area del texto.
//
// EL PROBLEMA QUE MIDE. En la demo jugable (DC-002.140) el panel de Akara ABRIA y el motor entregaba el texto, pero
// en pantalla el recuadro salia NEGRO con su borde y sin una sola letra. Lo unico que el log decia era
// `pixeles= 140976`, y ese numero es `Ancho * Alto` del panel: el area del FONDO, no la del texto. O sea: el
// contador viejo no distinguia "el panel se pinto" de "las letras se pintaron". Esta tarjeta pone los dos numeros
// que faltaban, medidos sobre el lienzo:
//
//  1. LOS GLIFOS DE LOS TEXTOS: cuantas letras se pidieron, cuantas tenian glifo cargado, cuantas se mandaron al
//     pincel, y cuanta TINTA (indices distintos de 0) suman esos glifos.
//  2. EL AREA DEL TEXTO EN EL LIENZO: cuantos pixeles NO VACIOS (R,G,B != 0,0,0) hay adentro del panel justo
//     despues de pintar el fondo y otra vez despues de pintar los glifos. La diferencia es la tinta que llego a la
//     pantalla -- que es lo que el ojo ve como letras, sin depender de mirar la foto. El crc32 del area es el
//     segundo testigo: puede pasar que la tinta caiga sobre pixeles que ya no eran negros (el borde, la linea del
//     medio) y el conteo no se mueva; el crc si se mueve.
//
// SE MIDE UNA VEZ POR PANEL (los dos primeros tamaños distintos: el del dialogo y el de la quest), no por cuadro: el
// barrido del area es un recorrido de ~141.000 pixeles y el panel se pinta en todos los cuadros mientras esta
// abierto. Los contadores de glifos se llevan por cuadro igual, y las lineas del resumen son las del ultimo medido.

// panelesAMedir142: cuantos paneles distintos se miden con el barrido del area (en la demo son tres: el del dialogo
// del NPC, el de la quest al aceptar y el de la quest al volver, que tienen anchos distintos).
const panelesAMedir142 = 3

var (
	// lo que se sabe de la carga de los glifos (se llena en `cargarLosGlifosDelPanel` / `cargarGlifosDeTextos`)
	glifosCargados142   int // glifos en el mapa del panel
	glifosConTinta142   int // de esos, cuantos tienen algun indice != 0 (si son 0, no hay nada que pintar)
	glifosVacios142     int // de esos, cuantos vinieron TODO en cero (el sprite no trajo la letra)
	bytesDelSprite142   int // bytes del font16.DC6 que se pidieron
	crcDelSprite142     int // crc32 de esos bytes: dice si el archivo es el mismo del host
	cuadrosDelSprite142 int // cuadros del DC6 de la fuente (256 en font16)
	bytesDeLosGlifos142 int // bytes de indices que se quedaron vivos en el monton de C (224 por letra)

	// lo que hace el pincel con el texto, POR CUADRO (se cuenta en `dibujarUnTexto`)
	letrasPedidas142  int // letras de los textos del panel (nombre + renglones + filas)
	glifosPintados142 int // letras con glifo: se llamo al pincel
	letrasSinGlifo142 int // letras que no estan en el mapa (no se pudo pintar)
	pxTinta142        int // pixeles de tinta (indices != 0) de los glifos que se mandaron al pincel

	// la medida del area del texto sobre el lienzo (los panelesAMedir142 primeros paneles distintos)
	panelesMedidos142     int
	tamanosMedidos142     [panelesAMedir142][2]int
	pxDelAreaTexto142     int
	pxNoVaciosAntes142    int
	pxNoVaciosDespues142  int
	crcDelAreaAntes142    uint32
	crcDelAreaDespues142  uint32
	msDelTexto142         int
	anchoDelAreaTexto142  int
	altoDelAreaTexto142   int
	nombreDelAreaTexto142 string
)

// hayQueMedirEstePanel142: si este tamaño de panel todavia no se midio.
func hayQueMedirEstePanel142(anchoZ, altoZ int) bool {
	if panelesMedidos142 >= panelesAMedir142 {
		return false
	}

	for i := 0; i < panelesMedidos142; i++ {
		if tamanosMedidos142[i][0] == anchoZ && tamanosMedidos142[i][1] == altoZ {
			return false
		}
	}

	return true
}

// guardarElPanelMedido142: anota el tamaño medido y los numeros que van al resumen.
func guardarElPanelMedido142(nombre string, anchoZ, altoZ, antes, despues int, crcAntes, crcDespues uint32,
	ms int) {
	if panelesMedidos142 < panelesAMedir142 {
		tamanosMedidos142[panelesMedidos142] = [2]int{anchoZ, altoZ}
	}

	panelesMedidos142++

	nombreDelAreaTexto142 = nombre
	anchoDelAreaTexto142, altoDelAreaTexto142 = anchoZ, altoZ
	pxDelAreaTexto142 = anchoZ * altoZ
	pxNoVaciosAntes142, pxNoVaciosDespues142 = antes, despues
	crcDelAreaAntes142, crcDelAreaDespues142 = crcAntes, crcDespues
	msDelTexto142 = ms

	println("JUEGO142: PANEL TEXTO nombre=", nombre, "medio=", panelesMedidos142, "caja=", anchoZ, "x", altoZ,
		"pixeles=", pxDelAreaTexto142, "noVaciosAntes=", antes, "noVaciosDespues=", despues, "tintaEnPantalla=",
		despues-antes, "letrasPedidas=", letrasPedidas142, "glifosPintados=", glifosPintados142,
		"sinGlifo=", letrasSinGlifo142, "pixelesDeTinta=", pxTinta142, "crcAntes=", int(crcAntes),
		"crcDespues=", int(crcDespues), "elAreaCambio=", crcAntes != crcDespues, "ms=", ms)
}

// contarNoVaciosDeUnArea: cuantos pixeles del rectangulo NO son negros (R, G y B en 0). Es el numero que separa
// "el fondo del panel" (que pinta indice oscuro, borde claro y la linea del medio) de "las letras".
func contarNoVaciosDeUnArea(pixeles []byte, x0, y0, anchoZ, altoZ int) int {
	if anchoZ <= 0 || altoZ <= 0 {
		return 0
	}

	if x0 < 0 {
		x0, anchoZ = 0, anchoZ+x0
	}

	if y0 < 0 {
		y0, altoZ = 0, altoZ+y0
	}

	if x0+anchoZ > ancho {
		anchoZ = ancho - x0
	}

	if y0+altoZ > alto {
		altoZ = alto - y0
	}

	cuenta := 0

	for y := y0; y < y0+altoZ; y++ {
		for x := x0; x < x0+anchoZ; x++ {
			j := (y*ancho + x) * 4

			if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
				cuenta++
			}
		}
	}

	return cuenta
}

// crcDeUnArea: el crc32 de los bytes de un rectangulo del lienzo. Es el segundo testigo de "el area cambio": el
// conteo de no vacios puede quedarse igual si la tinta cae justo sobre pixeles que ya no eran negros.
func crcDeUnArea(pixeles []byte, x0, y0, anchoZ, altoZ int) uint32 {
	if anchoZ <= 0 || altoZ <= 0 {
		return 0
	}

	if x0 < 0 {
		x0, anchoZ = 0, anchoZ+x0
	}

	if y0 < 0 {
		y0, altoZ = 0, altoZ+y0
	}

	if x0+anchoZ > ancho {
		anchoZ = ancho - x0
	}

	if y0+altoZ > alto {
		altoZ = alto - y0
	}

	h := crc32.NewIEEE()

	for y := y0; y < y0+altoZ; y++ {
		j := (y*ancho + x0) * 4
		h.Write(pixeles[j : j+anchoZ*4])
	}

	return h.Sum32()
}

// resumenDelTexto142: las lineas del cierre con los numeros del texto del panel.
func resumenDelTexto142() {
	tintaEnPantalla := pxNoVaciosDespues142 - pxNoVaciosAntes142

	println("JUEGO142: PANEL TEXTO-RESUMEN panelesMedidos=", panelesMedidos142, "ultimo=", nombreDelAreaTexto142,
		"caja=", anchoDelAreaTexto142, "x", altoDelAreaTexto142, "pixeles=", pxDelAreaTexto142,
		"noVaciosAntes=", pxNoVaciosAntes142, "noVaciosDespues=", pxNoVaciosDespues142,
		"tintaEnPantalla=", tintaEnPantalla, "crcAntes=", int(crcDelAreaAntes142),
		"crcDespues=", int(crcDelAreaDespues142), "elAreaCambio=", crcDelAreaAntes142 != crcDelAreaDespues142,
		"ms=", msDelTexto142)

	println("JUEGO142: PANEL GLIFOS-RESUMEN cargados=", glifosCargados142, "conTinta=", glifosConTinta142,
		"vacios=", glifosVacios142, "bytes=", bytesDeLosGlifos142, "spriteBytes=", bytesDelSprite142,
		"spriteCuadros=", cuadrosDelSprite142, "crcSprite=", crcDelSprite142)

	// EL CRITERIO DE ESTA TARJETA, en un renglon: hay glifos con tinta, se midio un panel y el area del texto del
	// lienzo se lleno con esa tinta (pixeles no vacios despues > antes Y el crc cambio).
	llego := glifosConTinta142 > 0 && panelesMedidos142 > 0 && pxNoVaciosDespues142 > pxNoVaciosAntes142 &&
		crcDelAreaAntes142 != crcDelAreaDespues142

	println("JUEGO142: PANEL TEXTO CRITERIO glifosConTinta=", glifosConTinta142 > 0,
		"seMidioUnPanel=", panelesMedidos142 > 0, "laTintaLlegoAlLienzo=",
		pxNoVaciosDespues142 > pxNoVaciosAntes142, "elAreaCambio=", crcDelAreaAntes142 != crcDelAreaDespues142,
		"cumple=", llego)
}
