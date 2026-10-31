//go:build dreamcast

// DC-002.82 -- FASE D: EL COMPOSITOR DEL MOTOR (`d2mapengine`) CARGA EL PUEBLO EN LA CONSOLA, Y PRIMERO
// LAS TABLAS DE RECORDS.
//
// QUE MIDE, EN ORDEN (las tres cosas de la tarjeta).
//
//  1. LAS TABLAS. `d2mapengine.GenerateMap` no arranca sin `asset.Records.Level.Presets`/`Level.Types`, o
//     sea sin las tablas de `d2resource.DataDictionaries()` que en la PC carga `d2app.initDataDictionaries`.
//     Aca se cargan UNA POR UNA por el camino del motor (`am.LoadRecords`) y se informa, por tabla, lo que
//     pide el monton de C antes y despues, cuanto tardo, y si entro. El pico del monton de C es el numero
//     que manda (el `runtime.ReadMemStats` de libgodc devuelve cero: DC-002.9).
//
//  2. EL PUEBLO POR EL MOTOR. Con las tablas que entren: `d2mapengine.CreateMapEngine` +
//     `LoadStamp(RegionAct1Town, preset, archivo)` + `ResetMap` + `PlaceStamp`, tres fases (entrar al
//     pueblo, cambiar a la transicion Este, volver). Se informa por fase: monton usado/arena/libre antes,
//     pico muestreado entre pasos, tiempo y el censo del mapa compuesto (casillas con piso, con pared,
//     entidades) mas un crc32 de la grilla de identidades (estilo, secuencia, tipo, prop1) que se compara
//     contra el MISMO censo hecho sobre el .ds1: si coinciden, el motor copio el pueblo entero.
//
//  3. LA PARED, si no entra. No hay camino intermedio inventado: si el motor muere, el numero del paso en
//     que murio y el monton de ese momento quedan impresos ANTES de morir, uno por linea.
//
// EL ARRANQUE ES EL DE DC-002.81: `d2asset.ArrancarComoConsola()` (una linea) y despues el AssetManager
// del motor (`NewAssetManager` + `AddSource`, como `d2app`), con DOS MPQ: `PATCH_D2.MPQ` (donde viven las
// tablas) y `D2DATA.MPQ` (donde viven los .ds1 y los .dt1). Sin el parche no hay LvlTypes/LvlPrest.
//
// QUE NO ENTRA: el cooperativo (DC-002.76), el personaje, los presupuestos de cache (.75), el semiespacio
// del GC (.78 seccion 4) y optimizar el tiempo de carga.
package main

