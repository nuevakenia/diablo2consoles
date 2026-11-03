package main

// DC-002.138 -- EL MVP JUGABLE: UNA CORRIDA CON TODO JUNTO, DE PUNTA A PUNTA.
//
// QUE ES. El POC `juego-131` ya tiene TODAS las piezas (18.414 lineas, 32 archivos): mapa, jugador, animacion,
// controles, HUD, minimapa, split, monstruo, combate, muerte, drop, inventario, alijo, NPCs, dialogo, quest y
// mercader. Cada tarjeta las midio POR SEPARADO y con su propio guion de cuadros FIJOS. Lo que no existia es UNA
// corrida que las use TODAS JUNTAS, en el orden en que se juega, sin que una pise a la otra. Eso es este archivo.
//
// POR QUE UN GUION Y NO NUEVE CORRIDAS. Los cuadros fijos funcionan cuando la corrida es de una tarjeta: el de la
// 124 sabe que el bicho muere en el 6 y el objeto cae en el 7. Encadenadas, esos numeros chocan (el panel del
// inventario se abriria mientras el jugador todavia camina al bicho). Aca el guion es una MAQUINA DE ETAPAS: cada
// etapa se cierra con lo que MIDIO la anterior (la vida del bicho, el objeto recogido, el panel cerrado, la
// distancia a Akara) y recien entonces ARMA el cuadro de la etapa siguiente en las palancas de las otras tarjetas
// (`cuadroAbreElInventario`, `cuadroAbreElAlijo`, `guionDeLaQuest`). Asi el camino de cada pieza es EL SUYO, sin
// una linea cambiada, y lo unico nuevo es quien decide cuando.
//
// LA VENTANA DEL PUEBLO, Y POR QUE EL MVP CORRE EN LA DE AKARA. La cache de tiles del renderer tiene UN tramo de
// 3.481.600 B (10 bloques de VRAM, `knobs.go`) y la ventana del centro del pueblo ya son 2.600.960 B (51
// imagenes): no entran DOS zonas (medido en DC-002.116: las dos ventanas del split son 3.276.800 y el movimiento
// derrama al monton de Go, donde el recolector muere). Akara esta en (38.4,3.2) del townE1 y el centro del POC en
// (28,20): 19,8 celdas. Caminar hasta ella pide la ventana del centro MAS la de su esquina. Por eso el MVP corre
// ENTERO en la ventana de Akara: el jugador arranca a 3,1 celdas de ella (`VISTA_X=37 VISTA_Y=6`), el bicho nace
// a 1,4 celdas del jugador y el cofre queda en la celda de arranque, que es lo mismo que hace la corrida de
// DC-002.124 pero corrido 9 celdas. Se dice en el log, con las celdas, para que nadie lo lea como el pueblo
// entero.
//
// LAS NUEVE MARCAS DE LA TABLA (la tarjeta las pide con si/no MEDIDO, no con un relato):
//
//	1 CAMINA          la cruceta mueve al heroe: celdas caminadas, celdas/segundo del motor y las direcciones del
//	                  COF que se usaron, con el modo TW (caminar en el pueblo) en el cuadro
//	2 MATA            el golpe baja la vida del Returned a 0 y el bicho cae: los cuadros del DT, uno por cuadro
//	3 RECOGE          el objeto que solto el cadaver pasa del piso al inventario del jugador (regla del motor)
//	4 INVENTARIO      el panel se abre y el objeto se ve EN SU CELDA (la grilla de `d2inventory`, 10x4)
//	5 ALIJO           el objeto se guarda en el cofre del pueblo (6x8) y vuelve: DOS grillas, un objeto
//	6 HABLA           el jugador esta dentro del radio de Akara y el panel abre con SU pagina del log
//	7 ACEPTA          `Hear(64)`: la Den of Evil queda aceptada en el registro del guardado
//	8 AVANZA          el evento de area (pueblo -> cueva), las bajas de la cueva y la pagina 5 ("volve con Akara")
//	9 VUELVE-A-CAMINAR el mundo y el HUD siguen bien DESPUES de todo: el heroe camina otra vez, el HUD y el minimapa
//	                  pintan, y el cuadro del mundo se repite identico (cuadros+1 contra cuadros+2)
//
// LO QUE NO ENTRA: los fps (esa es DC-002.139: aca se juzga que TODO junto funcione, con los fps que haya), las
// habilidades y el guardado a disco.
//
// LAS PALANCAS DE ESTA TARJETA: `mvpKnob` (si/no), `VISTA_X`/`VISTA_Y` (la ventana, 37 y 6), `NPC_LISTA` (tiene que
// traer `akara`), `JUGADORES=1`, `MONSTRUOS=2` (uno para el combate del MVP y otro para la cueva de la quest) y
// `FASE=apagado` (los tres cuadros del cierre son la prueba de la marca 9).

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
)

// ============================== LAS ETAPAS ==============================

const (
	mvpArranca = iota
	mvpCaminaAlBicho
	mvpGolpea
	mvpCae
	mvpRecoge
	mvpInventario
	mvpAlijo
	mvpCaminaAAkara
	mvpAcepta
	mvpCierraElAceptar
	mvpCueva
	mvpLimpia
	mvpCaminaDeVuelta
	mvpTerminado
)

// nombreDeLaEtapa: para el log (una etapa sin nombre no se puede discutir en el registro).
var nombreDeLaEtapa = []string{
	"arranca", "camina-al-bicho", "golpea", "cae", "recoge", "inventario", "alijo", "camina-a-akara", "acepta",
	"cierra-el-aceptar", "cueva", "limpia", "camina-de-vuelta", "terminado",
}

// mvpCuadroLejos: el cuadro que "no llega nunca". Se pone en las etapas de la quest que este guion NO usa, porque
// su camino las dispara por numero (`case n == guionDeLaQuest.Cobra`) y un cero las haria saltar en el cuadro 0.
const mvpCuadroLejos = 100000

// mvpSinMarca / mvpMarca: las nueve marcas del criterio, por indice (1..9).
const (
	mvpMarcas = 9
)

