package main

import "math"

// DC-002.124 -- EL ALIJO DEL PUEBLO: EL COFRE, EL GUARDAR Y EL DEVOLVER.
//
// LA PREMISA DE LA TARJETA NO SE CUMPLE, Y ESTA MEDIDO ANTES DE ESCRIBIR NADA. La tarjeta dice "el alijo del
// pueblo es un objeto del .ds1". Se midio (`console/dc/sonda-inventario-124`, seccion 2) y NO lo es: el pueblo del
// acto 1 son cuatro `.ds1` (townE1/N1/W1/S1 de d2data.mpq, 57.691/57.652/57.700/57.672 B) con 170 objetos y 13
// IDs distintos (0 Dummy, 1/3 Casket, 2 Shrine, 4 LargeUrn, 5 chest, 7 Barrel, 8 TowerTome, 13 Door, 33/102
// Dummy, 52 Urn, 110 not used) — nombrados con objects.txt del PARCHE y tambien con el de d2data.mpq (mismo
// nombre en las dos versiones) — y NINGUNO es el alijo. La tabla SI tiene las filas del cofre (`objects.txt` del
// parche: id 186, 187, 188 y 203..205 = "stash"; 125..128/155/159/274/416/463 = "hidden stash"), pero el `.ds1`
// de este disco no coloca ninguna, y `d2exp.mpq` (que el disco del POC no lleva) tampoco trae otro `.ds1` del
// pueblo. O sea: en esta instalacion el cofre lo pone el juego por su cuenta, no el mapa.
//
// QUE HACE ENTONCES EL POC, y por que es honesto: el alijo se PONE en una celda conocida del pueblo —la celda de
// arranque del jugador, (28,20), que es la que el POC ya usa en todas las corridas— con el id 187 de la tabla, el
// que el juego llama "stash" en el acto 1. El log lo dice en cada corrida (`ALIJO ORIGEN ... elDs1LoTrae= false`),
// asi que nadie puede confundir esto con "el alijo estaba en el mapa".
//
// COMO SE ABRE. En la PC el cofre se abre con el PUNTERO (un clic sobre el objeto). En la consola no hay puntero,
// asi que el POC usa PROXIMIDAD con `radioDelAlijo` (2,0 celdas, el numero esta en knobs.go con su medicion): el
// jugador que queda a 1,41 celdas (la diagonal) despues de recoger el objeto abre el cofre; con el `pickupRange`
// de 0,9 celdas de los objetos del piso no abriria nunca.
//
// LAS GRILLAS SON LAS DEL MOTOR: el alijo es `d2inventory.Dims(ContainerStash, true)` = 6x8 = 48 celdas (el mismo
// numero que el panel del banco de la expansion). El guardar y el devolver mueven EL OBJETO entre las dos grillas
// con las mismas funciones de `inventario_124.go` (`meter`, `vaciarCelda`): no hay una segunda copia del objeto.

const (
	// el panel del alijo: a la derecha y arriba del cuadro (el del jugador va abajo a la izquierda, y los dos
	// paneles no se solapan: 176+288 = 464 < 236... ver las cuentas abajo).
	panelAlijoX = 232
	panelAlijoY = 96

	// celdaDelAlijoX/Y: donde esta el cofre. La celda de arranque del jugador en el pueblo (la misma de todas las
	// corridas del POC: `celdaP1X`/`celdaP1Y` en knobs.go valen 0,0 = la celda del POC, (28,20)).
	//
	// DC-002.138: paso a VAR (abajo, con el resto de lo del MVP) y la razon es la misma que la de los cuadros del
	// guion: el MVP JUGABLE corre en OTRA ventana del pueblo (la de Akara, ver `mvp_138.go`) y el cofre tiene que
	// estar donde el jugador queda despues de recoger el objeto, que ahi no es (28,20). Sin MVP valen 28 y 20, que
	// es lo que valian cuando eran `const`.

	// idDelAlijo: la fila de objects.txt que el juego llama "stash" en el acto 1 (medido en la sonda).
	idDelAlijo = 187
)

var (
	// DC-002.138: la celda del cofre, que ahora es palanca (ver el comentario de arriba). Sin MVP son (28,20), la
	// celda de arranque de todas las corridas del POC.
	celdaDelAlijoX float64 = 28
	celdaDelAlijoY float64 = 20

	abiertoElAlijo  bool
	guardoElAlijo   bool
	devolvioElAlijo bool

	distanciaAlAlijo       float64
	celdaDelJugadorAlAbrir float64
)

// cargarElAlijo: la grilla ya la armo `cargarElInventario` (las dos nacen juntas). Aca se dice DONDE esta el
// cofre y por que, que es lo que el log tiene que poder auditar.
func cargarElAlijo() {
	if inventarioKnob != "si" || alijoKnob != "si" {
		return
	}

	println("JUEGO124: ALIJO ORIGEN id=", idDelAlijo, "nombre=", "stash", "celdaDelPueblo=", celdaDelAlijoX,
		",", celdaDelAlijoY, "elDs1LoTrae= false", "radio=", radioDelAlijo,
		"grilla=", grillaAlijo.W, "x", grillaAlijo.H, "celdas=", len(grillaAlijo.Celdas))
	println("JUEGO124: ALIJO AVISO el .ds1 del pueblo de este disco NO coloca el cofre (170 objetos, 13 ids,",
		"ninguno es stash: sonda-inventario-124): el POC lo pone en la celda de arranque del jugador")
}