import (
	"hash/crc32"
	"runtime"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_monton_arena
func d2cMontonArena() int

//extern d2c_monton_libre
func d2cMontonLibre() int

//extern d2c_monton_usado
func d2cMontonUsado() int

//extern d2c_sbrk_actual
func d2cSbrkActual() uint

//extern d2c_monton_techo
func d2cMontonTecho() uint

//extern d2c_gc_frena
func d2cGcFrena()

//extern d2c_gc_suelta
func d2cGcSuelta()

// init: la consola serie es la UNICA salida de texto en la Dreamcast.
func init() { d2cSerialInit() }

const (
	rutaParche = "/cd/JUEGO/PATCH_D2.MPQ"
	rutaDatos  = "/cd/JUEGO/D2DATA.MPQ"

	// el pueblo del acto 1: RegionAct1Town (d2enum) -> LevelType[1] "Act 1 - Town" (9 .dt1) y
	// Preset[1] "Act 1 - Town 1" (TownN1/TownE1/TownS1/TownW1.ds1). El archivo 0 es TownN1.ds1, el MISMO
	// .ds1 que midieron DC-002.70/.75/.77/.78/.81 (crc32 del lienzo 3507647456).
	regionPueblo = d2enum.RegionAct1Town
	presetA      = 1
	archivoA     = 0

	// la segunda zona: la misma region (mismo LevelType, o sea la misma lista de .dt1) con el preset de la
	// transicion Este. Es una zona DISTINTA del pueblo en el .ds1 y de tamano propio, que es lo que hay que
	// medir al "cambiar de zona" con el compositor del motor.
	presetB  = 2
	archivoB = 1

	// el lienzo de referencia del POC: la misma ventana que midieron las tarjetas anteriores (512x512
	// centrada en la celda 28,20 del pueblo), para que el crc32 sea comparable.
	ladoVentana = 512
	centroAx    = 28
	centroAy    = 20
)

// tablasDelCompositor: las tablas que el compositor toca de verdad. Se dice con el codigo en la mano:
// `LoadStamp` lee Presets y Types; `ResetMap` recorre `levelType.Files`; los mapas automaticos y los
// warps salen de AutoMap/LevelWarp. Para componer SOLO casillas (sin entidades) alcanzan las dos primeras.
var tablasDelCompositor = map[string]bool{
	d2resource.LevelType:          true,
	d2resource.LevelPreset:        true,
	d2resource.LevelWarp:          true,
	d2resource.AutoMap:            true,
	d2resource.LevelDetails:       true,
	d2resource.LevelMaze:          true,
	d2resource.LevelSubstitutions: true,
	d2resource.ObjectType:         true,
	d2resource.ObjectDetails:      true,
	d2resource.MonPreset:          true,
	d2resource.MonStats:           true,
	d2resource.MonStats2:          true,
}

// vigilante: el pico del monton de C dentro de una fase, muestreado en cada paso.
type vigilante struct {
	usadoPico int
	arenaPico int
	libreMin  int
	dondePico string
}

func nuevoVigilante() *vigilante {
	return &vigilante{libreMin: 1 << 30}
}

func (v *vigilante) muestra(donde string) {
	usado := d2cMontonUsado()
	arena := d2cMontonArena()
	libre := d2cMontonLibre()

	if usado > v.usadoPico {
		v.usadoPico = usado
		v.dondePico = donde
	}

	if arena > v.arenaPico {
		v.arenaPico = arena
	}

	if libre < v.libreMin {
		v.libreMin = libre
	}
}

func main() {
	// DC-002.88: SONDA DE MAPAS. El compositor murio DOS veces en `runtime.mapassign` (illegal instruction
	// en la SH-4): una con `map[[3]int32]bool` y otra con `map[[3]int32]int` (`MapEngine.dt1KeyCount`, que
	// escribe `extendRank` al cargar cada `.dt1`). Los caches de d2asset (`map[string]*cacheNode`) si
	// funcionan. Aca se prueban, en orden y con tamano realista, las tres formas de clave para saber cual
	// sirve en la consola antes de tocar el motor.
	sondaDeMapas()

	println("TABLAS: arranca DC-002.82 -- el compositor del MOTOR (d2mapengine) carga el pueblo en la consola")
	println("TABLAS: modoTablas=", modoTablas, "(0 ninguna, 1 las del nivel, 2 todas) hacerMapa=", hacerMapa,
		"conEntidades=", conEntidades, "lienzoReferencia=", lienzoReferencia, "tilesDelStamp=", tilesDelStamp)
	println("TABLAS: monton al arrancar: " + estado())
	println("TABLAS: sizeof int=", int(unsafe.Sizeof(int(0))), "puntero=", int(unsafe.Sizeof(uintptr(0))),
		"d2ds1.Tile=", int(unsafe.Sizeof(d2ds1.Tile{})), "d2dt1.Tile=", int(unsafe.Sizeof(d2dt1.Tile{})),
		"MapTile=", int(unsafe.Sizeof(d2mapengine.MapTile{})))

	// DC-002.81: el arranque de la consola en UNA linea.
	d2asset.ArrancarComoConsola()

	p := d2asset.PresupuestosVigentes()
	println("TABLAS: presupuestos: dt1s=", p.DT1s, "ds1s=", p.DS1s, "sumaDatos=", p.SumaDeLosCachesDeDatos())

	am, err := d2asset.NewAssetManager(d2util.LogLevelDefault)
	if err != nil {
		println("TABLAS: NewAssetManager FALLO:", err.Error())

		return
	}

	// el orden del motor: el parche primero (es donde estan las tablas), los datos despues (.ds1/.dt1).
	for _, fuente := range []string{rutaParche, rutaDatos} {
		if err := am.AddSource(fuente, types.AssetSourceMPQ); err != nil {
			println("TABLAS: AddSource FALLO:", fuente, err.Error())

			return
		}
	}

	arena := d2asset.EstadoDeLosTiles()
	println("TABLAS: assetmanager del motor listo; caminoDeTilesDeConsola=", arena.CaminoDeConsola,
		"monton="+estado())

	// ============================== 1. LAS TABLAS ==============================
	resumenTablas := cargarLasTablas(am)

	println("TABLAS: ================ TABLAS: VEREDICTO ================")
	println("TABLAS: intentadas=", resumenTablas.intentadas, "cargadas=", resumenTablas.cargadas,
		"fallidas=", resumenTablas.fallidas, "ms=", resumenTablas.ms)
	println("TABLAS: monton usado al empezar las tablas=", resumenTablas.usadoAntes,
		"al terminar=", resumenTablas.usadoDespues, "delta=", resumenTablas.usadoDespues-resumenTablas.usadoAntes)
	println("TABLAS: arena al terminar=", d2cMontonArena(), "libre=", d2cMontonLibre())

	if !hacerMapa {
		println("TABLAS: FIN (hacerMapa=false)")

		return
	}

	if resumenTablas.cargadas == 0 {
		println("TABLAS: FIN -- sin tablas no hay compositor que probar")

		return
	}

	// ============================== 2. EL PUEBLO POR EL MOTOR ==============================
	// pedirArenaPrimero: pide el arena de tiles ANTES de armar el MapEngine, con un .dt1 suelto. El arena
	// es UNA constante de d2asset (capacidadDeLaArenaDeTiles = 4.849.148 B, dimensionada en DC-002.78 para
	// los 8 .dt1 del pueblo del POC) y se pide entera la primera vez que se carga un .dt1. Si la pared es el
	// arena, la corrida muere ACA y la ultima linea del log dice con cuanto margen se pidio.
	if pedirArenaPrimero {
		println("ARENA: capacidad que pide d2asset = 4849148 B (constante de DC-002.78) reservada=",
			d2asset.CapacidadReservadaDelArena())
		println("ARENA: los 9 .dt1 del level type del pueblo miden 4652603 B crudos; margen del monton antes de pedirla =", margen())

		_, errArena := am.LoadDT1("act1/town/floor.dt1")
		if errArena != nil {
			println("ARENA: el .dt1 de prueba FALLO:", errArena.Error())
		} else {
			println("ARENA: arena pedida y un .dt1 cargado; monton=", estado())
		}
	}

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelDefault, am)
	println("MAPA: MapEngine creado; monton=", estado())

	a := componer(engine, "a) ENTRA AL PUEBLO", presetA, archivoA)
	b := componer(engine, "b) CAMBIA A LA TRANSICION ESTE", presetB, archivoB)
	c := componer(engine, "c) VUELVE AL PUEBLO", presetA, archivoA)

	println("")
	println("MAPA: ================ MAPA: VEREDICTO ================")

	for _, fase := range []*faseMapa{a, b, c} {
		if fase == nil {
			println("MAPA: fase PERDIDA (ver el FALLO de arriba)")

			continue
		}

		println("MAPA:", fase.rotulo, "w=", fase.ancho, "h=", fase.alto, "tiles=", fase.tiles,
			"dt1s=", fase.dt1s, "tilesDT1=", fase.tilesDT1, "pisos=", fase.pisos, "paredes=", fase.paredes,
			"entidades=", fase.entidades)
		println("MAPA:", fase.rotulo, "crcGrilla=", int(fase.crc), "ms=", fase.ms,
			"picoUsado=", fase.usadoPico, "(en: "+fase.dondePico+")", "arenaPico=", fase.arenaPico,
			"libreMin=", fase.libreMin)
		println("MAPA:", fase.rotulo, "monton al terminar=", fase.usadoAlFinal, "arena=", d2cMontonArena(),
			"libre=", d2cMontonLibre())
	}

	if a != nil && c != nil {
		println("MAPA: pico(c) - pico(a) =", c.usadoPico-a.usadoPico, "bytes")
		println("MAPA: arena(c) - arena(a) =", c.arenaPico-a.arenaPico, "bytes")

		if a.crc == c.crc && a.pisos == c.pisos && a.paredes == c.paredes {
			println("MAPA: GRILLA OK -- el pueblo de la vuelta es identico al de la entrada (crc", int(a.crc), ")")
		} else {
			println("MAPA: GRILLA CAMBIO -- entrada crc=", int(a.crc), "pisos=", a.pisos,
				"paredes=", a.paredes, "; vuelta crc=", int(c.crc), "pisos=", c.pisos, "paredes=", c.paredes)
		}
	}

	// el instrumento del arena: cuantas veces hubo que tirarlo (cada reinicio vacia el cache de dt1s y,
	// con el compositor del motor, deja apuntando al arena viejo a los tiles que el MapEngine ya copio).
	estadoArena := d2asset.EstadoDeLosTiles()
	println("MAPA: ARENA de tiles: caminoDeConsola=", estadoArena.CaminoDeConsola,
		"capacidad=", estadoArena.Capacidad, "pedida=", estadoArena.CapacidadPedida, "usado=", estadoArena.Usado,
		"reinicios=", estadoArena.Reinicios, "archivos=", estadoArena.ArchivosLeidos)
	println("MAPA: BLOQUES de tiles: usados=", estadoArena.BloquesUsados, "bytes=", estadoArena.BloquesBytes,
		"tramos=", estadoArena.BloquesTramos, "capacidad=", estadoArena.BloquesCapacidad,
		"scratch=", estadoArena.Scratch)

	if prestamo, hay := d2asset.EstadoDelPrestamoDeTiles(); hay {
		println("MAPA: PRESTAMO de la lectura: lecturas=", prestamo.Lecturas, "sectores=", prestamo.Sectores,
			"reservado=", prestamo.Reservado, "crecidas=", prestamo.Crecidas)
	}

	// ============================== 3. EL LIENZO DE REFERENCIA ==============================
	// El compositor del POC (`nivel.go`) sobre el MISMO .ds1 del pueblo: si su crc32 da 3507647456, el
	// binario esta mirando el mismo pueblo que las tarjetas anteriores y la comparacion tiene sentido.
	// Se corre AL FINAL a proposito: el lienzo (262.144 B) y el scratch (81.920 B) pasan de 64 KB y en esta
	// consola quedan tomados, asi que no pueden contaminar el pico de las fases del motor.
	if lienzoReferencia {
		lienzoDeReferencia(am)
	}

	println("TABLAS: monton al final: " + estado())
	println("TABLAS: FIN")
}

