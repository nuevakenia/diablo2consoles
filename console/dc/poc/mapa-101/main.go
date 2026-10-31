//go:build dreamcast

// DC-002.101 -- EL PUEBLO COMPUESTO Y DIBUJADO EN LA CONSOLA CON EL VIDEO ENCENDIDO.
//
// QUE MIDE. DC-002.94 dejo medido que el camino de dibujo del MOTOR compone el cuadro del pueblo (host:
// 307.196 de 307.200 pixeles, crc32 1437022778, 19 ms) y que en la consola ese camino NO entra por memoria: la
// cache de tiles RGBA del primer cuadro son 10,4 MB y el monton de C del POC, con el video arriba, no llega ni
// a cargar las tablas (moría en la cuarta).
//
// Esta tarjeta mide y aplica tres cosas:
//
//  1. LA CACHE EN INDICES (`CacheDeTilesEnIndices`, DC-002.101): el mismo cuadro con 1 B por pixel en vez de
//     4. Medido en el host con `console/dc/medidas/render-294 -perezoso -indices`: 86 superficies y
//     2.723.840 B (2,60 MB) en vez de 10.895.360 B (10,39 MB), con el MISMO crc32 (1437022778) y el mismo PNG,
//     byte a byte.
//  2. SOLTAR EL ARENA DE BLOQUES antes de dibujar (`d2asset.SoltarArenaDeBloques`, DC-002.101): los 2.621.440 B
//     de tramos + 1.008.003 B del buffer prestado vuelven al monton cuando la cache ya esta armada. Se mide
//     con `mallinfo` antes y despues, y se comprueba que el cuadro 2 (ya sin el arena) da el mismo crc32.
//  3. SOLO LAS TABLAS DEL COMPOSITOR: DC-002.88 midio que componer el pueblo necesita LvlTypes y LvlPrest
//     (+324.112 B). Cargar las 12 de la lista era lo que mataba al recolector en la cuarta, con 2.512 B libres
//     (DC-002.94).
//
// EL ARRANQUE ES EL DE DC-002.81/.88: video primero (el PVR necesita el monton vacio: DC-002.50),
// `d2asset.ArrancarComoConsola()`, el AssetManager con los dos MPQ, las tablas, el pueblo por `d2mapengine` y
// el cuadro por `d2maprenderer` sobre `pvr.Surface`.
package main

import (
	"hash/crc32"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2term"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_monton_usado
func d2cMontonUsado() int

//extern d2c_monton_libre
func d2cMontonLibre() int

//extern d2c_monton_arena
func d2cMontonArena() int

//extern d2c_sbrk_actual
func d2cSbrkActual() uint

//extern d2c_monton_techo
func d2cMontonTecho() uint

//extern d2c_libre
func d2cLibre(p uintptr)

func init() { d2cSerialInit() }

const (
	rutaParche = "/cd/JUEGO/PATCH_D2.MPQ"
	rutaDatos  = "/cd/JUEGO/D2DATA.MPQ"

	regionPueblo = d2enum.RegionAct1Town
	presetA      = 1
	archivoA     = 0

	// la pantalla de la consola y la celda del pueblo de las tarjetas anteriores (640x480 centrado en 28,20)
	ancho, alto = 640, 480
	celdaX      = 28.0
	celdaY      = 20.0

	// cuadros a presentar: el 1 compone y arma la cache, el 2 se dibuja DESPUES de soltar el arena
	cuadros = 3
)

// tablasDelCompositor son las 12 de la lista; SOLO SE CARGAN LAS PRIMERAS (`tablasQueSeCargan`), que son las
// que el compositor pide de verdad (DC-002.88: LvlTypes + LvlPrest = +324.112 B). Cargar las 12 es lo que
// mataba al recolector en la cuarta con el video encendido.
var tablasDelCompositor = []string{
	d2resource.LevelType,
	d2resource.LevelPreset,
	d2resource.LevelWarp,
	d2resource.AutoMap,
	d2resource.LevelDetails,
	d2resource.LevelMaze,
	d2resource.LevelSubstitutions,
	d2resource.ObjectType,
	d2resource.ObjectDetails,
	d2resource.MonPreset,
	d2resource.MonStats,
	d2resource.MonStats2,
}

const tablasQueSeCargan = 2

// sinIndices apaga la cache en indices (deja la RGBA de siempre): es la baseline del A/B en la consola.
var sinIndices = false

// el estado del cuadro
var (
	rendererDelMapa *d2maprenderer.MapRenderer
	cuadro          int
	crudPrimerCuadro uint32
)

func memoria(etapa string) {
	println("MAPA101: MEMORIA", etapa, "usado=", d2cMontonUsado(), "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena(), "sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()))
}