var (
	// la etapa y su reloj
	mvpEtapa          = mvpArranca
	mvpCuadroEtapa    = 0
	mvpCuadrosEtapa   = 0
	mvpMsEtapa        = int64(0)
	mvpCuadrosTotales = 0

	// lo que la etapa mide: el camino de este tramo
	mvpCeldaAntesX, mvpCeldaAntesY float64
	mvpCeldasDeLaEtapa             float64
	mvpCeldasCaminadas             float64
	mvpCuadrosCaminando            = 0
	mvpDirecciones                 = [16]bool{}
	mvpDireccionesUsadas           = 0
	mvpBloqueosDeLaEtapa           int

	// el objetivo del tramo (celdas del mapa)
	mvpVaHaciaX, mvpVaHaciaY float64
	mvpCamina                = false

	// el escenario, medido al arrancar
	mvpArranqueX, mvpArranqueY float64
	mvpBichoX, mvpBichoY       float64
	mvpPeleaX, mvpPeleaY       float64
	mvpAkaraX, mvpAkaraY       float64
	mvpAlijoX, mvpAlijoY       float64
	mvpCuadro1                 int

	// las nueve marcas: si paso, en que cuadro, con que evidencia y con que crc32 del lienzo
	mvpMarca           [mvpMarcas + 1]bool
	mvpMarcaCuadro     [mvpMarcas + 1]int
	mvpMarcaEvidencia  [mvpMarcas + 1]string
	mvpMarcaCrc        [mvpMarcas + 1]uint32
	mvpMarcaCrcDelMapa [mvpMarcas + 1]uint32
	mvpMarcaCeldas     [mvpMarcas + 1]string

	// la muerte del bicho (marca 2)
	mvpGolpes, mvpDano, mvpCuadrosDT int
	mvpVidaInicial                   int

	// el objeto (marcas 3, 4 y 5)
	mvpObjeto          string
	mvpCeldaDelObjeto  string
	mvpCeldaEnLaGrilla string
	mvpCeldaEnElAlijo  string
	mvpPxDelPanel      int
	mvpPxDelPanelMax   int
	mvpPxDelPanelAlijo int

	// el ultimo cuadro DIBUJADO: las marcas se anotan desde `actualizar` (antes de que este cuadro se pinte), asi
	// que el crc que queda en la marca es el del cuadro anterior, que es el ultimo que el ojo pudo ver.
	mvpCrcDelCuadro        uint32
	mvpCrcDelMapaDelCuadro uint32

	// la quest (marcas 6, 7 y 8)
	mvpPaginaAlHablar int
	mvpPaginaAlLimpia int
	mvpVivosEnLaCueva int

	// el cierre (marca 9)
	mvpPxHUDDelCuadro       int
	mvpPxMinimapaDelCuadro  int
	mvpLlenosDelCuadro      int
	mvpPxHUDAlCerrar        int
	mvpPxMinimapaAlCerrar   int
	mvpCrcDelMapaAlArrancar uint32
	mvpCrcDelMapaAlCerrar   uint32
	mvpCrcDelMapaCierre2    uint32
	mvpLlenosAlArrancar     int
	mvpLlenosAlCerrar       int
	mvpCrcCierre1           uint32
	mvpCrcCierre2           uint32
	mvpCeldasDeLaVuelta     float64
	mvpCeldasDeLaVueltaIni  float64
	mvpTablaImpresa         bool
	mvpCierreImpreso        bool
)

// ============================== EL ARRANQUE ==============================

// arrancarElMVP: deja el ESCENARIO del MVP y las palancas de las otras tarjetas en su punto de partida. Se llama
// al final de `main`, cuando ya estan cargados el pueblo, el bicho, el drop, el inventario, el alijo, la quest y
// los NPCs, y antes de que el ciclo del PVR empiece a pedir cuadros.
func arrancarElMVP() {
	if mvpKnob != "si" {
		return
	}

	mvpArranqueX, mvpArranqueY = celdaDelJugadorX(), celdaDelJugadorY()

	if len(bichos) > 0 {
		mvpBichoX, mvpBichoY = bichos[0].Est.Celda()

		if bichos[0].Vida != nil {
			mvpVidaInicial = bichos[0].Vida.HP
		}

		// DONDE SE PELEA: EL MEDIO DEL CAMINO DEL BICHO. El Returned no persigue: PATRULLA entre A y B (2 celdas,
		// `patrullaDx/Dy/Largo`) a 6,25 celdas por segundo, o sea 7,5 celdas por cuadro de este POC: se teletransporta
		// de un extremo al otro y se da vuelta. Perseguirlo cuadro a cuadro es imposible (medido: 43 celdas de
		// zigzag en 23 cuadros y ni un golpe a tiro). Lo que si funciona es lo que hace un jugador: pararse EN SU
		// CAMINO -- la celda del jugador BLOQUEA al bicho (la colision es de celda y en los dos sentidos, `monstruo_121.go`)
		// -- y pegarle cuando pasa. El medio del camino esta a 1,41 celdas de los dos extremos, que es la distancia
		// con la que la tabla del exe deja pegar (`alcanceGolpe=7`).
		mvpPeleaX = (bichos[0].A[0] + bichos[0].B[0]) / 2
		mvpPeleaY = (bichos[0].A[1] + bichos[0].B[1]) / 2
	}

	// EL COFRE, UNA CELDA AL LADO DE DONDE CAE EL OBJETO. El `.ds1` de este disco no coloca el cofre (medido en
	// `sonda-inventario-124`), asi que el POC lo pone el: DC-002.124 lo ponia en la celda de arranque del jugador
	// porque ahi el jugador quedaba a 1,41 celdas del objeto. Aca el jugador arranca LEJOS del bicho (el primer
	// tramo es el camino que se mide) y recoge el objeto al lado del cadaver, asi que el cofre va al lado de la
	// celda del cadaver: a 1,0-1,4 celdas del jugador cuando guarda, dentro del radio de 2,0 de la palanca.
	if len(bichos) > 0 {
		celdaDelAlijoX, celdaDelAlijoY = mvpBichoX+1.0, mvpBichoY
	} else {
		celdaDelAlijoX, celdaDelAlijoY = mvpArranqueX, mvpArranqueY
	}

	mvpAlijoX, mvpAlijoY = celdaDelAlijoX, celdaDelAlijoY

	if n := elNPCAkara(); n != nil {
		mvpAkaraX, mvpAkaraY = n.CeldaX, n.CeldaY
	}

	// LA QUEST, EN SILENCIO HASTA QUE SU ETAPA LA ARME. Su camino dispara por numero de cuadro; con los numeros
	// lejos no toca el registro ni abre un panel hasta que el guion del MVP la llame.
	guionDeLaQuest.Acepta = mvpCuadroLejos
	guionDeLaQuest.Cierra = mvpCuadroLejos
	guionDeLaQuest.Cueva = mvpCuadroLejos
	guionDeLaQuest.Mata = mvpCuadroLejos
	guionDeLaQuest.Limpia = mvpCuadroLejos
	guionDeLaQuest.Vuelve = mvpCuadroLejos
	guionDeLaQuest.Cobra = mvpCuadroLejos
	guionDeLaQuest.Verifica = mvpCuadroLejos

	// los paneles de las otras tarjetas, tambien lejos: los abre ESTE guion, un cuadro antes de cada etapa
	cuadroAbreElInventario = mvpCuadroLejos
	cuadroAbreElAlijo = mvpCuadroLejos
	cuadroAbreElDialogo = mvpCuadroLejos

	println("JUEGO138: MVP ARRANCA -- el MVP jugable: nueve acciones en UNA corrida")
	println("JUEGO138: MVP ESCENARIO jugador=", mvpArranqueX, mvpArranqueY, "bicho=", mvpBichoX, mvpBichoY,
		"alijo=", mvpAlijoX, mvpAlijoY, "akara=", mvpAkaraX, mvpAkaraY,
		"distanciaJugadorAkara=", distanciaEntre(mvpArranqueX, mvpArranqueY, mvpAkaraX, mvpAkaraY),
		"distanciaJugadorBicho=", distanciaEntre(mvpArranqueX, mvpArranqueY, mvpBichoX, mvpBichoY),
		"ventana= la de Akara (la cache de la 3.481.600 B no aguanta dos zonas: DC-002.116)",
		"bichos=", len(bichos), "vidaDelBicho1=", mvpVidaInicial, "npcs=", len(npcsDelPueblo),
		"cuadrosDelGuion=", cuadros, "fase=", faseKnob)
	println("JUEGO138: MVP ESCENARIO-2 monstruos=", monstruosKnob, "(1 = el del combate del MVP; el resto, la cueva",
		"de la quest) npcLista= akara+... jugadores=", len(jugadores), "radios= habla", radioQuest, "recogida 0,9",
		"alijo", radioDelAlijo)
}