// resumenDeTablas: lo que dejo el paso 1.
type resumenDeTablas struct {
	intentadas   int
	cargadas     int
	fallidas     int
	ms           int
	usadoAntes   int
	usadoDespues int
}

// cargarLasTablas: TODAS las tablas (o las del nivel, segun la palanca), UNA POR UNA, con el monton de C
// medido alrededor de cada una. La linea "TABLA ..." se imprime ANTES de cargar: si esa tabla se lleva el
// monton puesto, el log dice cual fue y con cuanto iba.
func cargarLasTablas(am *d2asset.AssetManager) resumenDeTablas {
	todas := d2resource.DataDictionaries()

	tablas := make([]string, 0, len(todas))

	for _, ruta := range todas {
		switch modoTablas {
		case 0:
			// ninguna
		case 1:
			if tablasDelCompositor[ruta] {
				tablas = append(tablas, ruta)
			}
		case 3:
			// SOLO las dos imprescindibles del compositor de casillas: LvlTypes (LoadStamp/ResetMap) y
			// LvlPrest (LoadStamp). Con estas dos y sin entidades, `d2mapengine` compone el mapa.
			if ruta == d2resource.LevelType || ruta == d2resource.LevelPreset {
				tablas = append(tablas, ruta)
			}
		default:
			tablas = append(tablas, ruta)
		}
	}

	r := resumenDeTablas{intentadas: len(tablas), usadoAntes: d2cMontonUsado()}
	t0 := d2cMs()

	for i := 0; i < len(tablas); i++ {
		// queda en el registro de DC-002.39: con gccgo en esta consola, la CADENA de una vuelta de `range`
		// se lee ANTES del recolctor y puede quedar con el largo de otro lado despues del GC. Por eso la
		// ruta se RELEE del arreglo DESPUES del GC, en cada vuelta.
		antesGC := d2cMontonUsado()

		if gcEntreTablas {
			runtime.GC()
			println("TABLA", i+1, "GC devolvio=", antesGC-d2cMontonUsado(), "monton="+estado())
		}

		ruta := tablas[i]

		// gcInhibidoTablas: el recolector FRENADO durante la carga de la tabla, el mismo freno que el
		// runtime de libgodc se pone en sus secciones criticas (`d2c_gc_frena`, ver csrc/shim.c). Medido en
		// DC-002.78 para los .dt1: el recolector de libgodc es de COPIA y salta por umbral EN MEDIO del
		// parseo; con punteros vivos en el lector, el SH-4 se va a una direccion que el recolector movio.
		if gcInhibidoTablas {
			d2cGcFrena()
		}

		println("TABLA", i+1, "de", len(tablas), ruta, "<-- cargando", "usado=", d2cMontonUsado(),
			"arena=", d2cMontonArena(), "libre=", d2cMontonLibre(), "margen=", margen())

		antes := d2cMontonUsado()
		tIni := d2cMs()

		err := am.LoadRecords(ruta)

		if gcInhibidoTablas {
			d2cGcSuelta()
		}

		ms := int(d2cMs() - tIni)
		despues := d2cMontonUsado()

		if err != nil {
			r.fallidas++
			println("TABLA", i+1, ruta, "FALLO:", err.Error(), "ms=", ms)

			continue
		}

		r.cargadas++

		// las filas que quedaron en los Records: es la prueba de que la tabla se consumo, no solo de que
		// el archivo se leyo (con `delta=0` el monton no dice nada).
		println("TABLA", i+1, ruta, "OK usado=", despues, "delta=", despues-antes, "ms=", ms,
			"| types=", len(am.Records.Level.Types), "presets=", len(am.Records.Level.Presets),
			"objects=", len(am.Records.Object.Details), "monstats=", len(am.Records.Monster.Stats))
	}

	r.ms = int(d2cMs() - t0)
	r.usadoDespues = d2cMontonUsado()

	return r
}

