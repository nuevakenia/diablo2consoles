//go:build dreamcast

package main

// DC-002.147 -- EL BLOOD MOOR POR REGIONES QUE SE CAMBIAN EN CALIENTE (sin volver a compilar).
//
// QUE FALTABA. DC-002.145 dejo UNA region del Blood Moor de verdad en la consola (32 x 32 desde 24,32) compuesta
// con el plan que el DRLG calcula en el host. El plan estaba cableado en el binario y la region no se podia
// cambiar: para ver la vecina habia que regenerar el `.go` del plan y recompilar. Esta tarjeta hace que el cambio
// de region sea lo que dice el cuerpo de la tarjeta: VOLVER A LLAMAR `ResetMapParaTilesExactos` y reescribir las
// casillas con `SetExactTiles`, con el plan de la region que toca ya adentro del binario.
//
// LAS TRES PIEZAS.
//
//  1. EL NIVEL ENTERO ES DATO. `moor_regiones_147.go` (generado por `moor-drlg -plan-regiones`) trae las SEIS
//     regiones que cubren las 56 x 96 casillas del nivel, en FORMA PLANA: 8 `uint16` por casilla y 3 por registro
//     de tile. Medido en el host: 118.630 B para las 5.056 casillas con algo y sus 6.289 registros. La forma de
//     DC-002.145 (`[]moorCelda` con tres rebanadas por casilla) costaba 45.056 B de `.data` por UNA region de
//     1.024 casillas, o sea ~222.000 B para el nivel: la forma plana es la mitad, no reserva nada al arrancar y
//     es la que deja el nivel entero EMBEBIDO sin pedirle nada al disco ni a la VMU (118.630 B, menos que los
//     122.022 B de la tabla del automapa que el binario ya trae; el numero esta en la evidencia).
//
//  2. LA REJILLA NO SE SOLAPA. `moor-drlg` arma la rejilla sin solaparse: 56 = 24 + 32, asi que la columna del
//     oeste mide 24 y la del este 32; el alto son 96 = 32 + 32 + 32. Da SEIS regiones, y la cuarta es EXACTAMENTE
//     la region de DC-002.145 (32 x 32 desde 24,32), asi que su evidencia sigue valiendo (arena 1.534.112 B).
//     La rejilla sin solapes es lo que hace que "en que region esta el jugador" tenga UNA respuesta.
//
//  3. EL CAMBIO. Cada cuadro, despues del movimiento, se mira si el jugador se salio de la caja de la region que
//     esta puesta (`demoMiraElBordeDeLaRegion`). Si se salio, la region destino es la que CONTIENE su celda del
//     NIVEL (region + celda local) y se compone: se suelta lo del nivel que se deja (cache del renderer, arena de
//     bloques, casillas), se llama `ResetMapParaTilesExactos` con las claves y el buffer de la region nueva, se
//     escriben sus casillas y se vuelve a colgar del renderer. El jugador NO se mueve del mundo: se le resta el
//     origen de la region nueva a su celda del nivel, asi que cruza el borde caminando y sigue en la misma piedra.
//
// LO QUE NO HACE. No compone dos regiones a la vez: la RAM de la consola no tiene lugar para la arena de bloques
// de dos regiones (la mas barata pide 1,1 MB y la mas cara 1,9 MB contra ~3,0 MB libres en el cruce). La costura
// entre regiones, entonces, se ve cuando el jugador camina pegado al borde (la ventana son ~20 x 15 casillas y la
// region mide 24-32). Y no genera: el plan sigue viniendo del DRLG en el host.