// elNPCAkara: el NPC de Akara en la lista cargada (nil si `NPC_LISTA` no la trae, que es el unico motivo por el que
// la marca 6 no podria pasar).
func elNPCAkara() *npc {
	for _, n := range npcsDelPueblo {
		if n != nil && n.ClaseExe == claseAkaraNPC {
			return n
		}
	}

	return nil
}

// distanciaEntre: la distancia en celdas entre dos puntos (la misma cuenta que usa el atajo, sin `math`).
func distanciaEntre(ax, ay, bx, by float64) float64 {
	dx, dy := ax-bx, ay-by

	return raizCuadrada(dx*dx + dy*dy)
}

// ============================== EL GUION: UN CUADRO ==============================

// pasoDelMVP: lo que el guion hace en ESTE cuadro. Se llama al principio de `actualizar` (antes de que las otras
// tarjetas lean sus palancas) para que lo que este guion arme ya valga en este cuadro.
func pasoDelMVP() {
	if mvpKnob != "si" {
		return
	}

	n := cuadro + 1
	if n != mvpCuadro1 {
		mvpCuadro1 = n
		mvpCuadrosTotales++
	}

	if n != mvpCuadroEtapa {
		// el cambio de cuadro del POC: se cierra lo que la etapa midio y se abre la siguiente
		mvpCuadroEtapa = n
	}

	// el camino de la etapa: la distancia que camino el jugador en ESTE cuadro (la resta de celdas, que es lo que
	// termina sumando 1,875 celdas por cuadro como el motor).
	if ax, ay := celdaDelJugadorX(), celdaDelJugadorY(); mvpCamina {
		d := distanciaEntre(ax, ay, mvpCeldaAntesX, mvpCeldaAntesY)
		if d > 0 {
			mvpCeldasDeLaEtapa += d
			mvpCeldasCaminadas += d
			mvpCuadrosCaminando++

			if mov != nil {
				if !mvpDirecciones[mov.Direccion] {
					mvpDirecciones[mov.Direccion] = true
					mvpDireccionesUsadas++
				}
			}
		}
	}

	mvpCeldaAntesX, mvpCeldaAntesY = celdaDelJugadorX(), celdaDelJugadorY()
	mvpCuadrosEtapa++

	switch mvpEtapa {
	case mvpArranca:
		mvpVaHaciaX, mvpVaHaciaY = mvpPeleaX, mvpPeleaY
		mvpCamina = true
		mvpIrA(mvpCaminaAlBicho, "camina al bicho: bits de la cruceta hacia el medio de su camino (maple -> servicio -> Paso)")
	case mvpCaminaAlBicho:
		// se cierra cuando el jugador esta EN el camino del bicho (a 1,0 del medio) o ya lo tiene a tiro
		cercaDelBicho := distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), mvpBichoX, mvpBichoY) <= 1.4
		if distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), mvpPeleaX, mvpPeleaY) <= 1.0 ||
			cercaDelBicho || mvpCuadrosEtapa > 8 {
			mvpCamina = false
			mvpMarca1()
			mvpIrA(mvpGolpea, "el heroe llega al camino del bicho: se le pega con el boton Y (pulsoDeGolpe)")
		}
	case mvpGolpea:
		// EL HEROE SE PARA EN EL CAMINO Y PEGA. El bicho patrulla 7,5 celdas por cuadro entre sus dos extremos: si
		// el heroe lo persigue, salta de un lado al otro y nunca queda a tiro (medido: 43 celdas de zigzag y cero
		// golpes). Parado en el medio, el bicho choca contra su celda y espera ahi, a tiro de la tabla del exe.
		// Si se fue lejos igual, se da un paso hacia el medio: nunca hacia el bicho.
		mvpVaHaciaX, mvpVaHaciaY = mvpPeleaX, mvpPeleaY
		mvpCamina = distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), mvpPeleaX, mvpPeleaY) > 1.2

		if len(bichos) > 0 && bichos[0].Vida != nil && bichos[0].Vida.HP <= 0 {
			mvpCamina = false
			mvpIrA(mvpCae, "la vida del Returned llego a 0: la animacion de muerte (DT, 19 cuadros)")
		} else if mvpCuadrosEtapa > 50 {
			mvpCamina = false
			mvpAviso("el bicho no murio en 50 cuadros")
			mvpIrA(mvpCae, "el bicho no murio: se sigue igual (la marca 2 va a decir que NO)")
		}
	case mvpCae:
		if len(bichos) > 0 && bichos[0].Modo == modoBestiaMuerta {
			mvpMarca2()
			mvpIrA(mvpRecoge, "el cadaver quedo en el piso (DD): el objeto esta en el suelo y se camina hasta el")
		} else if mvpCuadrosEtapa > 34 {
			mvpAviso("el bicho no llego al modo DD en 34 cuadros")
			mvpMarca2()
			mvpIrA(mvpRecoge, "se sigue al drop")
		}
	case mvpRecoge:
		// los bits los pone el instrumento de la recogida de DC-002.123 (la regla del motor, radio 0,9)
		if hayObjetoRecogido() {
			mvpMarca3()
			mvpIrA(mvpInventario, "el objeto esta en el inventario: se abre el panel en el cuadro siguiente")
		} else if mvpCuadrosEtapa > 10 {
			mvpAviso("el objeto no se recogio en 10 cuadros")
			mvpMarca3()
			mvpIrA(mvpInventario, "se sigue: la marca 3 va a decir que NO")
		}
	case mvpInventario:
		if mvpCuadrosEtapa == 1 {
			cuadroAbreElInventario = n + 1

			println("JUEGO138: MVP ARMA el panel del inventario en el cuadro", cuadroAbreElInventario,
				"(lo abre `pasoDelInventario`, el camino de DC-002.124)")
		}

		if grillaJugador != nil && grillaJugador.Veces > 0 && !grillaJugador.Abierta {
			mvpMarca4()
			mvpIrA(mvpAlijo, "el panel del inventario mostro el objeto en su celda y se cerro: el alijo del pueblo")
		} else if mvpCuadrosEtapa > 10 {
			mvpAviso("el inventario no abrio/cerro en 10 cuadros")
			mvpIrA(mvpAlijo, "se sigue con el alijo")
		}
	case mvpAlijo:
		if mvpCuadrosEtapa == 1 {
			// EL COFRE DONDE CAYO EL OBJETO. El `.ds1` de este disco no coloca el cofre (medido en
			// `sonda-inventario-124`): lo pone el POC. DC-002.124 lo ponia en una celda fija del pueblo porque el
			// jugador recogía a 1,41 celdas de ella; en el MVP el objeto cae donde muere el bicho -- que patrulla 2
			// celdas -- asi que la celda se fija RECIEN ACA, donde el jugador lo recogio: a 0 celdas del cofre.
			// Es el mismo instrumento, declarado en el log con la celda que se uso.
			if len(suelo) > 0 {
				celdaDelAlijoX, celdaDelAlijoY = suelo[0].CeldaX, suelo[0].CeldaY
			} else {
				celdaDelAlijoX, celdaDelAlijoY = celdaDelJugadorX(), celdaDelJugadorY()
			}

			mvpAlijoX, mvpAlijoY = celdaDelAlijoX, celdaDelAlijoY
			cuadroAbreElAlijo = n + 1

			println("JUEGO138: MVP ARMA el alijo en el cuadro", cuadroAbreElAlijo,
				"jugador=", celdaDelJugadorX(), celdaDelJugadorY(), "alijo=", celdaDelAlijoX, celdaDelAlijoY,
				"distancia=", distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), celdaDelAlijoX, celdaDelAlijoY),
				"radio=", radioDelAlijo)
		}

		// LA FOTO DEL COFRE CON EL OBJETO ADENTRO. Cuando el objeto vuelve al inventario el cofre queda vacio, asi
		// que el detalle de la grilla del cofre hay que guardarlo en el cuadro en que el objeto ESTA en el.
		if guardoElAlijo && mvpCeldaEnElAlijo == "" && grillaAlijo != nil && grillaAlijo.ocupadas() > 0 {
			mvpCeldaEnElAlijo = detalleDeUnaGrilla(grillaAlijo)
			mvpPxDelPanelAlijo = mvpPxDelPanelMax

			println("JUEGO138: MVP 5 COFRE-CON-EL-OBJETO cuadro=", cuadro+1, "alijo=", mvpCeldaEnElAlijo,
				"celdaDelCofre=", celdaDelAlijoX, celdaDelAlijoY)
		}

		if guardoElAlijo && !abiertoElAlijo {
			mvpMarca5()
			mvpIrA(mvpCaminaAAkara, "el objeto estuvo en el cofre y volvio: el heroe camina hasta Akara")
			mvpVaHaciaX, mvpVaHaciaY = mvpAkaraX, mvpAkaraY
			mvpCamina = true
		} else if mvpCuadrosEtapa > 10 {
			mvpAviso("el alijo no guardo/cerro en 10 cuadros")
			mvpMarca5()
			mvpIrA(mvpCaminaAAkara, "se sigue caminando a Akara")
			mvpVaHaciaX, mvpVaHaciaY = mvpAkaraX, mvpAkaraY
			mvpCamina = true
		}
	case mvpCaminaAAkara:
		if distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), mvpAkaraX, mvpAkaraY) <= 1.9 ||
			mvpCuadrosEtapa > 12 {
			mvpCamina = false
			mvpMarca6a()
			mvpIrA(mvpAcepta, "el heroe esta dentro del radio de Akara: `Hear(64)` le entrega la Den of Evil")
		}
	case mvpAcepta:
		if mvpCuadrosEtapa == 1 {
			guionDeLaQuest.Acepta = n + 1
			guionDeLaQuest.Cierra = n + 3

			println("JUEGO138: MVP ARMA el aceptar de la Den of Evil en el cuadro", guionDeLaQuest.Acepta,
				"y el cierre del panel en el", guionDeLaQuest.Cierra,
				"(el camino de DC-002.128: Activate + Hear(64))")
		}

		if questAceptada {
			mvpMarca6b()
			mvpMarca7()
			mvpIrA(mvpCierraElAceptar, "la Den of Evil esta ACEPTADA; el panel se cierra dos cuadros despues")
		} else if mvpCuadrosEtapa > 5 {
			mvpAviso("la quest no quedo aceptada en 5 cuadros")
			mvpMarca6b()
			mvpMarca7()
			mvpIrA(mvpCierraElAceptar, "se sigue (la marca 7 va a decir que NO)")
		}
	case mvpCierraElAceptar:
		if mvpCuadrosEtapa > 3 {
			mvpIrA(mvpCueva, "a la cueva: el evento de area del motor (pueblo 1 -> Den of Evil 8)")
		}
	case mvpCueva:
		if mvpCuadrosEtapa == 1 {
			guionDeLaQuest.Cueva = n + 1
			guionDeLaQuest.Mata = n + 2
			guionDeLaQuest.Limpia = n + 2 + len(bichos) + 1
			guionDeLaQuest.Cierra = guionDeLaQuest.Limpia + 2
			guionDeLaQuest.Vuelve = mvpCuadroLejos
			guionDeLaQuest.Cobra = mvpCuadroLejos
			guionDeLaQuest.Verifica = mvpCuadroLejos

			mvpVivosEnLaCueva = questBichosConVida()

			println("JUEGO138: MVP ARMA la cueva en el cuadro", guionDeLaQuest.Cueva, "las bajas desde el",
				guionDeLaQuest.Mata, "y la cueva limpia en el", guionDeLaQuest.Limpia,
				"vivosEnLaCueva=", mvpVivosEnLaCueva, "bichos=", len(bichos),
				"(el bicho 1 murio en el pueblo para el combate del MVP; el 2 muere en la cueva)")
		}

		if mvpCuadrosEtapa > 6 {
			mvpIrA(mvpLimpia, "la cueva queda limpia: `Tick(8)` pasa la quest a \"volve con Akara\"")
		}
	case mvpLimpia:
		if questCompletada || mvpCuadrosEtapa > 6 {
			mvpMarca8()
			mvpIrA(mvpCaminaDeVuelta, "la Den of Evil avanzo y la recompensa quedo pendiente: el heroe camina otra vez")
			mvpVaHaciaX, mvpVaHaciaY = mvpArranqueX, mvpArranqueY
			mvpCamina = true
			mvpCeldasDeLaVueltaIni = mvpCeldasCaminadas
		}
	case mvpCaminaDeVuelta:
		if mvpCuadrosEtapa > 5 || distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), mvpArranqueX, mvpArranqueY) <= 1.0 {
			mvpCamina = false
			mvpCeldasDeLaVuelta = mvpCeldasCaminadas - mvpCeldasDeLaVueltaIni
			mvpIrA(mvpTerminado, "las nueve acciones corrieron: los tres cuadros del cierre son la prueba del mundo")

			// LA TABLA NO SE IMPRIME ACA. La marca 9 se mide en los cuadros del cierre del POC (dos cuadros mas
			// abajo), asi que la tabla DEFINITIVA la imprime `mvpMarca9`. Aca se dice como va la cuenta hasta ahora:
			// si el cierre no llegara, el registro tiene todas las marcas una por una igual.
			si := 0
			for i := 1; i <= mvpMarcas; i++ {
				if mvpMarca[i] {
					si++
				}
			}

			println("JUEGO138: MVP GUION-TERMINADO cuadro=", cuadro+1, "marcasHastaAca=", si, "de", mvpMarcas,
				"la 9 se mide en los cuadros del cierre (", cuadros+1, "..", cuadros+3, ")")

			// EL GUION CIERRA LA CORRIDA. `cuadros` es el presupuesto del guion y los tres cuadros de DESPUES
			// (`cuadros+1..+3`) son el cierre del POC: el origen, la repetibilidad y la vuelta. Cuando el guion
			// termina antes del presupuesto no se dejan cuadros muertos: se adelanta el reloj al ultimo y el
			// cierre corre detras. El log dice el cuadro en el que se cerro (`cuadrosDeLaCorrida` de la tabla).
			if cuadro < cuadros {
				println("JUEGO138: MVP CIERRA la corrida en el cuadro", cuadro+1, "de", cuadros,
					": los tres cuadros del cierre (origen, repetibilidad y vuelta) van ahora")
				cuadro = cuadros - 1
			}
		}
	}
}

