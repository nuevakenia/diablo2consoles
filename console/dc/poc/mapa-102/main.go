//go:build dreamcast

// DC-002.102 -- EL CENSO DEL ARRANQUE CON VIDEO Y LOS TRES CAMINOS DE LA MEMORIA.
//
// QUE MIDE. DC-002.101 dejo medido que con el video encendido el POC del pueblo muere ANTES de componer, y
// que no es la cache de tiles (ya en indices: 2,60 MB y el mismo cuadro al byte) sino el reparto del monton:
//
//	gc-imagen-hi            0x8C70F000   (fin de la imagen del binario)
//	semiespacio 0 / 1       0x8C70F000 / 0x8C90F040   (2 MB cada uno, 4.194.304 B de UNA vez)
//	techo del monton        0x8CFF0000
//	monton de C disponible  5.115.840 B, de los cuales 4.354.344 son los dos semiespacios
//	con las 2 tablas        usado 5.033.808 B, libre 2.160 B  -> muere en la lectura de la 3ra
//
// Esta tarjeta mide TRES caminos y aplica el mejor:
//
//	(a) UN SOLO SEMIESPACIO. El recolector de libgodc es de COPIA (`gc_copy.c`: `from = space[1-active]`,
//	    `to = space[active]`), asi que la pregunta "funciona con uno?" se contesta midiendo, no leyendo.
//	(b) SEMIESPACIO MENOR. Cada 256 KB menos son 512 KB mas de monton de C (son dos); el costo es que el
//	    recolector copia mas seguido y que el conjunto vivo tiene que entrar en el semiespacio.
//	(c) LAS TABLAS POR LA FUENTE CON DUENO. Hoy `LoadDataDictionary` abre un STREAM del MPQ, que
//	    prefetchea el archivo comprimido entero y no lo devuelve: +324.112 B de monton de C por las dos
//	    tablas. Por el tramo de la fuente (monton de C, UNO, reusado, con `free` inyectado) no se pide
//	    memoria por tabla. Medido en la MISMA corrida con el MISMO archivo por los dos caminos.
//
// EL ARRANQUE es el de DC-002.81/.88/.101: video primero (el PVR necesita el monton vacio, DC-002.50),
// `d2asset.ArrancarComoConsola()`, el AssetManager con los dos MPQ, las 2 tablas que el compositor pide
// (DC-002.88: LvlTypes + LvlPrest), el pueblo por `d2mapengine` y el cuadro por `d2maprenderer` sobre
// `pvr.Surface` con la cache en indices.
package main

import (
	"hash/crc32"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2term"
	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
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

	// la pantalla de la consola y la celda del pueblo de las tarjetas anteriores (640x480 centrado en 28,20):
	// el cuadro de referencia es el del host, 307.196 de 307.200 pixeles, crc32 1437022778 (DC-002.94)
	ancho, alto = 640, 480
	celdaX      = 28.0
	celdaY      = 20.0

	cuadros = 3

	// los dos numeros del cuadro de referencia del HOST (DC-002.94/.101), para comparar en la consola
	pixelesHost = 307196
	crcHost     = 1437022778

	// modoTablas elige que camino leen las tablas (DC-002.102): "stream" (lo de hoy), "fuente" (el nuevo,
	// por el tramo de la fuente con dueno) o "ab" (los dos, sobre el MISMO archivo, para comparar deltas).
	modoTablas = "fuente"

	// soltarLecturas: devolver el tramo de lecturas del motor antes de componer (y antes de armar la cache)
	soltarLecturas = true
)

// tablasDelCompositor: se cargan SOLO las dos primeras, que son las que el compositor pide de verdad
// (DC-002.88: LvlTypes + LvlPrest = +324.112 B). Cargar las 12 mataba al recolector en la cuarta.
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

var (
	// fuentesDeConsola: las dos Source de consola (parche y datos), para el instrumento.
	fuentesDeConsola []*fuenteconsola.Fuente

	rendererDelMapa  *d2maprenderer.MapRenderer
	cuadro           int
	crudPrimerCuadro uint32
	picoMonton       int
)

func memoria(etapa string) {
	usado := d2cMontonUsado()
	if usado > picoMonton {
		picoMonton = usado
	}

	println("MAPA102: MEMORIA", etapa, "usado=", usado, "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena(), "sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()),
		"pico=", picoMonton)
}