// faseMapa: lo que dejo UNA fase del compositor del motor.
type faseMapa struct {
	rotulo string

	ancho, alto int
	tiles       int
	dt1s        int
	tilesDT1    int
	pisos       int
	paredes     int
	entidades   int
	crc         uint32

	ms             int
	usadoPico      int
	arenaPico      int
	libreMin       int
	dondePico      string
	usadoAlFinal   int
	usadoAlEmpezar int
}

// componer: una fase entera del compositor del motor. Cada paso se mide y se imprime; si un paso paniquea
// (el motor paniquea en LoadStamp cuando el preset no tiene archivos, y en PlaceStamp si el stamp no entra
// en el mapa) el panic se ATRAPA aca y se informa como lo que es: una excepcion medida, no una corrida
// perdida.
func componer(engine *d2mapengine.MapEngine, rotulo string, preset, archivo int) (fase *faseMapa) {
	println("")
	println("MAPA: ===== " + rotulo + " =====")

	runtime.GC()

	f := &faseMapa{rotulo: rotulo, usadoAlEmpezar: d2cMontonUsado()}

	println("MAPA:", rotulo, "monton antes (tras GC):", f.usadoAlEmpezar, "arena=", d2cMontonArena(),
		"libre=", d2cMontonLibre())

	v := nuevoVigilante()
	v.muestra("arranque de la fase")

	t0 := d2cMs()

	var stampErr error

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				stampErr = errorDePanic(rec)
			}
		}()

		stamp := engine.LoadStamp(regionPueblo, preset, archivo)
		if stamp == nil {
			stampErr = errorDeTexto("LoadStamp devolvio nil")

			return
		}

		v.muestra("LoadStamp (los .dt1 del level type y el .ds1)")

		tamano := stamp.Size()

		f.ancho, f.alto = tamano.Width, tamano.Height
		println("MAPA:", rotulo, "LoadStamp OK: ds1 w=", f.ancho, "h=", f.alto,
			"usado=", d2cMontonUsado(), "arena=", d2cMontonArena())

		// DC-002.88: la lista de `.dt1` del `.ds1` del stamp, que es la que el nivel carga de verdad.
		rutas := stamp.TilesDelDS1()
		println("MAPA:", rotulo, "los .dt1 del .ds1=", len(rutas), "en el level type=", cuentaTilesDelLevelType(engine))

		if tilesDelStamp {
			// el arena se reserva ANTES de la primera carga de un .dt1: es el tamano con el que se va a
			// pedir, y el `.dt1` que no entre tira el arena y con el el cache (el estado lo dice).
			engine.ResetMapParaStamp(regionPueblo, f.ancho, f.alto, stamp)
		} else {
			engine.ResetMap(regionPueblo, f.ancho, f.alto)
		}

		v.muestra("ResetMap")

		juego := engine.Tiles()
		f.tiles = len(*juego)

		// los .dt1 que `ResetMap` carga para este tipo de nivel (la lista de 32 casillas de LvlTypes)
		tipos := engine.LevelType()
		for i := range tipos.Files {
			if tipos.Files[i] != "" && tipos.Files[i] != "0" {
				f.dt1s++
			}
		}

		println("MAPA:", rotulo, "ResetMap OK: tiles=", f.tiles, "archivosDT1=", f.dt1s,
			"usado=", d2cMontonUsado())

		estadoArena := d2asset.EstadoDeLosTiles()
		println("MAPA:", rotulo, "ARENA capacidad=", estadoArena.Capacidad, "pedida=",
			estadoArena.CapacidadPedida, "usado=", estadoArena.Usado, "reinicios=", estadoArena.Reinicios,
			"archivos=", estadoArena.ArchivosLeidos)
		println("MAPA:", rotulo, "BLOQUES usados=", estadoArena.BloquesUsados, "bytes=",
			estadoArena.BloquesBytes, "tramos=", estadoArena.BloquesTramos, "capacidad=",
			estadoArena.BloquesCapacidad, "scratch=", estadoArena.Scratch)

		if conEntidades {
			engine.PlaceStamp(stamp, 0, 0)
		} else {
			// sin entidades: las casillas SOLAS. `keep` en false y `monsters` en false es el camino del
			// motor (`PlaceStampClippedWhere`) sin pedirle monstruos ni objetos, que necesitan la tabla de
			// lookups en disco y la animacion (d2animdata) que este binario no carga.
			engine.PlaceStampClippedWhere(stamp, 0, 0, f.ancho, f.alto, func(int) bool { return false }, false)
		}

		v.muestra("PlaceStamp")
	}()

	f.ms = int(d2cMs() - t0)

	if stampErr != nil {
		println("MAPA:", rotulo, "EXCEPCION:", stampErr.Error(), "ms=", f.ms, "usado=", d2cMontonUsado(),
			"arena=", d2cMontonArena(), "libre=", d2cMontonLibre())
		println("MAPA:", rotulo, "picoUsado=", v.usadoPico, "(en: "+v.dondePico+")", "arenaPico=", v.arenaPico,
			"libreMin=", v.libreMin)

		return nil
	}

	f.pisos, f.paredes, f.crc = censo(engine.Tiles())
	f.entidades = len(engine.Entities())
	f.usadoPico = v.usadoPico
	f.arenaPico = v.arenaPico
	f.libreMin = v.libreMin
	f.dondePico = v.dondePico
	f.usadoAlFinal = d2cMontonUsado()

	println("MAPA:", rotulo, "CASILLAS pisos=", f.pisos, "paredes=", f.paredes,
		"crcGrilla=", int(f.crc), "entidades=", f.entidades)
	println("MAPA:", rotulo, "PICO usado=", f.usadoPico, "(en: "+f.dondePico+")", "arena=", f.arenaPico,
		"libreMin=", f.libreMin, "ms=", f.ms)

	return f
}