// mvpIrA: cambia de etapa imprimiendo el porque (una etapa sin motivo no se puede auditar) y arranca su reloj.
func mvpIrA(etapa int, motivo string) {
	println("JUEGO138: MVP ETAPA de=", nombreDeLaEtapa[mvpEtapa], "a=", nombreDeLaEtapa[etapa], "cuadro=",
		cuadro+1, "celdasDeLaEtapa=", mvpCeldasDeLaEtapa, "cuadrosDeLaEtapa=", mvpCuadrosEtapa, "porque=", motivo)

	mvpEtapa = etapa
	mvpCuadrosEtapa = 0
	mvpCeldasDeLaEtapa = 0
	mvpMsEtapa = int64(d2cMs())
	mvpCeldaAntesX, mvpCeldaAntesY = celdaDelJugadorX(), celdaDelJugadorY()
}

// mvpAviso: algo no salio como el guion esperaba. Se dice y se sigue: una corrida que se corta no mide nada.
func mvpAviso(que string) {
	println("JUEGO138: MVP AVISO cuadro=", cuadro+1, "etapa=", nombreDeLaEtapa[mvpEtapa], "aviso=", que)
}

// ============================== LOS BITS DEL MVP ==============================

// bitsDelMVP: los bits de la cruceta que la ETAPA pide, o `false` si esta etapa no camina (ahi manda el guion de
// siempre o el instrumento de la recogida). La cuenta es la inversa de la proyeccion del piso, la MISMA de
// `bitsHaciaElObjeto` (DC-002.123): la diferencia en celdas se proyecta a la pantalla con (dx-dy)*80 y (dx+dy)*40.
func bitsDelMVP(p *participante) (uint32, bool) {
	if mvpKnob != "si" || !mvpCamina || p == nil || p.Mov == nil || p.Mov.Est == nil {
		return 0, false
	}

	cx, cy := p.Mov.Est.Celda()
	dx, dy := mvpVaHaciaX-cx, mvpVaHaciaY-cy

	if dx*dx+dy*dy < 0.0001 {
		return 0, false
	}

	px := (dx - dy) * float64(ladoCeldaX)
	py := (dx + dy) * float64(ladoCeldaY)

	bits := uint32(0)

	if px > 1 {
		bits |= bitDerecha
	} else if px < -1 {
		bits |= bitIzquierda
	}

	if py > 1 {
		bits |= bitAbajo
	} else if py < -1 {
		bits |= bitArriba
	}

	return bits, bits != 0
}