func main() {
	println("MAPA102: arranca DC-002.102 -- el censo del arranque con video y los tres caminos del monton")
	println("MAPA102: modo de tablas=", modoTablas, "soltarLecturas=", soltarLecturas)
	memoria("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("MAPA102: el video no arranco:", err.Error())

		return
	}

	println("MAPA102: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla())
	memoria("despues-del-video")

	// ============================== 1. EL MOTOR ==============================
	d2asset.ArrancarComoConsola()
	d2asset.UsarLiberadorDeExternos(func(p uintptr) { d2cLibre(p) })

	if modoTablas != "stream" {
		d2asset.ActivarTablasPorLaFuente()
	}

	println("MAPA102: tablasPorLaFuente=", d2asset.TablasPorLaFuenteActivas())
	memoria("despues-de-arrancar-como-consola")

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		println("MAPA102: NewAssetManager FALLO:", err.Error())

		return
	}

	// OJO, MEDIDO EN ESTA TARJETA: las dos fuentes se abren UNA sola vez, con la Source de consola (abajo).
	// Agregar ademas las fuentes MPQ de siempre (`AddSource`) abre los dos MPQ DOS VECES y cuesta 355.328 B
	// mas de monton de C (medido: assetmanager +355.328 y despues las fuentes +355.328). La Fuente de consola
	// es tambien `asset.Source` (Open/Exists/Path), asi que reemplaza a la de siempre sin perder nada.
	println("MAPA102: assetmanager listo")
	memoria("despues-del-assetmanager")

	// ============================== 1b. LAS DOS FUENTES DE CONSOLA (DC-002.96/.99) ==============================
	// Sin esto el motor lee por el `mpq.Source` de siempre y NADA pasa por el tramo: medido en la corrida de
	// "ab" (MAPA102: tramo pedidos= 0), o sea que las tablas y los `.dt1` seguian pidiendo memoria por archivo.
	// Con la Fuente de consola registrada PRIMERO, `LoadFile` lee en UN tramo del monton de C (reusado, con
	// `free` inyectado) y los `.dt1` se leen con el prestamo de trabajo de la fuente.
	for _, ruta := range []string{rutaDatos, rutaParche} { // el parche queda PRIMERO: se registra al final
		if !registrarFuente(am, ruta) {
			return
		}
	}

	println("MAPA102: fuentes en el loader=", len(am.Sources))
	memoria("despues-de-las-fuentes")

	// ============================== 2. LAS TABLAS ==============================
	cargadas := 0

	// el A/B sobre el MISMO archivo: LvlPrest por el stream de hoy (que es el que dejo medido el delta de
	// 324.112 B en DC-002.101) y despues por el tramo de la fuente.
	switch modoTablas {
	case "ab":
		if cargarTablaPorLaFuente(am, tablasDelCompositor[0]) {
			cargadas++
		}

		cargarTablaPorStream(am, tablasDelCompositor[1])

		if cargarTablaPorLaFuente(am, tablasDelCompositor[1]) {
			cargadas++
		}
	default:
		for i := 0; i < tablasQueSeCargan && i < len(tablasDelCompositor); i++ {
			if modoTablas == "stream" {
				if cargarTablaPorStream(am, tablasDelCompositor[i]) {
					cargadas++
				}

				continue
			}

			if cargarTablaPorLaFuente(am, tablasDelCompositor[i]) {
				cargadas++
			}
		}
	}

	println("MAPA102: tablas cargadas=", cargadas, "de", tablasQueSeCargan)
	instrumentoDeLasFuentes("despues-de-las-tablas")
	memoria("despues-de-las-tablas")

	// el tramo de las tablas ya no hace falta: la tabla se parseo entera y lo que queda vivo son las filas
	if soltarLecturas {
		tramosAntes := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()
		tramosDespues := d2asset.EstadoDeLosTramos()

		println("MAPA102: LECTURAS soltadas; tramoAntes=", tramosAntes.BytesDelTramo, "pedidos=", tramosAntes.Pedidos,
			"reusos=", tramosAntes.Reusos, "liberados=", tramosDespues.Liberados)
		memoria("despues-de-soltar-las-lecturas-de-las-tablas")
	}

	// ============================== 3. EL PUEBLO ==============================
	runtime.GC()

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	t0 := d2cMs()

	stamp := engine.LoadStamp(regionPueblo, presetA, archivoA)
	if stamp == nil {
		println("MAPA102: LoadStamp devolvio nil")

		return
	}

	tamano := stamp.Size()

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	pisos, paredes, crcGrilla := censo(engine)

	println("MAPA102: MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crcGrilla), "ms=", int(d2cMs()-t0))

	estado := d2asset.EstadoDeLosTiles()
	println("MAPA102: ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
		"tramos=", estado.BloquesTramos, "capacidad=", estado.BloquesCapacidad, "scratch=", estado.Scratch)
	instrumentoDeLasFuentes("despues-de-componer")
	memoria("despues-de-componer")

	if soltarLecturas {
		tramos := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()

		println("MAPA102: LECTURAS soltadas despues de componer; tramo=", tramos.BytesDelTramo,
			"lecturas=", tramos.Lecturas, "bytes=", int(tramos.Bytes))
		memoria("despues-de-soltar-las-lecturas-del-mapa")
	}

	// ============================== 4. EL RENDERER (CACHE EN INDICES) ==============================
	term, err := d2term.NewTerminal()
	if err != nil {
		println("MAPA102: NewTerminal FALLO:", err.Error())

		return
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY,
		d2maprenderer.ComoConsola(ancho, alto),
		d2maprenderer.CacheDeTilesPerezosa(),
		d2maprenderer.CacheDeTilesEnIndices())

	espacio := rendererDelMapa.TamanoDelCuadro()
	println("MAPA102: renderer listo; cuadro=", espacio.W, espacio.H,
		"cachePerezosa=", rendererDelMapa.CachePerezosa(), "cacheEnIndices=", rendererDelMapa.CacheDeIndices())
	memoria("despues-del-renderer")

	// ============================== 5. EL CUADRO EN PANTALLA ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) DC-002.102"); err != nil {
		println("MAPA102: el ciclo de cuadros salio con error:", err.Error())
	}

	println("MAPA102: FIN pico del monton=", picoMonton, "de arena=", d2cMontonArena())
}