func main() {
	println("MAPA101: arranca DC-002.101 -- el pueblo EN PANTALLA con el video encendido")
	memoria("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("MAPA101: el video no arranco:", err.Error())

		return
	}

	println("MAPA101: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla())
	memoria("despues-del-video")

	// ============================== 1. EL MOTOR ==============================
	d2asset.ArrancarComoConsola()
	d2asset.UsarLiberadorDeExternos(func(p uintptr) { d2cLibre(p) })

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		println("MAPA101: NewAssetManager FALLO:", err.Error())

		return
	}

	for _, fuente := range []string{rutaParche, rutaDatos} {
		if err := am.AddSource(fuente, types.AssetSourceMPQ); err != nil {
			println("MAPA101: AddSource FALLO:", fuente, err.Error())

			return
		}
	}

	println("MAPA101: assetmanager listo")
	memoria("despues-del-assetmanager")

	// ============================== 2. LAS TABLAS DEL COMPOSITOR ==============================
	cargadas := 0

	for i := 0; i < tablasQueSeCargan && i < len(tablasDelCompositor); i++ {
		ruta := tablasDelCompositor[i]

		if err := am.LoadRecords(ruta); err != nil {
			println("MAPA101: TABLA FALLO:", ruta, err.Error())

			continue
		}

		cargadas++
	}

	println("MAPA101: tablas cargadas=", cargadas, "de", tablasQueSeCargan)
	memoria("despues-de-las-tablas")

	// ============================== 3. EL PUEBLO ==============================
	runtime.GC()

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	t0 := d2cMs()

	stamp := engine.LoadStamp(regionPueblo, presetA, archivoA)
	if stamp == nil {
		println("MAPA101: LoadStamp devolvio nil")

		return
	}

	tamano := stamp.Size()

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	pisos, paredes, crcGrilla := censo(engine)

	println("MAPA101: MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crcGrilla), "ms=", int(d2cMs()-t0))

	estado := d2asset.EstadoDeLosTiles()
	println("MAPA101: ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
		"tramos=", estado.BloquesTramos, "capacidad=", estado.BloquesCapacidad, "scratch=", estado.Scratch)
	memoria("despues-de-componer")

	// ============================== 4. EL RENDERER (CACHE EN INDICES) ==============================
	term, err := d2term.NewTerminal()
	if err != nil {
		println("MAPA101: NewTerminal FALLO:", err.Error())

		return
	}

	opciones := []d2maprenderer.Opcion{
		d2maprenderer.ComoConsola(ancho, alto),
		d2maprenderer.CacheDeTilesPerezosa(),
	}

	if !sinIndices {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnIndices())
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY, opciones...)

	espacio := rendererDelMapa.TamanoDelCuadro()
	println("MAPA101: renderer listo; cuadro=", espacio.W, espacio.H,
		"cachePerezosa=", rendererDelMapa.CachePerezosa(), "cacheEnIndices=", rendererDelMapa.CacheDeIndices())

	// ============================== 5. EL CUADRO EN PANTALLA ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) DC-002.101"); err != nil {
		println("MAPA101: el ciclo de cuadros salio con error:", err.Error())
	}

	println("MAPA101: FIN")
}

// dibujar: el cuadro 1 compone (y arma la cache perezosa), despues se SUELTA el arena de bloques, y el cuadro
// 2 se dibuja con la cache ya armada y el arena devuelto. Los crc32 de los dos tienen que ser el mismo.
func dibujar(superficie d2interface.Surface) error {
	if rendererDelMapa == nil {
		return nil
	}

	cuadro++

	if cuadro > cuadros {
		return nil
	}

	t0 := d2cMs()

	rendererDelMapa.Render(superficie)

	ms := int(d2cMs() - t0)

	malla, ok := superficie.(*pvr.Surface)
	if !ok {
		println("MAPA101: el Renderer no entrego una Surface del PVR")

		return nil
	}

	pixeles := malla.Imagen().Pixeles()
	llenos := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	crudo := crc32.ChecksumIEEE(pixeles)

	cache := rendererDelMapa.EstadoDeLaCache()
	println("MAPA101: CUADRO", cuadro, "pixelesNoVacios=", llenos, "de", ancho*alto, "crc32=", int(crudo),
		"ms=", ms, "cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes,
		"cacheEnIndices=", cache.EnIndices)
	memoria("despues-del-cuadro1")

	if cuadro == 1 {
		crudPrimerCuadro = crudo

		// el arena ya no hace falta: la cache del cuadro esta armada y el cuadro se pinta de ahi
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("MAPA101: ARENA SOLTADA tramos=", tramos, "bytes=", bytes)
		memoria("despues-de-soltar-el-arena")
	}

	return nil
}

func actualizar() error { return nil }

// censo: pisos, paredes y el crc32 de la grilla de identidades (el MISMO numero de DC-002.88: -1596781041).
func censo(engine *d2mapengine.MapEngine) (pisos, paredes int, crc uint32) {
	var buf [8]byte

	dig := crc32.NewIEEE()

	juego := *engine.Tiles()

	for i := range juego {
		t := &juego[i]

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