// ============================== LO QUE SE ANOTA EN EL CUADRO ==============================

// anotarElCuadroDelMVP: el cuadro YA DIBUJADO. Guarda el crc32 del lienzo, los pixeles del HUD y del minimapa (lo
// que la marca 9 necesita) y, cuando la etapa lo pide, deja la evidencia de la marca con el cuadro exacto en el que
// se vio. Se llama al final de `dibujar`, cuando ya se pinto TODO (paneles, HUD y minimapa).
func anotarElCuadroDelMVP(pixeles []byte, crc, crcMapa uint32, pxHUD, pxMinimapa, pxDialogo, llenos int) {
	if mvpKnob != "si" {
		return
	}

	mvpPxHUDDelCuadro = pxHUD
	mvpPxMinimapaDelCuadro = pxMinimapa
	mvpLlenosDelCuadro = llenos
	mvpPxDelPanel = pxDialogo

	if pxDialogo > mvpPxDelPanelMax {
		mvpPxDelPanelMax = pxDialogo
	}

	mvpCrcDelCuadro = crc
	mvpCrcDelMapaDelCuadro = crcMapa

	if cuadro == 1 {
		mvpCrcDelMapaAlArrancar = crcMapa
		mvpLlenosAlArrancar = llenos
		mvpMarcaCrcDelMapa[0] = crcMapa

		println("JUEGO138: MVP CUADRO-1 worldCrc=", int(crc), "mapaCrc=", int(crcMapa), "llenos=", llenos,
			"de", ancho*alto, "pxHUD=", pxHUD, "pxMinimapa=", pxMinimapa, "celda=", celdaDelJugadorX(),
			celdaDelJugadorY(), "hostCrc=", crcHost, "(el del host es la celda 28,20: aca NO se compara)")
	}

	// LA MARCA 9 SE ANOTA EN LOS CUADROS DEL CIERRE, no al final del guion: lo que hay que probar es que el mundo y
	// el HUD siguen bien DESPUES de todo, y eso se ve en el cuadro del cierre.
	if cuadro == cuadros+1 {
		mvpCrcCierre1 = crc
		mvpCrcDelMapaAlCerrar = crcMapa
		mvpLlenosAlCerrar = llenos
		mvpPxHUDAlCerrar = pxHUD
		mvpPxMinimapaAlCerrar = pxMinimapa
	}

	if cuadro == cuadros+2 {
		mvpCrcCierre2 = crc
		mvpCrcDelMapaCierre2 = crcMapa
		mvpMarca9()
	}
}

