//go:build dreamcast

// DC-002.103 -- EL PUEBLO EN PANTALLA DE PUNTA A PUNTA: componer, soltar el arena y dibujar por el PVR.
//
// QUE MIDE. Es el criterio que DC-002.94 dejo pendiente y que DC-002.101/.102 dejaron a un paso: con el video
// encendido el monton alcanza para COMPONER el pueblo (crcGrilla -1596781041, el mismo del host) pero no para
// dibujarlo. Las tres piezas ya estaban medidas por separado -- el camino de dibujo (d2maprenderer + PVR,
// 640x480), la cache de tiles en INDICES (2.723.840 B en vez de 10.895.360 B, mismo crc32 1437022778) y el
// arena de bloques que se suelta (3.629.443 B) -- y esta corrida las junta, en el orden que dice `knobs.go`.
//
// EL ARRANQUE es el de DC-002.81/.88/.101/.102: video primero (el PVR necesita el monton vacio, DC-002.50),
// `d2asset.ArrancarComoConsola()`, el AssetManager con los dos MPQ por la Source de consola (D2DATA + PATCH_D2,
// un tramo del monton de C reusado con `free` inyectado), las 2 tablas que el compositor pide de verdad
// (DC-002.88: LvlTypes + LvlPrest), el pueblo por `d2mapengine` y el cuadro por `d2maprenderer` sobre
// `pvr.Surface` con la cache en indices y perezosa.
//
// EL RELOJ Y EL CUADRO. `ms` de cada cuadro es el tiempo del `Render` completo (lo que tarda la cache en
// armarse en el primero y lo que tarda el cuadro en regimen en los demas). El crc32 se calcula sobre los
// pixeles RGBA del lienzo del PVR (que vive en la VRAM, ver pvr/pantalla.go) con la MISMA funcion que el
// medidor del host (`console/dc/medidas/render-294`), asi los dos numeros son comparables byte a byte.
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

	// la pantalla de la consola y la celda del pueblo de las tarjetas anteriores (640x480 centrado en 28,20)
	ancho, alto = 640, 480
	celdaX      = 28.0
	celdaY      = 20.0

	// los dos numeros del cuadro de referencia del HOST (DC-002.94/.101/.103, medidos con
	// `go run ./console/dc/medidas/render-294 -perezoso -indices -cuadros 3`): 307.196 de 307.200 pixeles,
	// crc32 1437022778 y ~13 ms por cuadro en el host.
	pixelesHost = 307196
	crcHost     = 1437022778
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

	rendererDelMapa *d2maprenderer.MapRenderer
	cuadro          int
	picoMonton      int

	// tramoDePixeles: el tramo de la cache reservado al arranque (DC-002.104), nil si la palanca esta apagada.
	tramoDePixeles *d2maprenderer.TramoDeLaCache
)

func memoria(etapa string) {
	usado := d2cMontonUsado()
	if usado > picoMonton {
		picoMonton = usado
	}

	println("MAPA103: MEMORIA", etapa, "usado=", usado, "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena(), "sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()),
		"pico=", picoMonton)
}