// pasoDelAlijo: el instrumento (el emulador no tiene puntero) y las dos operaciones del criterio, en cuadros
// distintos para que cada una se pueda ver en el log: abrir + guardar, y despues devolver.
func pasoDelAlijo() {
	if inventarioKnob != "si" || alijoKnob != "si" || grillaAlijo == nil || alijoAuto != "si" {
		return
	}

	// ---- abrir: en el cuadro que dice la palanca, si el jugador esta dentro del radio ----
	if !abiertoElAlijo && cuadro+1 == cuadroAbreElAlijo {
		ax, ay := 0.0, 0.0

		if len(jugadores) > 0 && jugadores[0].Mov != nil && jugadores[0].Mov.Est != nil {
			ax, ay = jugadores[0].Mov.Est.Celda()
		}

		dx, dy := ax-float64(celdaDelAlijoX), ay-float64(celdaDelAlijoY)
		distanciaAlAlijo = raizCuadrada(dx*dx + dy*dy)
		celdaDelJugadorAlAbrir = ax

		jugador := "P1"
		if len(jugadores) > 0 {
			jugador = jugadores[0].Nombre
		}

		if distanciaAlAlijo > radioDelAlijo {
			println("JUEGO124: ALIJO NO-ABRE jugador=", jugador, "celda=", ax, ay, "alijo=", celdaDelAlijoX,
				",", celdaDelAlijoY, "distancia=", distanciaAlAlijo, "radio=", radioDelAlijo,
				": el jugador no esta cerca del cofre")

			return
		}

		abiertoElAlijo = true
		grillaAlijo.Abierta = true
		grillaAlijo.Veces++

		println("JUEGO124: ALIJO ABRE jugador=", jugador, "celda=", ax, ay, "alijo=", celdaDelAlijoX, ",",
			celdaDelAlijoY, "distancia=", distanciaAlAlijo, "radio=", radioDelAlijo, "cuadro=", cuadro+1,
			"panel=", grillaAlijo.X0, grillaAlijo.Y0, "grilla=", grillaAlijo.W, "x", grillaAlijo.H)

		return
	}

	if !abiertoElAlijo {
		return
	}

	// ---- guardar: el objeto del inventario pasa al alijo (la primera celda libre del cofre) ----
	if !guardoElAlijo {
		if fila, columna, codigo, ok := grillaJugador.primeraCelda(); ok {
			celdas := enLaGrilla.AltoCeldas
			ancho := enLaGrilla.AnchoCeldas

			destino := grillaAlijo.meter(codigo, ancho, celdas)
			if destino == nil {
				println("JUEGO124: ALIJO GUARDA NO-ENTRA codigo=", codigo, "huella=", ancho, "x", celdas,
					"libres=", len(grillaAlijo.Celdas)-grillaAlijo.ocupadas())

				return
			}

			grillaJugador.vaciarCelda(fila, columna, ancho, celdas)
			guardoElAlijo = true

			println("JUEGO124: ALIJO GUARDA codigo=", codigo, "delInventario=", fila, ",", columna,
				"alAlijo=", destino.Fila, ",", destino.Columna, "ocupadasInventario=", grillaJugador.ocupadas(),
				"ocupadasAlijo=", grillaAlijo.ocupadas(), "cuadro=", cuadro+1)
		}

		return
	}

	// ---- devolver: un cuadro despues, el objeto vuelve al inventario ----
	if !devolvioElAlijo && cuadro+1 > cuadroAbreElAlijo {
		if fila, columna, codigo, ok := grillaAlijo.primeraCelda(); ok {
			destino := grillaJugador.meter(codigo, enLaGrilla.AnchoCeldas, enLaGrilla.AltoCeldas)
			if destino == nil {
				println("JUEGO124: ALIJO DEVUELVE NO-ENTRA codigo=", codigo)

				return
			}

			grillaAlijo.vaciarCelda(fila, columna, enLaGrilla.AnchoCeldas, enLaGrilla.AltoCeldas)
			devolvioElAlijo = true

			println("JUEGO124: ALIJO DEVUELVE codigo=", codigo, "delAlijo=", fila, ",", columna,
				"alInventario=", destino.Fila, ",", destino.Columna, "ocupadasInventario=",
				grillaJugador.ocupadas(), "ocupadasAlijo=", grillaAlijo.ocupadas(), "cuadro=", cuadro+1)
		}
	}

	// ---- cerrar: dos cuadros despues de abrir, ya con el objeto de vuelta en el inventario ----
	if cerrarLosPaneles == "si" && devolvioElAlijo && cuadro+1 >= cuadroAbreElAlijo+2 && grillaAlijo.Abierta {
		grillaAlijo.Abierta = false
		abiertoElAlijo = false

		println("JUEGO124: ALIJO CIERRA cuadro=", cuadro+1, "jugador=", detalleDeUnaGrilla(grillaJugador),
			"alijo=", detalleDeUnaGrilla(grillaAlijo), "volvio=", elObjetoVolvioAlInventario())
	}
}

// raizCuadrada: la raiz, con el math del POC (que ya lo importa en los otros archivos de la geometria).
func raizCuadrada(v float64) float64 { return math.Sqrt(v) }

// devolverElAlijoAlInventario: usado por el cierre para afirmar que el objeto volvio.
func elObjetoVolvioAlInventario() bool {
	if grillaJugador == nil {
		return false
	}

	_, _, codigo, ok := grillaJugador.primeraCelda()

	return ok && codigo == codigoDelObjeto() && devolvioElAlijo
}