// ============================== LAS NUEVE MARCAS ==============================

// mvpMarca1: EL CAMINO. Lo que se mide es lo que el motor movio: celdas, celdas por segundo y las direcciones del
// COF que se usaron (la cruceta tiene 8 estados y el COF del heroe, 16 direcciones).
func mvpMarca1() {
	mvpMarca[1] = mvpCeldasCaminadas > 0 && mvpCuadrosCaminando > 0
	mvpMarcaCuadro[1] = cuadro + 1
	mvpMarcaCeldas[1] = "celdas=" + ftoa(mvpCeldasCaminadas) + " cuadrosCaminando=" + itoa(mvpCuadrosCaminando) +
		" direccionesDelCOF=" + itoa(mvpDireccionesUsadas)
	mvpMarcaCrc[1] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[1] = mvpCrcDelMapaDelCuadro
	mvpMarcaEvidencia[1] = "el heroe camino " + ftoa(mvpCeldasDeLaEtapa) + " celdas hasta el bicho con los bits de " +
		"la cruceta (maple -> servicio -> Paso, 1,875 celdas/cuadro); modo= " + modoDelCuadro(true)

	println("JUEGO138: MVP 1 CAMINA cumple=", siNo(mvpMarca[1]), "cuadro=", cuadro+1, "celdas=",
		mvpCeldasCaminadas, "cuadrosCaminando=", mvpCuadrosCaminando, "direccionesDelCOF=", mvpDireccionesUsadas,
		"crc=", int(mvpMarcaCrc[1]))
	println("JUEGO138: MVP 1 DETALLE", mvpMarcaEvidencia[1])
}

// mvpMarca2: LA MUERTE. La vida del Returned de la tabla llega a 0 y el cadaver pasa por el DT (19 cuadros del
// motor, uno por cuadro del POC con `muertePorCuadro`).
func mvpMarca2() {
	vida, modo := 0, "?"

	if len(bichos) > 0 {
		if bichos[0].Vida != nil {
			vida = bichos[0].Vida.HP
		}

		modo = bichos[0].Modo
	}

	mvpCuadrosDT = 0
	dtDibujados := 0

	if len(bichos) > 0 && bichos[0].Muerte != nil {
		mvpCuadrosDT = bichos[0].Muerte.CuadrosDT
		dtDibujados = bichos[0].Muerte.Dibujados
	}

	mvpMarca[2] = vida <= 0 && modo == modoBestiaMuerta
	mvpMarcaCuadro[2] = cuadro + 1
	mvpMarcaCrc[2] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[2] = mvpCrcDelMapaDelCuadro
	mvpMarcaCeldas[2] = "vida=" + itoa(vida) + " modo=" + modo + " cuadrosDT=" + itoa(mvpCuadrosDT) +
		" dtDibujados=" + itoa(dtDibujados)
	mvpMarcaEvidencia[2] = "el Returned (vida de la tabla " + itoa(mvpVidaInicial) + ") quedo en " + itoa(vida) +
		" y el cadaver en " + modo + ": " + itoa(dtDibujados) + " de " + itoa(mvpCuadrosDT) + " cuadros del DT"

	println("JUEGO138: MVP 2 MATA cumple=", siNo(mvpMarca[2]), "cuadro=", cuadro+1, "vidaDelBicho=", vida,
		"modo=", modo, "cuadrosDT=", mvpCuadrosDT, "crc=", int(mvpMarcaCrc[2]))
	println("JUEGO138: MVP 2 DETALLE", mvpMarcaEvidencia[2])
}

// mvpMarca3: LA RECOGIDA. El objeto que solto el cadaver deja de estar en el piso y pasa al inventario del
// jugador, por la regla del motor (`pickupRange`, 0,9 celdas medida sobre el paso).
func mvpMarca3() {
	mvpMarca[3] = hayObjetoRecogido()

	for _, it := range suelo {
		mvpObjeto = it.Codigo + " (" + it.Nombre + ")"
		mvpCeldaDelObjeto = ftoa(it.CeldaX) + "," + ftoa(it.CeldaY)

		if it.Recogido {
			mvpMarcaCuadro[3] = it.CuadroRecogida
		}
	}

	mvpMarcaCrc[3] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[3] = mvpCrcDelMapaDelCuadro
	mvpMarcaCeldas[3] = "objeto=" + mvpObjeto + " celdaDelPiso=" + mvpCeldaDelObjeto + " enElSuelo=" +
		itoa(enElSuelo())
	mvpMarcaEvidencia[3] = "el objeto " + mvpObjeto + " que cayo en " + mvpCeldaDelObjeto +
		" esta en el inventario del jugador (suelo=" + itoa(enElSuelo()) + ")"

	println("JUEGO138: MVP 3 RECOGE cumple=", siNo(mvpMarca[3]), "cuadro=", mvpMarcaCuadro[3], "objeto=",
		mvpObjeto, "celdaDelPiso=", mvpCeldaDelObjeto, "enElSuelo=", enElSuelo(), "crc=", int(mvpMarcaCrc[3]))
	println("JUEGO138: MVP 3 DETALLE", mvpMarcaEvidencia[3])
}

// mvpMarca4: EL INVENTARIO. El panel del jugador se abrio y el objeto esta EN SU CELDA (la grilla de 10x4 que
// dice `d2inventory.Dims(ContainerInventory)`). No se cuenta cuantas celdas ocupa -- las flechas (`aqv`) tienen
// huella 1x3 y ocupan TRES --: se mira que la primera celda ocupada sea la del objeto recogido.
func mvpMarca4() {
	ocupadas := -1
	codigoEnLaGrilla, hay := "", false

	if grillaJugador != nil {
		ocupadas = grillaJugador.ocupadas()
		mvpCeldaEnLaGrilla = detalleDeUnaGrilla(grillaJugador)

		_, _, codigoEnLaGrilla, hay = grillaJugador.primeraCelda()
	}

	codigoDelObjeto := ""
	if enLaGrilla != nil {
		codigoDelObjeto = enLaGrilla.Codigo
	}

	esElObjeto := hay && (codigoDelObjeto == "" || codigoEnLaGrilla == codigoDelObjeto)

	mvpMarca[4] = grillaJugador != nil && grillaJugador.Veces > 0 && esElObjeto
	mvpMarcaCuadro[4] = cuadro + 1
	mvpMarcaCrc[4] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[4] = mvpCrcDelMapaDelCuadro
	mvpMarcaCeldas[4] = "grilla=" + mvpCeldaEnLaGrilla + " ocupadas=" + itoa(ocupadas) + " abierta=" +
		siNo(grillaJugador != nil && grillaJugador.Abierta)
	mvpMarcaEvidencia[4] = "el panel del inventario abrio " + itoa(grillaJugador.Veces) +
		" vez/veces y mostro el objeto en su celda (" + mvpCeldaEnLaGrilla +
		"); los pixeles que pinto el panel los mide DC-002.124 en `JUEGO124: INV CUADRO` del mismo cuadro"
	mvpMarcaEvidencia[4] += "; despues se cerro y dejo el mundo como estaba"

	println("JUEGO138: MVP 4 INVENTARIO cumple=", siNo(mvpMarca[4]), "cuadro=", mvpMarcaCuadro[4], "grilla=",
		mvpCeldaEnLaGrilla, "ocupadas=", ocupadas, "pxDelPanelMax=", mvpPxDelPanelMax, "crc=", int(mvpMarcaCrc[4]))
	println("JUEGO138: MVP 4 DETALLE", mvpMarcaEvidencia[4])
}