// registrarFuente abre el MPQ con la Source de consola, le inyecta el `free` del shim y la pone PRIMERA en
// el loader del motor (DC-002.96/.99). Devuelve false si el MPQ no abre.
func registrarFuente(am *d2asset.AssetManager, ruta string) bool {
	f, err := fuenteconsola.Abrir(ruta)
	if err != nil {
		println("MAPA102: MPQ-ABRIR FALLO:", ruta, err.Error())

		return false
	}

	f.UsarLiberador(fuenteconsola.Liberador(d2cLibre))
	f.RegistrarEn(am.Loader)

	println("MAPA102: fuente registrada", ruta)
	fuentesDeConsola = append(fuentesDeConsola, f)

	return true
}

// instrumentoDeLasFuentes: el censo de la fuente (pedidos, liberados, vivos, prestamo) para el log.
func instrumentoDeLasFuentes(etapa string) {
	for _, f := range fuentesDeConsola {
		e := f.Estado()
		println("MAPA102: FUENTE", etapa, f.Path(), "abiertos=", e.Abiertos, "enBuffer=", e.LecturasEnBuffer,
			"conPrestamo=", e.LecturasConPrestamo, "pedidos=", e.Pedidos, "liberados=", e.Liberados,
			"vivos=", int(e.BytesVivos), "prestamos=", e.Prestamos, "prestamosLib=", e.PrestamosLiberados,
			"prestamoReservado=", e.PrestamoReservado)
	}
}

// cargarTablaPorLaFuente: la tabla por el tramo de la fuente (monton de C, reusado, con dueno).
func cargarTablaPorLaFuente(am *d2asset.AssetManager, ruta string) bool {
	usado := d2cMontonUsado()
	tamano, _ := am.TamanoDeArchivo(ruta)

	if err := am.LoadRecords(ruta); err != nil {
		println("MAPA102: TABLA FALLO (fuente):", ruta, err.Error())

		return false
	}

	println("MAPA102: TABLA fuente", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// cargarTablaPorStream: el camino de HOY (`LoadDataDictionary` por stream del MPQ), para el A/B.
func cargarTablaPorStream(am *d2asset.AssetManager, ruta string) bool {
	usado := d2cMontonUsado()
	tamano, _ := am.TamanoDeArchivo(ruta)

	dict, err := am.LoadDataDictionaryPorStream(ruta)
	if err != nil {
		println("MAPA102: TABLA FALLO (stream):", ruta, err.Error())

		return false
	}

	if err := am.Records.Load(ruta, dict); err != nil {
		println("MAPA102: TABLA FALLO (stream, records):", ruta, err.Error())

		return false
	}

	println("MAPA102: TABLA stream", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// dibujar: el cuadro 1 compone (y arma la cache perezosa), despues se SUELTA el arena de bloques, y los
// cuadros 2 y 3 se dibujan con la cache ya armada. Los tres crc32 tienen que ser el mismo.
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
		println("MAPA102: el Renderer no entrego una Surface del PVR")

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
	println("MAPA102: CUADRO", cuadro, "pixelesNoVacios=", llenos, "de", ancho*alto, "crc32=", int(crudo),
		"ms=", ms, "cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes,
		"cacheEnIndices=", cache.EnIndices, "hostPixeles=", pixelesHost, "hostCrc=", int(crcHost))
	memoria("despues-del-cuadro")

	if cuadro == 1 {
		crudPrimerCuadro = crudo

		// el arena ya no hace falta: la cache del cuadro esta armada y el cuadro se pinta de ahi
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("MAPA102: ARENA DE BLOQUES SOLTADA tramos=", tramos, "bytes=", bytes)
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