// cuentaTilesDelLevelType: cuantos archivos de tile declara el level type del motor (los 32 campos, sin
// vacios). Es el otro lado del numero de DC-002.88: el level type dice N y el `.ds1` usa M.
func cuentaTilesDelLevelType(engine *d2mapengine.MapEngine) int {
	tipos := engine.LevelType()

	n := 0

	for i := range tipos.Files {
		if tipos.Files[i] != "" && tipos.Files[i] != "0" {
			n++
		}
	}

	return n
}

// censo: casillas con piso y con pared, y un crc32 de la GRILLA de identidades (estilo, secuencia, tipo,
// prop1 de la primera capa de piso y de la primera de pared). Es el numero que se compara contra el mismo
// censo hecho sobre el .ds1 -- no es un crc de pixeles (eso es el lienzo, y lo compone el POC).
func censo(juego *[]d2mapengine.MapTile) (pisos, paredes int, crc uint32) {
	var buf [8]byte

	// la consola tiene un puente de hash/crc32 con `NewIEEE` (no exporta `Update`/`IEEETable`), asi que el
	// crc se acumula escribiendo casilla por casilla en el mismo digest.
	dig := crc32.NewIEEE()

	for i := range *juego {
		t := &(*juego)[i]

		for j := range buf {
			buf[j] = 0
		}

		if len(t.Components.Floors) > 0 {
			fl := &t.Components.Floors[0]
			buf[0], buf[1], buf[2], buf[3] = fl.Style, fl.Sequence, byte(fl.Type), fl.Prop1

			if !fl.Hidden() && fl.Prop1 != 0 {
				pisos++
			}
		}

		if len(t.Components.Walls) > 0 {
			w := &t.Components.Walls[0]
			buf[4], buf[5], buf[6], buf[7] = w.Style, w.Sequence, byte(w.Type), w.Prop1

			if !w.Hidden() && w.Prop1 != 0 {
				paredes++
			}
		}

		_, _ = dig.Write(buf[:])
	}

	return pisos, paredes, dig.Sum32()
}