// mvpMarca5: EL ALIJO. El objeto viajo del inventario (10x4) al cofre del pueblo (6x8) y volvio: son DOS grillas y
// UN objeto, sin una segunda copia.
func mvpMarca5() {
	ocupadasJugador, ocupadasAlijo := -1, -1

	if grillaJugador != nil {
		ocupadasJugador = grillaJugador.ocupadas()
	}

	if grillaAlijo != nil {
		ocupadasAlijo = grillaAlijo.ocupadas()

		if mvpCeldaEnElAlijo == "" {
			mvpCeldaEnElAlijo = detalleDeUnaGrilla(grillaAlijo)
		}
	}

	mvpMarca[5] = guardoElAlijo && devolvioElAlijo
	mvpMarcaCuadro[5] = cuadro + 1
	mvpMarcaCrc[5] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[5] = mvpCrcDelMapaDelCuadro
	mvpMarcaCeldas[5] = "guardo=" + siNo(guardoElAlijo) + " devolvio=" + siNo(devolvioElAlijo) +
		" ocupadasInventario=" + itoa(ocupadasJugador) + " ocupadasAlijo=" + itoa(ocupadasAlijo)
	mvpMarcaEvidencia[5] = "el objeto entro al cofre del pueblo (alijo= " + mvpCeldaEnElAlijo +
		") y volvio al inventario: el cofre queda vacio y el objeto, en la grilla del jugador; los pixeles de cada" +
		" panel (34.560 el del inventario y 40.832 el del cofre) los mide DC-002.124 en sus `JUEGO124: INV CUADRO`"

	println("JUEGO138: MVP 5 ALIJO cumple=", siNo(mvpMarca[5]), "cuadro=", mvpMarcaCuadro[5], "guardo=",
		siNo(guardoElAlijo), "devolvio=", siNo(devolvioElAlijo), "ocupadasAlijo=", ocupadasAlijo,
		"crc=", int(mvpMarcaCrc[5]))
	println("JUEGO138: MVP 5 DETALLE", mvpMarcaEvidencia[5])
}

// mvpMarca6a: EL CAMINO A AKARA. Se anota aparte porque la marca 6 es hablar: aca queda la distancia con la que se
// llego (y si el camino se trabo, el numero lo dice).
func mvpMarca6a() {
	d := distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), mvpAkaraX, mvpAkaraY)

	println("JUEGO138: MVP 6 LLEGA-A-AKARA jugador=", celdaDelJugadorX(), celdaDelJugadorY(), "akara=",
		mvpAkaraX, mvpAkaraY, "distancia=", d, "radio=", radioQuest, "dentroDelRadio=", siNo(d <= radioQuest),
		"celdasCaminadas=", mvpCeldasDeLaEtapa, "cuadros=", mvpCuadrosEtapa)
}

// mvpMarca6b: HABLAR CON AKARA. El panel abrio con la pagina del log de la quest y el panel se pinto (px > 0).
func mvpMarca6b() {
	q := questJuego.Quest(d2quest.QuestDenOfEvil)

	if q != nil {
		mvpPaginaAlHablar = questJuego.LogPage(q)
	}

	d := distanciaEntre(celdaDelJugadorX(), celdaDelJugadorY(), mvpAkaraX, mvpAkaraY)

	mvpMarca[6] = d <= radioQuest && mvpPaginaAlHablar > 0
	mvpMarcaCuadro[6] = cuadro + 1
	mvpMarcaCrc[6] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[6] = mvpCrcDelMapaDelCuadro
	mvpMarcaCeldas[6] = "distancia=" + ftoa(d) + " paginaDelLog=" + itoa(mvpPaginaAlHablar)
	mvpMarcaEvidencia[6] = "el heroe hablo con Akara a " + ftoa(d) + " celdas (radio " + ftoa(radioQuest) +
		"): el panel abrio la pagina " + itoa(mvpPaginaAlHablar) + " del log de la quest"

	println("JUEGO138: MVP 6 HABLA cumple=", siNo(mvpMarca[6]), "cuadro=", cuadro+1, "distancia=", d,
		"pagina=", mvpPaginaAlHablar, "pxDelPanel=", mvpPxDelPanel, "crc=", int(mvpMarcaCrc[6]))
	println("JUEGO138: MVP 6 DETALLE", mvpMarcaEvidencia[6])
}

// mvpMarca7: ACEPTAR. El mensaje 64 del nodo A1Q1 deja la quest en el registro del guardado (`FlagStarted`).
func mvpMarca7() {
	estado := "?"

	q := questJuego.Quest(d2quest.QuestDenOfEvil)
	if q != nil {
		estado = estadoDeLaQuest(q)
	}

	mvpMarca[7] = questAceptada
	mvpMarcaCuadro[7] = cuadro + 1
	mvpMarcaCrc[7] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[7] = mvpCrcDelMapaDelCuadro
	mvpMarcaCeldas[7] = "aceptada=" + siNo(questAceptada) + " estado=" + estado
	mvpMarcaEvidencia[7] = "`Hear(64)` del nodo A1Q1: la Den of Evil quedo aceptada en el registro (" + estado + ")"

	println("JUEGO138: MVP 7 ACEPTA cumple=", siNo(mvpMarca[7]), "cuadro=", cuadro+1, "estado=", estado,
		"crc=", int(mvpMarcaCrc[7]))
	println("JUEGO138: MVP 7 DETALLE", mvpMarcaEvidencia[7])
}