func main() {
	println("MAPA103: arranca DC-002.103 -- el pueblo en pantalla de punta a punta; orden=", orden)
	println("MAPA103: cuadros=", cuadros, "modoTablas=", modoTablas, "soltarLecturas=", soltarLecturas,
		"tramoDeLaCache=", tramoDeLaCache, "tramoAlArranque=", tramoAlArranque)
	memoria("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("MAPA103: el video no arranco:", err.Error())

		return
	}

	println("MAPA103: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla())
	memoria("despues-del-video")

	// ============================== 0.b EL TRAMO DE LA CACHE (DC-002.104) ==============================
	// Se reserva ACA y no al crear el renderer: el monton de C (dlmalloc sobre `mm_sbrk`) se fragmenta con el
	// mapa y despues un pedido de 2,7 MB de un solo golpe ya no entra aunque "haya" memoria libre.
	if tramoDeLaCache > 0 && tramoAlArranque {
		tramoDePixeles = d2maprenderer.ReservarTramoDeLaCache(tramoDeLaCache)

		println("MAPA103: TRAMO DE LA CACHE reservado AL ARRANQUE bytes=", tramoDePixeles.Bytes())
		memoria("despues-del-tramo-de-la-cache")
	}

	// ============================== 1. EL MOTOR ==============================
	d2asset.ArrancarComoConsola()
	d2asset.UsarLiberadorDeExternos(func(p uintptr) { d2cLibre(p) })

	if modoTablas != "stream" {
		d2asset.ActivarTablasPorLaFuente()
	}

	println("MAPA103: tablasPorLaFuente=", d2asset.TablasPorLaFuenteActivas(),
		"liberadorDeExternos=", d2asset.HayLiberadorDeExternos())
	memoria("despues-de-arrancar-como-consola")

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		println("MAPA103: NewAssetManager FALLO:", err.Error())

		return
	}

	// las dos fuentes se abren UNA sola vez, con la Source de consola (DC-002.102: `AddSource` ademas abre los
	// dos MPQ DOS VECES y cuesta 355.328 B mas de monton de C).
	println("MAPA103: assetmanager listo")
	memoria("despues-del-assetmanager")

	for _, ruta := range []string{rutaDatos, rutaParche} { // el parche queda PRIMERO: se registra al final
		if !registrarFuente(am, ruta) {
			return
		}
	}

	println("MAPA103: fuentes en el loader=", len(am.Sources))
	memoria("despues-de-las-fuentes")

	// ============================== 2. LAS TABLAS ==============================
	cargadas := 0

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

	println("MAPA103: tablas cargadas=", cargadas, "de", tablasQueSeCargan)
	instrumentoDeLasFuentes("despues-de-las-tablas")
	memoria("despues-de-las-tablas")

	// el tramo de las tablas ya no hace falta: la tabla se parseo entera y lo que queda vivo son las filas
	if soltarLecturas {
		tramosAntes := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()
		tramosDespues := d2asset.EstadoDeLosTramos()

		println("MAPA103: LECTURAS soltadas; tramoAntes=", tramosAntes.BytesDelTramo, "pedidos=", tramosAntes.Pedidos,
			"reusos=", tramosAntes.Reusos, "liberados=", tramosDespues.Liberados)
		memoria("despues-de-soltar-las-lecturas-de-las-tablas")
	}

	// ============================== 3. EL PUEBLO ==============================
	runtime.GC()

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	t0 := d2cMs()

	stamp := engine.LoadStamp(regionPueblo, presetA, archivoA)
	if stamp == nil {
		println("MAPA103: LoadStamp devolvio nil")

		return
	}

	tamano := stamp.Size()

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	pisos, paredes, crcGrilla := censo(engine)

	println("MAPA103: MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crcGrilla), "ms=", int(d2cMs()-t0))

	estado := d2asset.EstadoDeLosTiles()
	println("MAPA103: ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
		"tramos=", estado.BloquesTramos, "capacidad=", estado.BloquesCapacidad, "scratch=", estado.Scratch)
	instrumentoDeLasFuentes("despues-de-componer")
	memoria("despues-de-componer")

	if soltarLecturas {
		tramos := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()

		println("MAPA103: LECTURAS soltadas despues de componer; tramo=", tramos.BytesDelTramo,
			"lecturas=", tramos.Lecturas, "bytes=", int(tramos.Bytes))
		memoria("despues-de-soltar-las-lecturas-del-mapa")
	}

	// ============================== 4. LO QUE SE SUELTA ANTES DE DIBUJAR ==============================
	switch orden {
	case "tarjeta":
		// el orden que pide la tarjeta: TODO el arena antes del renderer. Va contra el contrato de DC-002.101
		// (los `EncodedData` de cada tile apuntan a estos tramos): se corre para medir que pasa de verdad.
		println("MAPA103: ORDEN tarjeta -- se suelta el arena ENTERO antes de armar la cache (contra el contrato)")
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("MAPA103: ARENA DE BLOQUES SOLTADA (antes del renderer) tramos=", tramos, "bytes=", bytes)
		memoria("despues-de-soltar-el-arena")
	case "prestamo":
		// la mitad que si se puede devolver sin invalidar nada: el buffer prestado de la lectura del .dt1
		println("MAPA103: ORDEN prestamo -- se suelta SOLO el buffer prestado de lectura (el arena de bloques queda)")
		bytes := d2asset.SoltarPrestamoDeLecturaDeTiles()
		println("MAPA103: PRESTAMO DE LECTURA SOLTADO bytes=", bytes)
		memoria("despues-de-soltar-el-prestamo")
	default:
		println("MAPA103: ORDEN contrato -- el arena queda vivo hasta que la cache este armada")
	}

	// ============================== 5. EL RENDERER (CACHE EN INDICES) ==============================
	term, err := d2term.NewTerminal()
	if err != nil {
		println("MAPA103: NewTerminal FALLO:", err.Error())

		return
	}

	memoria("antes-del-renderer")

	opciones := []d2maprenderer.Opcion{
		d2maprenderer.ComoConsola(ancho, alto),
		d2maprenderer.CacheDeTilesPerezosa(),
		d2maprenderer.CacheDeTilesEnIndices(),
	}

	if tramoDePixeles != nil {
		// DC-002.104: los pixeles de la cache salen del tramo reservado al arranque (en la consola, monton de C)
		// y no del semiespacio del recolector.
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramo(tramoDePixeles))
		println("MAPA103: TRAMO DE LA CACHE en uso (reservado al arranque) bytes=", tramoDePixeles.Bytes())
	} else if tramoDeLaCache > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramo(tramoDeLaCache))
		println("MAPA103: TRAMO DE LA CACHE pedido TARDE (en el renderer) bytes=", tramoDeLaCache)
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY, opciones...)

	espacio := rendererDelMapa.TamanoDelCuadro()
	println("MAPA103: renderer listo; cuadro=", espacio.W, espacio.H,
		"cachePerezosa=", rendererDelMapa.CachePerezosa(), "cacheEnIndices=", rendererDelMapa.CacheDeIndices())
	censoDelTramo("despues-del-renderer")
	memoria("despues-del-renderer")

	// ============================== 6. EL CUADRO EN PANTALLA ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) DC-002.103"); err != nil {
		println("MAPA103: el ciclo de cuadros salio con error:", err.Error())
	}

	println("MAPA103: FIN pico del monton=", picoMonton, "de arena=", d2cMontonArena())
}