// lienzoDeReferencia: el compositor del POC (nivel.go) sobre el MISMO .ds1 del pueblo, para dejar en la
// misma corrida el crc32 que midieron DC-002.70/.75/.77/.78/.81 (3507647456) y la prueba de que el .ds1
// que carga el motor es el mismo pueblo.
func lienzoDeReferencia(am *d2asset.AssetManager) {
	println("")
	println("LIENZO: ===== el compositor del POC (nivel.go) sobre el mismo .ds1 =====")

	d, err := am.LoadDS1("act1/town/townn1.ds1")
	if err != nil {
		println("LIENZO: LoadDS1 FALLO:", err.Error())

		return
	}

	println("LIENZO: ds1 w=", d.Width(), "h=", d.Height(), "archivos=", len(d.Files), "usado=", d2cMontonUsado())

	// el mismo censo que el del mapa del motor: es la comparacion casilla por casilla
	pisosDS1, paredesDS1, crcDS1 := censoDelDS1(d)
	println("LIENZO: GRILLA DEL DS1 pisos=", pisosDS1, "paredes=", paredesDS1, "crcGrilla=", int(crcDS1))

	n := NuevoNivelEn(d, ladoVentana, ladoVentana, centroAx, centroAy, nil)
	println("LIENZO: ventana 512x512 origen=", n.OrigenX, n.OrigenY, "pisos=", n.PisosPedidos,
		"paredes=", n.ParedesPedidas, "usado=", d2cMontonUsado())

	for _, nombreDelDS1 := range d.Files {
		ruta := RutaDelDT1(nombreDelDS1)

		dt1, err := am.LoadDT1(ClaveRelativa(ruta))
		if err != nil {
			println("LIENZO: DT1 FALLO:", ruta, err.Error())

			continue
		}

		n.Alimentar(dt1.Tiles)
	}

	n.Cerrar()

	println("LIENZO: compuesto pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile,
		"pixelesNoVacios=", n.PixelesNoVacios(), "crc32=", int(n.CRC32()),
		"usado=", d2cMontonUsado())
}