// mvpMarca8: AVANZAR. El evento de area (pueblo -> cueva) y las bajas de la cueva llevan la quest a la pagina 5
// ("volve con Akara"), con la recompensa pendiente y el marcador de Akara encendido.
func mvpMarca8() {
	pagina, estado := 0, "?"
	pendiente, marcador := false, false

	q := questJuego.Quest(d2quest.QuestDenOfEvil)
	if q != nil {
		pagina = questJuego.LogPage(q)
		estado = estadoDeLaQuest(q)
		pendiente = questJuego.Rec.Get(q.Slot, d2quest.FlagRewardPending)
		marcador = questJuego.Marker(d2quest.NPCAkara)
	}

	mvpPaginaAlLimpia = pagina

	mvpMarca[8] = questCompletada && pagina >= paginaLimpiaQuest
	mvpMarcaCuadro[8] = cuadro + 1
	mvpMarcaCrc[8] = mvpCrcDelCuadro
	mvpMarcaCrcDelMapa[8] = mvpCrcDelMapaDelCuadro
	mvpMarcaCeldas[8] = "completada=" + siNo(questCompletada) + " pagina=" + itoa(pagina) + " pendiente=" +
		siNo(pendiente)
	mvpMarcaEvidencia[8] = "la Den of Evil avanzo: cueva (area 8), bajas de la cueva, pagina " + itoa(pagina) +
		" del log, recompensa pendiente=" + siNo(pendiente) + " y el marcador de Akara=" + siNo(marcador) +
		"; estado=" + estado

	println("JUEGO138: MVP 8 AVANZA cumple=", siNo(mvpMarca[8]), "cuadro=", cuadro+1, "completada=",
		siNo(questCompletada), "pagina=", pagina, "pendiente=", siNo(pendiente), "marcador=", siNo(marcador),
		"vivosEnLaCuevaAlEntrar=", mvpVivosEnLaCueva, "muertes=", questMuertes, "crc=", int(mvpMarcaCrc[8]))
	println("JUEGO138: MVP 8 DETALLE", mvpMarcaEvidencia[8])
}

// mvpMarca9: EL MUNDO Y EL HUD DESPUES DE TODO. Se mide en el cuadro del cierre: el heroe volvio a caminar, el HUD
// y el minimapa pintan, y el CUADRO DEL MAPA se repite identico entre cuadros+1 y cuadros+2. Se compara el mapa y
// no el lienzo entero porque el heroe QUIETO tiene su ciclo NU corriendo por reloj: su cuadro de animacion cambia
// en cada cuadro del POC, asi que el lienzo completo no puede repetirse nunca (medido: `JUEGO121: REPETIBILIDAD`
// da `igualAlCuadroAnterior= true` para el mapa y `igualConJugador= false`).
func mvpMarca9() {
	if mvpCierreImpreso {
		return
	}

	mvpCierreImpreso = true

	repite := mvpCrcDelMapaAlCerrar != 0 && mvpCrcDelMapaAlCerrar == mvpCrcDelMapaCierre2

	mvpMarca[9] = mvpCeldasDeLaVuelta > 0 && mvpPxHUDAlCerrar > 0 && mvpPxMinimapaAlCerrar > 0 && repite
	mvpMarcaCuadro[9] = cuadro
	mvpMarcaCrc[9] = mvpCrcCierre2
	mvpMarcaCrcDelMapa[9] = mvpCrcDelMapaAlCerrar
	mvpMarcaCeldas[9] = "celdasDeLaVuelta=" + ftoa(mvpCeldasDeLaVuelta) + " pxHUD=" + itoa(mvpPxHUDAlCerrar) +
		" pxMinimapa=" + itoa(mvpPxMinimapaAlCerrar) + " elMapaSeRepite=" + siNo(repite)
	mvpMarcaEvidencia[9] = "despues de las nueve acciones el heroe camino " + ftoa(mvpCeldasDeLaVuelta) +
		" celdas mas; el HUD pinto " + itoa(mvpPxHUDAlCerrar) + " px y el minimapa " + itoa(mvpPxMinimapaAlCerrar) +
		" px; y el cuadro del mapa se repite identico en dos cuadros seguidos (crc " +
		itoa(int(mvpCrcDelMapaAlCerrar)) + " dos veces, con " + itoa(mvpLlenosAlCerrar) + " px no vacios)"

	println("JUEGO138: MVP 9 VUELVE-A-CAMINAR cumple=", siNo(mvpMarca[9]), "cuadro=", cuadro,
		"celdasDeLaVuelta=", mvpCeldasDeLaVuelta, "pxHUD=", mvpPxHUDAlCerrar, "pxMinimapa=",
		mvpPxMinimapaAlCerrar, "mapaCrcCierre1=", int(mvpCrcDelMapaAlCerrar), "mapaCrcCierre2=",
		int(mvpCrcDelMapaCierre2), "repite=", siNo(repite), "lienzoCrc1=", int(mvpCrcCierre1), "lienzoCrc2=",
		int(mvpCrcCierre2), "mapaCrcAlArrancar=", int(mvpCrcDelMapaAlArrancar), "llenos=", mvpLlenosAlCerrar)
	println("JUEGO138: MVP 9 DETALLE", mvpMarcaEvidencia[9])

	if !mvpTablaImpresa {
		mvpImprimirLaTabla()
	}
}

// ============================== LA TABLA ==============================

// mvpImprimirLaTabla: LA TABLA DEL MVP. Una linea por marca, con el si/no MEDIDO y la evidencia que lo sostiene, y
// el total. Es lo que el runner del host convierte en la tabla de la tarjeta.
func mvpImprimirLaTabla() {
	mvpTablaImpresa = true

	nombres := [mvpMarcas + 1]string{"", "CAMINA", "MATA", "RECOGE", "INVENTARIO", "ALIJO", "HABLA", "ACEPTA",
		"AVANZA", "VUELVE-A-CAMINAR"}

	si := 0

	for i := 1; i <= mvpMarcas; i++ {
		if mvpMarca[i] {
			si++
		}

		println("JUEGO138: MVP TABLA", i, nombres[i], siNo(mvpMarca[i]), "cuadro=", mvpMarcaCuadro[i],
			"crc=", int(mvpMarcaCrc[i]), "medido=", mvpMarcaCeldas[i])
	}

	completo := si == mvpMarcas

	println("JUEGO138: MVP TABLA TOTAL marcas=", mvpMarcas, "cumplidas=", si, "mvpCompleto=", siNo(completo),
		"cuadrosDeLaCorrida=", mvpCuadrosTotales, "cuadrosDelGuion=", cuadros,
		"celdasCaminadas=", mvpCeldasCaminadas, "direccionesDelCOF=", mvpDireccionesUsadas,
		"objeto=", mvpObjeto, "paginaDeLaQuest=", mvpPaginaAlLimpia)
	println("JUEGO138: MVP CRITERIO unaCorrida= si nueveAcciones=", siNo(completo), "de", mvpMarcas,
		"mundoYHUDDespuesDeTodo=", siNo(mvpMarca[9]), "mvp=", siNo(completo))
}
