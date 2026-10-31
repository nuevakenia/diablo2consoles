//go:build dreamcast

// DC-002.94 -- EL PUEBLO COMPUESTO POR EL MOTOR, DIBUJADO POR `d2maprenderer` SOBRE EL PVR.
//
// QUE MIDE. Hasta DC-002.88 el pueblo se componia entero en la Dreamcast (2.337 pisos, 338 paredes, el
// crc32 de la grilla igual al del .ds1) pero no salia en pantalla. Esta tarjeta pone el camino de dibujo del
// MOTOR detras de esa grilla: el mismo `MapRenderer` que usa la PC, escribiendo en la `Surface` del backend
// PVR (`d2core/d2render/pvr`), con la camara en la celda del pueblo que midieron las tarjetas anteriores.
//
// LA MEMORIA ES LO QUE MANDA, y por eso hay dos palancas y se miden aparte:
//
//   - `ComoConsola(640,480)`: el cuadro es el de la consola. Sin esto `d2maprenderer` compone en el tamano de
//     `d2display`, que en esta plataforma no baja de 800x600.
//   - `CacheDeTilesPerezosa()`: la cache de tiles se arma al dibujar. La de entrada arma UNA superficie por
//     grafico distinto del nivel entero: medido en el host con `console/dc/medidas/render-294`, en el pueblo
//     son 277 superficies y 29,36 MB de RGBA. Con la palanca, el primer cuadro pide 86 (10,4 MB) y el
//     segundo no pide ninguna.
//
// LO QUE EL HOST YA MIDIO (mismo codigo, misma camara, `render-294`): el cuadro es de 307.196 pixeles no
// vacios de 307.200, llega crc32 1437022778 y los cuadros 2 y 3 dan el MISMO crc32 que el 1 (la cache
// perezosa dibuja exactamente el mismo cuadro). Ese es el numero contra el que se compara la consola.
//
// EL ARRANQUE ES EL DE DC-002.81/.88: video primero (el PVR necesita el monton vacio: DC-002.50),
// `d2asset.ArrancarComoConsola()`, el AssetManager del motor con los DOS MPQ y las tablas del compositor.
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

func init() { d2cSerialInit() }

const (
	rutaParche = "/cd/JUEGO/PATCH_D2.MPQ"
	rutaDatos  = "/cd/JUEGO/D2DATA.MPQ"

	regionPueblo = d2enum.RegionAct1Town
	presetA      = 1
	archivoA     = 0

	// la pantalla de la consola y la celda del pueblo que miraban las tarjetas anteriores (512x512 centrado
	// en la celda 28,20)
	ancho, alto = 640, 480
	celdaX      = 28.0
	celdaY      = 20.0

	// cuadros a presentar antes de terminar (el 1 dibuja y mide; los demas miden la cache ya armada)
	cuadros = 3
)

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

// el estado del cuadro: lo que el callback de dibujado usa cuadro a cuadro
var (
	rendererDelMapa *d2maprenderer.MapRenderer
	cuadro          int
)

func main() {
	println("MAPA294: arranca DC-002.94 -- el pueblo del MOTOR dibujado por d2maprenderer sobre el PVR")
	println("MAPA294: monton de C al arrancar: usado=", d2cMontonUsado(), "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena())

	// ============================== 0. EL VIDEO PRIMERO ==============================
	// el arranque del PVR necesita el monton vacio (DC-002.50), asi que va antes de cargar nada
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("MAPA294: el video no arranco:", err.Error())

		return
	}

	println("MAPA294: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla(),
		"monton libre=", d2cMontonLibre())

	// ============================== 1. EL MOTOR ==============================
	d2asset.ArrancarComoConsola()

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		println("MAPA294: NewAssetManager FALLO:", err.Error())

		return
	}

	for _, fuente := range []string{rutaParche, rutaDatos} {
		if err := am.AddSource(fuente, types.AssetSourceMPQ); err != nil {
			println("MAPA294: AddSource FALLO:", fuente, err.Error())

			return
		}
	}

	println("MAPA294: assetmanager listo; monton usado=", d2cMontonUsado(), "libre=", d2cMontonLibre())

	// ============================== 2. LAS TABLAS ==============================
	cargadas := 0

	for i := range tablasDelCompositor {
		ruta := tablasDelCompositor[i]

		if err := am.LoadRecords(ruta); err != nil {
			println("MAPA294: TABLA FALLO:", ruta, err.Error())

			continue
		}

		cargadas++
	}

	println("MAPA294: tablas cargadas=", cargadas, "de", len(tablasDelCompositor),
		"monton usado=", d2cMontonUsado(), "libre=", d2cMontonLibre())

	if cargadas == 0 {
		println("MAPA294: FIN -- sin tablas no hay compositor")

		return
	}

	// ============================== 3. EL PUEBLO ==============================
	runtime.GC()

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	t0 := d2cMs()

	stamp := engine.LoadStamp(regionPueblo, presetA, archivoA)
	if stamp == nil {
		println("MAPA294: LoadStamp devolvio nil")

		return
	}

	tamano := stamp.Size()

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	pisos, paredes, crcGrilla := censo(engine)

	println("MAPA294: MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crcGrilla), "ms=", int(d2cMs()-t0))
	println("MAPA294: despues de componer: usado=", d2cMontonUsado(), "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena())

	// ============================== 4. EL RENDERER DEL MAPA ==============================
	term, err := d2term.NewTerminal()
	if err != nil {
		println("MAPA294: NewTerminal FALLO:", err.Error())

		return
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY, d2maprenderer.ComoConsola(ancho, alto), d2maprenderer.CacheDeTilesPerezosa())

	espacio := rendererDelMapa.TamanoDelCuadro()
	println("MAPA294: renderer del mapa listo; cuadro=", espacio.W, espacio.H,
		"cachePerezosa=", rendererDelMapa.CachePerezosa(), "monton usado=", d2cMontonUsado(),
		"libre=", d2cMontonLibre())

	// ============================== 5. EL CUADRO EN PANTALLA ==============================
	// El lienzo del cuadro lo pone el Renderer (vive en VRAM): el callback solo compone encima.
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) DC-002.94"); err != nil {
		println("MAPA294: el ciclo de cuadros salio con error:", err.Error())
	}

	println("MAPA294: FIN")
}

// dibujar es el callback del motor: el primer cuadro compone el mapa con `d2maprenderer` y lo mide; los
// demas no lo repintan (miden la cache ya armada, que es lo que dice si un cuadro en regimen pide memoria).
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
		println("MAPA294: el Renderer no entrego una Surface del PVR")

		return nil
	}

	pixeles := malla.Imagen().Pixeles()
	llenos := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	anchoReal, altoReal := superficie.GetSize()

	println("MAPA294: CUADRO", cuadro, "tamano=", anchoReal, altoReal, "pixelesNoVacios=", llenos,
		"de", anchoReal*altoReal, "crc32=", int(crc32.ChecksumIEEE(pixeles)), "ms=", ms,
		"montonUsado=", d2cMontonUsado(), "libre=", d2cMontonLibre())

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