import (
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// moorPlanRegion es una region del nivel, con la forma que emite `moor-drlg -plan-regiones`: la geometria, el
// spawn, el buffer prestado de lectura y las dos listas planas. `Celdas` va de a 8 enteros por casilla
// (x, y, (offset, largo) de piso, de pared y de sombra sobre `Tiles`) y `Tiles` de a 3 por registro
// (pieza = indice de `Piezas`, indice del tile en el `.dt1`, flags de colision).
type moorPlanRegion struct {
	X, Y, Ancho, Alto int
	SpawnX, SpawnY    int
	Buffer, Registros int
	Piezas            []string
	Claves            []int64
	Celdas            []uint16
	Tiles             []uint16
}

// moorRegionInicialX/Y: la region con la que arranca la demo del moor (la de DC-002.145). No es la (0,0) a
// proposito: la evidencia de la primera composicion se compara con la de esa tarjeta.
const (
	moorRegionInicialX = 24
	moorRegionInicialY = 32
)

var (
	// moorRegionActual: la region que el motor tiene compuesta. nil mientras el nivel es el pueblo.
	moorRegionActual *moorPlanRegion

	// los contadores del streaming (van al informe y al log)
	moorCambiosDeRegion int
	moorBordesDelNivel  int

	// moorLibreAntes/moorSemiAntes: el monton y el semiespacio justo ANTES de componer la region que esta puesta.
	// Van en la linea `CRUZA OK` para que el antes/despues de cada cambio se lea en la misma linea.
	moorLibreAntes = -1
	moorSemiAntes  = -1

	// los tres buffers que se reusan por casilla: pedir tres rebanadas nuevas por casilla son 1.024 `make` en una
	// region de 32 x 32 y el semiespacio del recolector no esta para eso (DC-002.120 lo midio con las casillas
	// del pueblo: ~650.000 B para 2.337 casillas).
	moorPisos   []d2mapengine.ExactTile
	moorParedes []d2mapengine.ExactTile
	moorSombras []d2mapengine.ExactTile
)

// demoComponeLaRegionDelMoor: la PRIMERA region del moor, la del plan de DC-002.145. El pueblo ya lo solto
// `demoCruzaAlMoor` (cache de tiles, arena y casillas), asi que aca solo se pide lo del nivel nuevo.
func demoComponeLaRegionDelMoor(t0 int64) bool {
	return demoComponeRegion(moorRegionEn(moorRegionInicialX, moorRegionInicialY), t0, 0, 0, true)
}

// moorRegionEn: la region de la rejilla que arranca en (x, y), o nil si no hay ninguna ahi.
func moorRegionEn(x, y int) *moorPlanRegion {
	for i := range moorPlanes147 {
		if moorPlanes147[i].X == x && moorPlanes147[i].Y == y {
			return &moorPlanes147[i]
		}
	}

	return nil
}

// moorRegionQueContiene: la region cuyo rectangulo (en casillas del NIVEL) contiene ese punto. La rejilla no se
// solapa, asi que la respuesta es una sola; nil es "afuera del nivel".
func moorRegionQueContiene(gx, gy float64) *moorPlanRegion {
	for i := range moorPlanes147 {
		pl := &moorPlanes147[i]
		if gx >= float64(pl.X) && gy >= float64(pl.Y) &&
			gx < float64(pl.X+pl.Ancho) && gy < float64(pl.Y+pl.Alto) {
			return pl
		}
	}

	return nil
}

// demoComponeRegion: deja el mapa del motor con ESTA region compuesta. Es el camino de DC-002.145 generalizado:
// `primera` = viene del cruce del pueblo (se compone en el spawn del plan y la interfaz se rearma como nivel
// nuevo); con `primera` en false es un CAMBIO DE REGION (el jugador queda en su celda del nivel, `gx, gy`) y la
// interfaz NO se rearma: la tabla y el arte del minimapa son del MISMO nivel, y las celdas que se revelan van en
// coordenadas del nivel (ver `fuenteDelAutomapa`).
func demoComponeRegion(pl *moorPlanRegion, t0 int64, gx, gy float64, primera bool) bool {
	if motorDelMapa == nil || rendererDelMapa == nil || gestorDeAssets == nil {
		println("DEMO140: REGION SIN MOTOR (motor/renderer/gestor nulos): no se compone")

		return false
	}

	if pl == nil {
		println("DEMO140: REGION SIN PLAN para esa casilla: no se compone")

		return false
	}

	moorRegionActual = pl

	libre := d2cMontonLibre()

	moorLibreAntes, moorSemiAntes = libre, int(d2cGcUsado())

	println("DEMO140: REGION PLAN delDRLG nivel=", moorNivel147, "semilla=", moorSemilla147,
		"region=", pl.Ancho, "x", pl.Alto, "desde=", pl.X, pl.Y,
		"delNivel=", moorNivelAncho147, "x", moorNivelAlto147,
		"casillas=", len(pl.Celdas)/8, "registros=", pl.Registros,
		"piezas=", len(pl.Piezas), "claves=", len(pl.Claves))

	// ---- EL ORDEN DE LOS DOS PEDIDOS GRANDES (DC-002.144b, medido y con A/B) ----
	// El buffer prestado del `.dt1` mas grande se pide ANTES de los tramos del arena cuando la palanca esta en
	// "si": cae en el hueco fresco que dejo el arena del nivel anterior y los tramos crecen detras. Con "no"
	// queda el camino de DC-002.111 (un buffer por archivo, del tamano de ESE archivo).
	if moorScratchPrimero == "si" {
		d2asset.UsarScratchPorArchivo(false)
	} else {
		d2asset.UsarScratchPorArchivo(true)
	}

	println("DEMO140: REGION BUFFER primero=", moorScratchPrimero == "si", "delDT1MasGrande=", pl.Buffer,
		"libre=", libre, "servible=", int(d2cServible(uint(pl.Buffer))), "vramLibre=", pvr.VramDisponible())

	// el mapa del nivel nuevo, sin la lista de `.dt1` del level type: el arena se dimensiona con las claves de
	// la region y los `.dt1` los carga el motor cuando el primer tile de cada uno lo pide.
	d2mapengine.ResetMapParaTilesExactos(motorDelMapa, d2enum.RegionAct1Wilderness, pl.Ancho, pl.Alto, pl.Claves,
		pl.Buffer)

	// ---- LAS CASILLAS, UNA POR UNA ----
	conPiso := 0
	fallas := 0

	for i := 0; i+8 <= len(pl.Celdas); i += 8 {
		cx, cy := int(pl.Celdas[i]), int(pl.Celdas[i+1])

		moorPisos = moorRegistrosDeLaRegion(pl, int(pl.Celdas[i+2]), int(pl.Celdas[i+3]), moorPisos[:0])
		moorParedes = moorRegistrosDeLaRegion(pl, int(pl.Celdas[i+4]), int(pl.Celdas[i+5]), moorParedes[:0])
		moorSombras = moorRegistrosDeLaRegion(pl, int(pl.Celdas[i+6]), int(pl.Celdas[i+7]), moorSombras[:0])

		motorDelMapa.SetExactTiles(cx, cy, d2enum.RegionAct1Wilderness, false, moorPisos, moorParedes, moorSombras)

		if len(moorPisos) > 0 {
			conPiso++
		}

		if motorDelMapa.Tile(cx, cy) == nil {
			fallas++
		}
	}

	// el buffer prestado ya no lo mira nadie: los bloques que la region usa estan COPIADOS al arena.
	arena := d2asset.EstadoDeLosTiles()
	bufferSoltado := d2asset.SoltarPrestamoDeLecturaDeTiles()

	println("DEMO140: REGION ARENA copiado=", arena.BloquesBytes, "tramos=", arena.BloquesTramos,
		"capacidad=", arena.BloquesCapacidad, "bufferSoltado=", bufferSoltado,
		"libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()))

	rendererDelMapa.SetMapEngine(motorDelMapa)

	println("DEMO140: REGION COMPUESTA region=", pl.X, pl.Y, "casillas=", len(pl.Celdas)/8, "conPiso=", conPiso,
		"sinNada=", pl.Ancho*pl.Alto-conPiso, "fallas=", fallas, "tiles=", len(*motorDelMapa.Tiles()),
		"ms=", int64(d2cMs())-t0, "libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()),
		"vramLibre=", pvr.VramDisponible())

	// el jugador: en el cruce, donde el plan dice (el centro de una casilla con piso y sin paredes); en un cambio
	// de region, donde estaba en el NIVEL menos el origen de la region nueva (o sea: cruza caminando, sin salto).
	lx, ly := float64(pl.SpawnX)+0.5, float64(pl.SpawnY)+0.5

	if !primera {
		lx, ly = gx-float64(pl.X), gy-float64(pl.Y)
	}

	demoAcomodaElCruce("BloodMoor:region", lx, ly, t0, primera)

	runtime.GC()

	return true
}

// moorRegistrosDeLaRegion: pasa `largo` registros de la lista plana (desde `off`) a los `ExactTile` del motor,
// escribiendo en el buffer que se le pasa (que se reusa por casilla).
func moorRegistrosDeLaRegion(pl *moorPlanRegion, off, largo int, dst []d2mapengine.ExactTile) []d2mapengine.ExactTile {
	for k := 0; k < largo; k++ {
		b := 3 * (off + k)
		if b < 0 || b+2 >= len(pl.Tiles) {
			break
		}

		pieza := int(pl.Tiles[b])
		if pieza < 0 || pieza >= len(pl.Piezas) {
			continue
		}

		dst = append(dst, d2mapengine.ExactTile{
			File:  pl.Piezas[pieza],
			Index: int(pl.Tiles[b+1]),
			Flags: uint32(pl.Tiles[b+2]),
		})
	}

	return dst
}

// ============================== EL BORDE DE LA REGION (EL CAMBIO EN CALIENTE) ==============================

// demoMiraElBordeDeLaRegion: el disparador del streaming. Se llama cada cuadro DESPUES del movimiento y ANTES de
// la camara: si el jugador se salio de la caja de la region compuesta, se compone la region que contiene su celda
// del nivel. En el borde del NIVEL no hay plan: el jugador se queda en la region (se le recorta la celda) y se
// avisa UNA vez, para que el log no se llene de la misma linea.
func demoMiraElBordeDeLaRegion() {
	if moorRegionActual == nil || !moorStreaming() {
		return
	}

	pl := moorRegionActual

	lx, ly := celdaDelJugadorX(), celdaDelJugadorY()
	if lx >= 0 && lx < float64(pl.Ancho) && ly >= 0 && ly < float64(pl.Alto) {
		return
	}

	gx, gy := float64(pl.X)+lx, float64(pl.Y)+ly

	destino := moorRegionQueContiene(gx, gy)
	if destino == nil || destino == pl {
		demoRecortaEnElBordeDelNivel(pl, lx, ly, gx, gy)

		return
	}

	demoCambiaDeRegion(destino, gx, gy)
}

// demoCambiaDeRegion: EL CAMBIO DE REGION EN CALIENTE. Primero se suelta lo de la region que se deja (la cache de
// tiles del renderer, el arena de bloques y las casillas del motor) y recien despues se compone la nueva, que es
// el orden que DC-002.109/DC-002.120 midieron para que el nivel nuevo entre en la RAM de la consola.
func demoCambiaDeRegion(destino *moorPlanRegion, gx, gy float64) {
	origen := moorRegionActual
	t0 := int64(d2cMs())

	moorCambiosDeRegion++

	println("DEMO140: REGION CAMBIO n=", moorCambiosDeRegion, "de=", origen.X, origen.Y,
		"a=", destino.X, destino.Y, "jugadorEnElNivel=", gx, gy, "cuadro=", cuadro+1,
		"libreAntes=", d2cMontonLibre(), "semiespacioAntes=", int(d2cGcUsado()),
		"vramLibre=", pvr.VramDisponible())

	// ---- lo de la region que se deja ----
	bytes := rendererDelMapa.SoltarLaCacheDelNivel(soltarBloqueDeLaCache)

	gestorDeAssets.ReiniciarArenaDeTiles()
	motorDelMapa.SoltarLasCasillasDelNivelAnterior()

	runtime.GC()

	println("DEMO140: REGION CAMBIO soltado cache=", bytes, "libre=", d2cMontonLibre(),
		"semiespacio=", int(d2cGcUsado()), "vramLibre=", pvr.VramDisponible())

	demoComponeRegion(destino, t0, gx, gy, false)
}

// demoRecortaEnElBordeDelNivel: el jugador camino fuera de las cuatro regiones (o sea, fuera del nivel). Se le
// deja la celda en el borde de la region y se avisa una vez: no hay plan para lo que sigue, y andar por el vacio
// no es un cambio de region.
func demoRecortaEnElBordeDelNivel(pl *moorPlanRegion, lx, ly, gx, gy float64) {
	if len(jugadores) > 0 && jugadores[0].Mov != nil && jugadores[0].Mov.Est != nil {
		nx, ny := lx, ly

		if nx < 0 {
			nx = 0.5
		}

		if nx > float64(pl.Ancho)-0.5 {
			nx = float64(pl.Ancho) - 0.5
		}

		if ny < 0 {
			ny = 0.5
		}

		if ny > float64(pl.Alto)-0.5 {
			ny = float64(pl.Alto) - 0.5
		}

		jugadores[0].Mov.Est = NuevoEstado(nx, ny, false)
	}

	if moorBordesDelNivel == 0 {
		println("DEMO140: REGION BORDE DEL NIVEL jugadorEnElNivel=", gx, gy, "region=", pl.X, pl.Y,
			"no hay plan mas alla: se recorta la celda")
	}

	moorBordesDelNivel++
}

// moorStreaming: la palanca del streaming (`DEMO_MOOR_STREAM=si`, `knobs.go`). Con "no" el camino es el de
// DC-002.145 linea por linea: una sola region, sin borde.
func moorStreaming() bool { return demoMoorStream == "si" }

// regionDelLog: el origen de la region compuesta, para la linea `CRUZA OK` (dos enteros). Con -1, -1 el nivel no
// es el moor (el camino del `.ds1`, DC-002.144b).
func regionDelLog() (int, int) {
	if moorRegionActual == nil {
		return -1, -1
	}

	return moorRegionActual.X, moorRegionActual.Y
}

// ============================== EL AUTOMAPA DEL NIVEL ENTERO ==============================

// enCoordenadasDelNivel: la celda LOCAL del motor -> la celda del NIVEL. El automapa es del NIVEL (sus celdas se
// indexan por casilla del nivel, que es lo que hace el motor de la PC): con la region puesta hay que sumarle el
// origen, o las seis regiones se dibujarian una encima de la otra (todas empiezan en 0,0).
func enCoordenadasDelNivel(hx, hy float64) (float64, float64) {
	if moorRegionActual == nil {
		return hx, hy
	}

	return hx + float64(moorRegionActual.X), hy + float64(moorRegionActual.Y)
}

// fuenteDelAutomapa: el adaptador del automapa sobre el motor, con el origen de la region. Sin region puesta es
// exactamente `fuenteDelMotor` (el pueblo no cambia una linea): el tamano y la celda son los del motor.
type fuenteDelAutomapa struct{}

func (fuenteDelAutomapa) Size() (w, h int) {
	if moorRegionActual == nil {
		return fuenteDelMotor{}.Size()
	}

	return moorNivelAncho147, moorNivelAlto147
}

func (f fuenteDelAutomapa) Tiles(tx, ty int) []d2automap.TileRef {
	if moorRegionActual == nil {
		return fuenteDelMotor{}.Tiles(tx, ty)
	}

	return fuenteDelMotor{}.Tiles(tx-moorRegionActual.X, ty-moorRegionActual.Y)
}
