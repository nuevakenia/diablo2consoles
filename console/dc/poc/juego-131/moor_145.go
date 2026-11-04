//go:build dreamcast

package main

// DC-002.145 -- EL BLOOD MOOR DE VERDAD EN LA CONSOLA, COMPUESTO POR REGIONES CON EL DRLG.
//
// QUE ENTRA HOY Y QUE ENTRA CON ESTO. DC-002.144b hizo entrar en la consola el nivel de afuera como UNA PIEZA de
// `.ds1` (`Act1/Outdoors/Wild1.ds1`, 17 x 17, el preset 32 "Act 1 - Fence Fill 1", `levelID= 0`): 289 casillas
// que no son el Blood Moor. El Blood Moor de verdad es el nivel 2 de `Levels.txt`: 56 x 96 = 5.376 casillas, y
// NO TIENE FILA DE PRESET -- en el juego lo arma el DRLG por regiones, no un `.ds1` entero.
//
// POR QUE POR REGIONES Y NO ENTERO. Medido en el host (`console/dc/medidas/moor-drlg`, el DRLG del acto 1
// corriendo de verdad sobre los MPQ), el nivel entero pide:
//
//	arena de bloques  3.047.626 B   (los bloques RLE de los tiles que el nivel referencia, filtrados por clave)
//	buffer de lectura 1.141.544 B   (`act1/outdoors/cottages.dt1`, el `.dt1` mas grande del nivel)
//	                 -----------
//	                 4.189.170 B     contra los 2.994.912 B libres del cruce -> NO ENTRA (faltan ~1,2 MB)
//
// y una REGION de 16 x 16 pide:
//
//	arena de bloques    786.313 B   (region 0,48: 4 `.dt1`, 35 claves)
//	buffer de lectura 1.008.003 B   (`act1/town/floor.dt1`)
//	                 -----------
//	                 1.794.316 B     contra 2.994.912 libres -> SOBRA 1,2 MB
//
// EL REPARTO DEL TRABAJO (y por que esta asi). El DRLG es un port grande (25 paquetes de tablas, patrones de
// `.ds1`, `BuildTiles`) y la SH-4 tiene 16 MB: correrlo adentro pediria mas RAM que el nivel. Asi que el plan de
// la region se calcula en el HOST con el DRLG de verdad y se trae en el binario como DATOS
// (`moor_region_145.go`, generado por `console/dc/medidas/moor-drlg -plan`), y la CONSOLA compone: pide el
// arena de bloques de su region, deja el mapa limpio (`ResetMapParaTilesExactos`) y escribe cada casilla con
// `SetExactTiles`, que es el mismo camino que el motor de la PC usa para los niveles generados. Lo que la
// consola NO hace es generar: el plan es fijo (la semilla esta en el plan y en el log).
//
// LO QUE ESTA PALANCA NO ES. No es el nivel entero (5.376 casillas) ni el streaming de las regiones vecinas: es
// la primera region del Blood Moor de verdad en la consola, con su numero antes/despues. El streaming sale de
// aca: `demoComponeLaRegionDelMoor` ya toma la region como dato (`moorPlanoRegionX/Y` y `moorPlanoLado`), asi que
// pedir la siguiente es generar otro plan.