// ClaveRelativa: lo que pide `am.LoadDT1` -- la ruta del .dt1 SIN el prefijo "/data/global/tiles/",
// que el propio LoadDT1 antepone. `RutaDelDT1` (nivel.go) devuelve la ruta con el prefijo.
func ClaveRelativa(ruta string) string {
	return strings.TrimPrefix(ruta, "data/global/tiles/")
}

// censoDelDS1: el MISMO censo y el mismo crc que `censo`, pero leido de las capas del .ds1 tal cual
// vienen. Si los dos numeros coinciden, el `PlaceStamp` del motor copio el pueblo entero casilla por
// casilla (el mapa del motor es el .ds1, no una parte).
func censoDelDS1(d *d2ds1.DS1) (pisos, paredes int, crc uint32) {
	piso := d.GetFloor(0)
	pared := d.GetWall(0)

	var buf [8]byte

	dig := crc32.NewIEEE()

	for y := 0; y < d.Height(); y++ {
		for x := 0; x < d.Width(); x++ {
			for j := range buf {
				buf[j] = 0
			}

			if piso != nil {
				if t := piso.Tile(x, y); t != nil {
					if !t.Hidden() && t.Prop1 != 0 {
						pisos++
						buf[0], buf[1], buf[2], buf[3] = t.Style, t.Sequence, byte(t.Type), t.Prop1
					}
				}
			}

			if pared != nil {
				if t := pared.Tile(x, y); t != nil {
					if !t.Hidden() && t.Prop1 != 0 {
						paredes++
						buf[4], buf[5], buf[6], buf[7] = t.Style, t.Sequence, byte(t.Type), t.Prop1
					}
				}
			}

			_, _ = dig.Write(buf[:])
		}
	}

	return pisos, paredes, dig.Sum32()
}