// registrarFuente abre el MPQ con la Source de consola, le inyecta el `free` del shim y la pone PRIMERA en
// el loader del motor (DC-002.96/.99). Devuelve false si el MPQ no abre.
func registrarFuente(am *d2asset.AssetManager, ruta string) bool {
	f, err := fuenteconsola.Abrir(ruta)
	if err != nil {
		println("MAPA103: MPQ-ABRIR FALLO:", ruta, err.Error())

		return false
	}

	f.UsarLiberador(fuenteconsola.Liberador(d2cLibre))
	f.RegistrarEn(am.Loader)

	println("MAPA103: fuente registrada", ruta)
	fuentesDeConsola = append(fuentesDeConsola, f)

	return true
}

// censoDelTramo (DC-002.104): donde viven los pixeles de la cache. `imagenes` del tramo contra `derrames` es
// el numero que dice si el tramo alcanza; `bytesDerramados` cuanto habria que agrandarlo.
func censoDelTramo(etapa string) {
	if rendererDelMapa == nil {
		return
	}

	e := rendererDelMapa.CensoDelTramo()
	println("MAPA103: TRAMO", etapa, "activa=", e.Activa, "bytes=", e.Bytes, "usado=", e.Usado,
		"libre=", e.Libre, "imagenes=", e.Imagenes, "derrames=", e.Derrames, "bytesDerramados=", e.BytesDerramados)
}

// instrumentoDeLasFuentes: el censo de la fuente (pedidos, liberados, vivos, prestamo) para el log.
func instrumentoDeLasFuentes(etapa string) {
	for _, f := range fuentesDeConsola {
		e := f.Estado()
		println("MAPA103: FUENTE", etapa, f.Path(), "abiertos=", e.Abiertos, "enBuffer=", e.LecturasEnBuffer,
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
		println("MAPA103: TABLA FALLO (fuente):", ruta, err.Error())

		return false
	}

	println("MAPA103: TABLA fuente", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// cargarTablaPorStream: el camino de HOY (`LoadDataDictionary` por stream del MPQ), para el A/B.
func cargarTablaPorStream(am *d2asset.AssetManager, ruta string) bool {
	usado := d2cMontonUsado()
	tamano, _ := am.TamanoDeArchivo(ruta)

	dict, err := am.LoadDataDictionaryPorStream(ruta)
	if err != nil {
		println("MAPA103: TABLA FALLO (stream):", ruta, err.Error())

		return false
	}

	if err := am.Records.Load(ruta, dict); err != nil {
		println("MAPA103: TABLA FALLO (stream, records):", ruta, err.Error())

		return false
	}

	println("MAPA103: TABLA stream", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// dibujar: el cuadro 1 arma la cache perezosa (y por eso es el que mas tarda), y con el orden "contrato" o
// "prestamo" recien AHI se suelta el arena de bloques: despues del cuadro 1 nadie mira los `EncodedData`.
func dibujar(superficie d2interface.Surface) error {
	if rendererDelMapa == nil {
		return nil
	}

	cuadro++

	if cuadro > cuadros {
		return nil
	}

	println("MAPA103: CUADRO", cuadro, "ARRANCA libre=", d2cMontonLibre())

	t0 := d2cMs()

	rendererDelMapa.Render(superficie)

	ms := int(d2cMs() - t0)

	malla, ok := superficie.(*pvr.Surface)
	if !ok {
		println("MAPA103: el Renderer no entrego una Surface del PVR")

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
	println("MAPA103: CUADRO", cuadro, "pixelesNoVacios=", llenos, "de", ancho*alto, "crc32=", int(crudo),
		"hostPixeles=", pixelesHost, "hostCrc=", int(crcHost), "ms=", ms,
		"cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes, "cacheEnIndices=", cache.EnIndices)
	censoDelTramo("despues-del-cuadro")
	memoria("despues-del-cuadro")

	if cuadro == 1 && orden != "tarjeta" {
		// la cache del cuadro esta armada: el arena de bloques ya no hace falta (contrato de DC-002.101)
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("MAPA103: ARENA DE BLOQUES SOLTADA (despues de la cache) tramos=", tramos, "bytes=", bytes,
			"libre=", d2cMontonLibre())
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