import (
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// los tres buffers que se reusan por casilla: pedir tres rebanadas nuevas por casilla son 786 `make` en una
// region de 16 x 16 y el semiespacio del recolector no esta para eso (es el numero que DC-002.120 midio con las
// casillas del pueblo: ~650.000 B para 2.337 casillas).
var (
	moorPisos   []d2mapengine.ExactTile
	moorParedes []d2mapengine.ExactTile
	moorSombras []d2mapengine.ExactTile
)

// demoComponeLaRegionDelMoor: el nivel de afuera del cruce, compuesto con el plan del DRLG. Devuelve si el nivel
// nuevo quedo colgado del renderer. El pueblo ya lo solto `demoCruzaAlMoor` (cache de tiles, arena y casillas),
// asi que aca solo se pide lo del nivel nuevo.
func demoComponeLaRegionDelMoor(t0 int64) bool {
	if motorDelMapa == nil || rendererDelMapa == nil || gestorDeAssets == nil {
		println("DEMO140: REGION SIN MOTOR (motor/renderer/gestor nulos): no se compone")

		return false
	}

	lado := moorPlanoLado
	libre := d2cMontonLibre()

	println("DEMO140: REGION PLAN delDRLG nivel=", moorPlanoNivel, "semilla=", moorPlanoSemilla,
		"region=", lado, "x", lado, "desde=", moorPlanoRegionX, moorPlanoRegionY,
		"delNivel=", moorPlanoNivelAncho, "x", moorPlanoNivelAlto,
		"casillas=", len(moorPlanoCeldas), "registros=", moorPlanoRegistros,
		"piezas=", len(moorPlanoPiezas), "claves=", len(moorPlanoClaves))

	// ---- EL ORDEN DE LOS DOS PEDIDOS GRANDES (DC-002.144b, medido y con A/B) ----
	// El buffer prestado del `.dt1` mas grande se pide ANTES de los tramos del arena cuando la palanca esta en
	// "si": cae en el hueco fresco que dejo el arena del pueblo y los tramos crecen detras. Con "no" queda el
	// camino de DC-002.111 (un buffer por archivo, del tamano de ESE archivo).
	if moorScratchPrimero == "si" {
		d2asset.UsarScratchPorArchivo(false)
	} else {
		d2asset.UsarScratchPorArchivo(true)
	}

	println("DEMO140: REGION BUFFER primero=", moorScratchPrimero == "si", "delDT1MasGrande=", moorPlanoBuffer,
		"libre=", libre, "servible=", int(d2cServible(uint(moorPlanoBuffer))), "vramLibre=", pvr.VramDisponible())

	// el mapa del nivel nuevo, sin la lista de `.dt1` del level type: el arena se dimensiona con las claves de
	// la region y los `.dt1` los carga el motor cuando el primer tile de cada uno lo pide.
	d2mapengine.ResetMapParaTilesExactos(motorDelMapa, d2enum.RegionAct1Wilderness, lado, lado, moorPlanoClaves,
		moorPlanoBuffer)

	// ---- LAS CASILLAS, UNA POR UNA ----
	conPiso := 0
	fallas := 0

	for i := range moorPlanoCeldas {
		c := &moorPlanoCeldas[i]

		moorPisos = moorPisos[:0]
		moorParedes = moorParedes[:0]
		moorSombras = moorSombras[:0]

		moorPisos = moorRegistros(c.Pisos, moorPisos)
		moorParedes = moorRegistros(c.Paredes, moorParedes)
		moorSombras = moorRegistros(c.Sombras, moorSombras)

		motorDelMapa.SetExactTiles(c.X, c.Y, d2enum.RegionAct1Wilderness, false, moorPisos, moorParedes, moorSombras)

		if len(moorPisos) > 0 {
			conPiso++
		}

		if motorDelMapa.Tile(c.X, c.Y) == nil {
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

	println("DEMO140: REGION COMPUESTA casillas=", len(moorPlanoCeldas), "conPiso=", conPiso,
		"sinNada=", lado*lado-conPiso, "fallas=", fallas, "tiles=", len(*motorDelMapa.Tiles()),
		"ms=", int64(d2cMs())-t0, "libre=", d2cMontonLibre(), "semiespacio=", int(d2cGcUsado()),
		"vramLibre=", pvr.VramDisponible())

	// el jugador arranca donde el plan dice (una casilla con piso y sin paredes, cerca del centro), no en el
	// centro geometrico: en el afuera el centro puede caer en un lago o en un borde.
	demoAcomodaElCruce("BloodMoor:"+moorNombreDeLaRegion(), float64(moorPlanoSpawnX)+0.5,
		float64(moorPlanoSpawnY)+0.5, t0)

	runtime.GC()

	return true
}

// moorRegistros: pasa los registros del plan (pieza, indice, flags) a los `ExactTile` del motor, escribiendo en
// el buffer que se le pasa (que se reusa por casilla).
func moorRegistros(lista []moorTile, dst []d2mapengine.ExactTile) []d2mapengine.ExactTile {
	for i := range lista {
		dst = append(dst, d2mapengine.ExactTile{
			File:  moorPlanoPiezas[lista[i].Pieza],
			Index: lista[i].Indice,
			Flags: lista[i].Flags,
		})
	}

	return dst
}

// moorNombreDeLaRegion: el nombre del nivel y la region, para el log y para el cierre de la demo. El numero de
// casillas se dice aparte (el nivel entero son 5.376 y esta region es una parte declarada).
func moorNombreDeLaRegion() string {
	if moorPlanoLado == moorPlanoNivelAncho && moorPlanoNivelAlto == moorPlanoLado &&
		moorPlanoRegionX == 0 && moorPlanoRegionY == 0 {
		return "entero"
	}

	return "region"
}