// errorDeTexto: un error con nombre, sin depender de fmt/errors en la consola.
type errorDeTexto string

func (e errorDeTexto) Error() string { return string(e) }

func errorDePanic(rec interface{}) error {
	if err, ok := rec.(error); ok {
		return errorDeTexto("panic: " + err.Error())
	}

	if texto, ok := rec.(string); ok {
		return errorDeTexto("panic: " + texto)
	}

	return errorDeTexto("panic (sin texto)")
}

// estado: los cuatro numeros del monton de C, juntos, mas el margen que le queda al `sbrk` antes del techo
// de KOS (ese margen es el que de verdad dice si algo entra: `libre` es lo libre DENTRO del arena, y el
// arena puede crecer por `sbrk` mientras haya techo).
func estado() string {
	return "arena=" + itoa(d2cMontonArena()) + " usado=" + itoa(d2cMontonUsado()) +
		" libre=" + itoa(d2cMontonLibre()) + " sbrk=" + hex32(d2cSbrkActual()) +
		" techo=" + hex32(d2cMontonTecho()) + " margen=" + itoa(margen())
}

// margen: lo que le queda al monton antes del techo de KOS.
func margen() int {
	return int(d2cMontonTecho() - d2cSbrkActual())
}

// ============================== utilidades (sin strconv ni fmt en la consola) ==============================

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	negativo := v < 0
	if negativo {
		v = -v
	}

	var buf [20]byte

	i := len(buf)

	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}

	if negativo {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

func hex32(v uint) string {
	const cifras = "0123456789abcdef"

	var buf [10]byte

	buf[0] = '0'
	buf[1] = 'x'

	for i := 0; i < 8; i++ {
		buf[2+i] = cifras[(v>>uint(28-4*i))&0xf]
	}

	return string(buf[:])
}

// sondaDeMapas (DC-002.88): que forma de mapa sobrevive en la SH-4. Se prueba la clave `int64` (la que
// propone el arreglo), la clave `[3]int32` (la del motor hoy) y una clave `string`, con 700 entradas cada una
// (mas que las 684 claves del pueblo) y decir cual revienta y en cual.
func sondaDeMapas() {
	println("SONDA: int64 como clave")

	deEnteros := map[int64]int{}
	for i := int64(0); i < 700; i++ {
		deEnteros[i*7+3] = int(i)
	}

	println("SONDA: int64 OK len=", len(deEnteros), "valor=", deEnteros[3])

	println("SONDA: string como clave")

	deTextos := map[string]int{}
	for i := 0; i < 700; i++ {
		deTextos["tile-"+string(rune(48+i%10))+string(rune(65+i/10%26))] = i
	}

	println("SONDA: string OK len=", len(deTextos))

	// la clave `[3]int32` se saco de la sonda: revienta la SH-4 en `runtime.mapassign` (medido, PC
	// 8c2343d2), que es justo lo que motivo pasar el motor a clave `int64`.
}
