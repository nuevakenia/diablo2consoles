//go:build dreamcast

// DC-002.112 -- EL PRIMER CUADRO DE JUEGO: EL PUEBLO **CON EL JUGADOR ENCIMA**, EN EL PVR.
//
// QUE MIDE. Las dos mitades del cuadro ya estaban hechas y nunca se habian juntado:
//
//   - el PUEBLO se compone con el motor (`d2mapengine`), se dibuja con el camino de dibujo del motor
//     (`d2maprenderer` con la cache de tiles en INDICES y en la VRAM del PVR) y se presenta a 640x480 --
//     DC-002.103/.106/.108, crc32 del cuadro 1437022778, que es el del host;
//   - el JUGADOR se compone desde el `.dcc` del COF de la amazona con CERO recolecciones (DC-002.98/.100) y
//     se dibuja con `RenderIndicesTintado` sobre la superficie del PVR (DC-002.69/.70/.97).
//
// Esta corrida dibuja los DOS en el MISMO lienzo y mide, en ese orden:
//
//  1. el crc32 del cuadro ANTES de dibujar al jugador (el mapa SOLO): tiene que ser 1437022778, el mismo de
//     DC-002.106/.108 y el del host (`console/dc/medidas/render-294`);
//  2. el crc32 del cuadro CON el jugador, los pixeles del area del jugador (los que el personaje pisa) y el
//     resto del cuadro;
//  3. el crc32 del cuadro DESPUES de devolver la caja del jugador a lo que habia: otra vez 1437022778. Esa es
//     la prueba de que el jugador solo toca su caja y el mapa queda intacto afuera;
//  4. el costo del jugador aparte: monton de C y semiespacio ANTES y DESPUES de componerlo y de dibujarlo.
//
// EL ORDEN DE LOS PASOS NO ES DECORACION. El PVR necesita el monton de C casi vacio para arrancar (DC-002.50),
// asi que el video va PRIMERO (como en DC-002.103); y el jugador se compone ANTES de que el pueblo se componga
// y de que se suelten las fuentes MPQ (que es lo que deja el monton para la cache de tiles), porque la
// decodificacion del `.dcc` pide ~86 KB de arena que se devuelven en el acto: el pico del jugador no se suma al
// pico de componer el pueblo.
//
// LO QUE NO ENTRA (la tarjeta lo dice): mover al jugador (DC-002.113, con maple) y tocar el orden de capas o la
// logica de `d2maprenderer`. El jugador se dibuja ENCIMA, despues del `Render` del mapa.
//
// LOS NUMEROS DE REFERENCIA, y de donde salen:
//
//	pixelesHost/crcHost   307.196 de 307.200 px y crc32 1437022778 -- el cuadro del pueblo en el host
//	                      (DC-002.94/.101/.106, `render-294 -perezoso -indices`)
//	amazona NU dir 0      caja 26x79, 7 capas dibujadas, 896 pixeles y crc32 1857478036 en el cuadro 0
//	                      (DC-002.38/.100, medido en la consola Y en el host con el mismo `personaje.go`)
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
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

//extern d2c_soltar_filas
func d2cSoltarFilas(mapa uintptr) int

//extern d2c_soltar_tramo
func d2cSoltarTramo(tramo uintptr, tamano int) int

//extern d2c_servible
func d2cServible(tope uint) uint

//extern d2c_soltar_bloque
func d2cSoltarBloque(p uintptr, tamano int) int

//extern d2c_gc_usado
func d2cGcUsado() uint

//extern d2c_gc_total
func d2cGcTotal() uint

//extern d2c_gc_colecciones
func d2cGcColecciones() uint

//extern d2c_free_external
func d2cFreeExternal(p uintptr)

//extern d2c_heap_libre
func d2cHeapLibre() uint

//extern d2c_gc_n
func d2cGcN() uint

func init() { d2cSerialInit() }

const (
	// El disco: los TRES MPQ del juego que hacen falta. El pueblo (los `.ds1`/`.dt1` y la paleta del acto) y
	// la paleta de las UNIDADES estan en d2data.mpq; las tablas del compositor, en patch_D2.mpq; y el COF y los
	// `.dcc` de la amazona, en d2char.mpq (medido en DC-002.38: en d2data.mpq la carpeta chars solo trae los
	// COF de los mercenarios).
	rutaParche  = "/cd/JUEGO/PATCH_D2.MPQ"
	rutaDatos   = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChar = "/cd/JUEGO/D2CHAR.MPQ"

	// la paleta de las unidades (personajes): 768 B, B,G,R.
	palUnidadesMPQ = "data/global/palette/Units/pal.dat"

	regionPueblo = d2enum.RegionAct1Town
	presetA      = 1
	archivoA     = 0

	// la pantalla de la consola y la celda del pueblo de las tarjetas anteriores: 640x480 centrado en 28,20.
	ancho, alto = 640, 480
	celdaX      = 28.0
	celdaY      = 20.0

	// los dos numeros del cuadro de referencia del HOST (DC-002.94/.101/.106, medidos con
	// `go run ./console/dc/medidas/render-294 -perezoso -indices -cuadros 3`): 307.196 de 307.200 pixeles y
	// crc32 1437022778.
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
	// fuentesDeConsola: las dos Source de consola (datos y parche), para el instrumento y para devolverlas.
	fuentesDeConsola []*fuenteconsola.Fuente

	rendererDelMapa *d2maprenderer.MapRenderer
	cuadro          int
	picoMonton      int

	// tramoDePixeles: el tramo de la cache reservado al arranque (DC-002.104), nil si la palanca esta apagada.
	tramoDePixeles *d2maprenderer.TramoDeLaCache

	// tablasCargadas (DC-002.105): las tablas que se cargaron de verdad, por nombre.
	tablasCargadas []string

	// ============================== EL JUGADOR (DC-002.112) ==============================
	//
	// jugador es el ciclo compuesto (NU, direccion 0, todos sus cuadros) y palJugador la paleta de las
	// unidades ya en RGBA. Los dos viven toda la corrida: el ciclo son ~16 KB de indices y la paleta 1 KB.
	jugador    *CicloDelJugador
	palJugador [256][4]uint8
	jugadorOK  bool

	// cajaMapa: lo que habia en el lienzo debajo del jugador, para devolverlo y poder medir el mapa sin el.
	// Se pide UNA vez (la caja del jugador es fija en este POC) y se reusa cuadro a cuadro.
	cajaMapa []byte

	// crcDelMapaSolo: el crc32 del cuadro antes de dibujar al jugador (el numero que tiene que seguir siendo
	// 1437022778). Se guarda para el resumen del final.
	crcDelMapaSolo uint32
)

func memoria(etapa string) {
	usado := d2cMontonUsado()
	if usado > picoMonton {
		picoMonton = usado
	}

	// DC-002.111: el pico del monton de C DURANTE la carga de los `.dt1` (la sonda de d2asset).
	if picoDeLaCarga := d2asset.PicoDelMontonDeC(); picoDeLaCarga > picoMonton {
		picoMonton = picoDeLaCarga
	}

	println("JUEGO112: MEMORIA", etapa, "usado=", usado, "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena(), "sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()),
		"pico=", picoMonton)
}

// semiespacio (DC-002.109): el censo del semiespacio del recolector -- lo que ocupa, cuanto mide y cuantas
// recolecciones van. Es el instrumento que separa "no hay memoria" de "no hay memoria DONDE se pide".
func semiespacio(etapa string) {
	usado, total := int(d2cGcUsado()), int(d2cGcTotal())
	pct := 0

	if total > 0 {
		pct = usado * 100 / total
	}

	println("JUEGO112: SEMIESPACIO", etapa, "usado=", usado, "total=", total, "pct=", pct,
		"colecciones=", int(d2cGcColecciones()))
}

// pruebaDeEscrituraEnVram (DC-002.108): escribe un patron conocido en un bloque de la VRAM y lo vuelve a leer.
// Separa "la VRAM no se puede usar como RAM" de "el dato se corrompe en otra parte".
func pruebaDeEscrituraEnVram() {
	const bytes = 348160

	b := pvr.VramReservar(bytes)
	if b == nil {
		println("JUEGO112: PRUEBA VRAM no se pudo reservar", bytes, "B")

		return
	}

	for i := 0; i < len(b); i++ {
		b[i] = byte(i*7 + 3)
	}

	malos := 0
	primero := -1

	for i := 0; i < len(b); i++ {
		if b[i] != byte(i*7+3) {
			malos++

			if primero < 0 {
				primero = i
			}
		}
	}

	println("JUEGO112: PRUEBA VRAM bytes=", len(b), "malos=", malos, "primeroMal=", primero,
		"leido=", int(b[0]), "esperado=", 3)

	pvr.VramSoltar(b)
}

// vram (DC-002.108): el censo de la VRAM del PVR en cada etapa.
func vram(etapa string) {
	c := pvr.FotoDeLaVram()

	println("JUEGO112: VRAM", etapa, "total=", c.Total, "disponible=", c.Disponible, "pedida=", c.Pedida,
		"pedidos=", c.Pedidos, "pedidoMayor=", c.PedidoMayor, "lienzo=", c.LienzoDelCuadro)
}

func main() {
	println("JUEGO112: arranca DC-002.112 -- el pueblo CON el jugador en el PVR")
	println("JUEGO112: cuadros=", cuadros, "jugador=", jugadorKnob, "modoTablas=", modoTablas,
		"soltarLecturas=", soltarLecturas, "tramoDeLaCache=", tramoDeLaCache, "tramoEnPedazos=", tramoEnPedazos,
		"sonda=", pedazoLoDiceElMonton, "tramoAlArranque=", tramoAlArranque, "cacheEnVram=", cacheEnVram,
		"soltarDespuesDeComponer=", soltarDespuesDeComponer, "orden=", orden)

	memoria("al-arrancar")
	semiespacio("al-arrancar")
	vram("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	// El PVR necesita el monton de C casi vacio para arrancar (DC-002.50): es el mismo orden de DC-002.103.
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("JUEGO112: el video no arranco:", err.Error())

		return
	}

	println("JUEGO112: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla())
	memoria("despues-del-video")
	vram("despues-del-video")

	if pruebaDeVram {
		pruebaDeEscrituraEnVram()
	}

	if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		println("JUEGO112: el tramo de la cache se pide EN PEDAZOS (techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "): no se reserva nada al arranque (DC-002.106)")
	} else if tramoDeLaCache > 0 && tramoAlArranque {
		tramoDePixeles = d2maprenderer.ReservarTramoDeLaCache(tramoDeLaCache)
		println("JUEGO112: TRAMO DE LA CACHE reservado AL ARRANQUE bytes=", tramoDePixeles.Bytes())
	}

	memoria("despues-del-tramo-de-la-cache")

	// ============================== 1. EL MOTOR ==============================
	d2asset.ArrancarComoConsola()
	d2asset.UsarLiberadorDeExternos(func(p uintptr) { d2cLibre(p) })

	if arenaDelDS1 == "encendido" {
		d2ds1.ActivarArenaDelDS1()
		d2ds1.UsarLiberadorDeLaArena(func(p uintptr, bytes int) int { return d2cSoltarBloque(p, bytes) })
	}

	arena := d2ds1.EstadoDeLaArena()
	println("JUEGO112: ARENA DEL DS1 palanca=", arenaDelDS1, "encendida=", arena.Encendida,
		"conLiberador=", arena.ConLiberador, "bytesPorCelda=", d2ds1.BytesPorCelda())

	d2asset.UsarScratchPorArchivo(scratchPorArchivo)
	println("JUEGO112: SCRATCH DE LECTURA porArchivo=", d2asset.ScratchPorArchivoActivo(),
		"liberador=", d2asset.HayLiberadorDeExternos())

	d2asset.UsarMedidorDelMontonDeC(func() int { return d2cMontonUsado() })

	if tramoDelArenaPorElMonton {
		d2asset.UsarSondaDelMontonDeC(func(tope int) int { return int(d2cServible(uint(tope))) })
	}

	println("JUEGO112: TRAMO DEL ARENA DE BLOQUES porElMonton=", tramoDelArenaPorElMonton,
		"sondaInstalada=", d2asset.HaySondaDelMontonDeC())

	d2asset.UsarSoltarTramosAlReiniciar(soltarTramosAlCambiarDeNivel)
	println("JUEGO112: TRAMOS AL CAMBIAR DE NIVEL seDevuelven=", d2asset.SoltarTramosAlReiniciarActivo())

	d2asset.UsarLiberadorDeFilas(&d2records.LiberadorDeFilas{
		SoltarMapa:  func(mapa unsafe.Pointer) int { return d2cSoltarFilas(uintptr(mapa)) },
		SoltarTramo: func(bloque unsafe.Pointer, bytes int) int { return d2cSoltarTramo(uintptr(bloque), bytes) },
	})

	if modoTablas != "stream" {
		d2asset.ActivarTablasPorLaFuente()
	}

	println("JUEGO112: tablasPorLaFuente=", d2asset.TablasPorLaFuenteActivas(),
		"liberadorDeExternos=", d2asset.HayLiberadorDeExternos())
	memoria("despues-de-arrancar-como-consola")

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		println("JUEGO112: NewAssetManager FALLO:", err.Error())

		return
	}

	println("JUEGO112: assetmanager listo")
	memoria("despues-del-assetmanager")

	for _, ruta := range []string{rutaDatos, rutaParche} { // el parche queda PRIMERO: se registra al final
		if !registrarFuente(am, ruta) {
			return
		}
	}

	println("JUEGO112: fuentes en el loader=", len(am.Sources))
	memoria("despues-de-las-fuentes")

	// ============================== 2. LAS TABLAS ==============================
	cargadas := 0

	for i := 0; i < tablasQueSeCargan && i < len(tablasDelCompositor); i++ {
		var ok bool

		if modoTablas == "stream" {
			ok = cargarTablaPorStream(am, tablasDelCompositor[i])
		} else {
			ok = cargarTablaPorLaFuente(am, tablasDelCompositor[i])
		}

		if ok {
			cargadas++
			tablasCargadas = append(tablasCargadas, tablasDelCompositor[i])
		}
	}

	println("JUEGO112: tablas cargadas=", cargadas, "de", tablasQueSeCargan)
	instrumentoDeLasFuentes("despues-de-las-tablas")
	memoria("despues-de-las-tablas")

	if soltarLecturas {
		tramosAntes := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()
		tramosDespues := d2asset.EstadoDeLosTramos()

		println("JUEGO112: LECTURAS soltadas; tramoAntes=", tramosAntes.BytesDelTramo, "pedidos=", tramosAntes.Pedidos,
			"reusos=", tramosAntes.Reusos, "liberados=", tramosDespues.Liberados)
		memoria("despues-de-soltar-las-lecturas-de-las-tablas")
	}

	// ============================== 3. EL JUGADOR (DC-002.112) ==============================
	// VA ACA, Y EL LUGAR ES LA MITAD DEL TRABAJO. Antes de componer el pueblo y antes de soltar las fuentes
	// MPQ (`soltarLoQueElCuadroNoLee`), porque:
	//   - los `.cof`/`.dcc` de la amazona y la paleta de las unidades salen de los MPQ (despues de soltarlas no
	//     se puede leer un byte);
	//   - la decodificacion del `.dcc` pide su arena al monton de C (DC-002.100) y la devuelve en el acto, o sea
	//     que su pico NO se suma al pico de componer el pueblo (que es el que mide 11.031.920 B);
	//   - lo que queda vivo del jugador es el ciclo compuesto: 8 cuadros de 26x79 indices (~16 KB).
	if jugadorKnob == "encendido" {
		memoria("jugador-antes")
		semiespacio("jugador-antes")

		cargarJugador(am)

		memoria("jugador-despues")
		semiespacio("jugador-despues")

		runtime.GC()

		memoria("jugador-despues-de-recolectar")
		semiespacio("jugador-despues-de-recolectar")
	} else {
		println("JUEGO112: JUGADOR apagado (la corrida de referencia de DC-002.106/.108: el pueblo solo)")
	}

	// ============================== 4. EL PUEBLO ==============================
	runtime.GC()

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	if _, _, _, ok := componerNivel(engine, presetA, archivoA); !ok {
		println("JUEGO112: LoadStamp devolvio nil")

		return
	}

	estado := d2asset.EstadoDeLosTiles()
	println("JUEGO112: ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
		"tramos=", estado.BloquesTramos, "capacidad=", estado.BloquesCapacidad, "tramoMenor=", estado.TramoMenor,
		"scratch=", estado.Scratch, "scratchPorArchivo=", estado.ScratchPorArchivo,
		"scratchSoltados=", estado.ScratchSoltados, "scratchBytes=", estado.ScratchBytes,
		"picoDelMonton=", estado.PicoDelMonton, "tramosDevueltos=", estado.TramosDevueltos,
		"tramosBytes=", estado.TramosBytes)
	instrumentoDeLasFuentes("despues-de-componer")
	memoria("despues-de-componer")
	semiespacio("despues-de-componer")

	if soltarLecturas {
		tramos := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()

		println("JUEGO112: LECTURAS soltadas despues de componer; tramo=", tramos.BytesDelTramo,
			"lecturas=", tramos.Lecturas, "bytes=", int(tramos.Bytes))
		memoria("despues-de-soltar-las-lecturas-del-mapa")
	}

	// ============================== 5. LO QUE EL CUADRO NO LEE (DC-002.105) ==============================
	soltarLoQueElCuadroNoLee(am)

	// ============================== 6. LO QUE SE SUELTA ANTES DE DIBUJAR ==============================
	switch orden {
	case "tarjeta":
		println("JUEGO112: ORDEN tarjeta -- se suelta el arena ENTERO antes de armar la cache (contra el contrato)")
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("JUEGO112: ARENA DE BLOQUES SOLTADA (antes del renderer) tramos=", tramos, "bytes=", bytes)
		memoria("despues-de-soltar-el-arena")
	case "prestamo":
		println("JUEGO112: ORDEN prestamo -- se suelta SOLO el buffer prestado de lectura (el arena de bloques queda)")
		bytes := d2asset.SoltarPrestamoDeLecturaDeTiles()
		println("JUEGO112: PRESTAMO DE LECTURA SOLTADO bytes=", bytes)
		memoria("despues-de-soltar-el-prestamo")
	default:
		println("JUEGO112: ORDEN contrato -- el arena queda vivo hasta que la cache este armada")
	}

	// ============================== 7. EL RENDERER (CACHE EN INDICES) ==============================
	term, err := d2term.NewTerminal()
	if err != nil {
		println("JUEGO112: NewTerminal FALLO:", err.Error())

		return
	}

	memoria("antes-del-renderer")
	vram("antes-del-renderer")

	opciones := []d2maprenderer.Opcion{
		d2maprenderer.ComoConsola(ancho, alto),
		d2maprenderer.CacheDeTilesPerezosa(),
		d2maprenderer.CacheDeTilesEnIndices(),
	}

	if tramoDePixeles != nil {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramo(tramoDePixeles))
		println("JUEGO112: TRAMO DE LA CACHE en uso (reservado al arranque) bytes=", tramoDePixeles.Bytes())
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 && cacheEnVram {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnBloquesDe(tramoDeLaCache, tramoEnPedazos,
			pvr.VramReservar, "vram", sondaDelTramoDeVram()...))
		println("JUEGO112: TRAMO DE LA CACHE EN BLOQUES DE VRAM techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "pedazoLoDiceLaVram=", pedazoLoDiceElMonton,
			"vramDisponible=", pvr.VramDisponible(),
			"vramServiriaDeUnGolpe=", pvr.VramServible(tramoDeLaCache))
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramoEnPedazos(tramoDeLaCache, tramoEnPedazos,
			sondaDelTramo()...))
		println("JUEGO112: TRAMO DE LA CACHE EN PEDAZOS techo=", tramoDeLaCache, "pedazo=", tramoEnPedazos,
			"pedazoLoDiceElMonton=", pedazoLoDiceElMonton)
	} else if tramoDeLaCache > 0 && cacheEnVram {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramoDe(tramoDeLaCache, pvr.VramReservar, "vram"))
		println("JUEGO112: TRAMO DE LA CACHE EN UN GOLPE DE VRAM bytes=", tramoDeLaCache,
			"vramDisponible=", pvr.VramDisponible())
	} else if tramoDeLaCache > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramo(tramoDeLaCache))
		println("JUEGO112: TRAMO DE LA CACHE pedido TARDE (en el renderer) bytes=", tramoDeLaCache)
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY, opciones...)

	espacio := rendererDelMapa.TamanoDelCuadro()
	println("JUEGO112: renderer listo; cuadro=", espacio.W, espacio.H,
		"cachePerezosa=", rendererDelMapa.CachePerezosa(), "cacheEnIndices=", rendererDelMapa.CacheDeIndices())
	censoDelTramo("despues-del-renderer")
	memoria("despues-del-renderer")
	semiespacio("despues-del-renderer")

	// ============================== 8. EL CUADRO EN PANTALLA ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) DC-002.112"); err != nil {
		println("JUEGO112: el ciclo de cuadros salio con error:", err.Error())
	}

	println("JUEGO112: FIN pico del monton=", picoMonton, "de arena=", d2cMontonArena(),
		"libre=", d2cMontonLibre(), "crcDelMapaSolo=", int(crcDelMapaSolo))
}

// componerNivel (DC-002.109): compone `preset/archivo` en el motor y devuelve el censo de la grilla. El `Stamp`
// y su `.ds1` parseado NO salen de aca: una variable local del que llama lo deja clavado en la pila, y en
// libgodc el barrido de pila es CONSERVADOR (ver DC-002.109/.110).
func componerNivel(engine *d2mapengine.MapEngine, preset, archivo int) (pisos, paredes int, crc uint32, ok bool) {
	t0 := d2cMs()

	stamp := engine.LoadStamp(regionPueblo, preset, archivo)
	if stamp == nil {
		return 0, 0, 0, false
	}

	tamano := stamp.Size()
	enArena := stamp.TilesDelDS1EnArena()
	bloque := stamp.BytesDeLaArenaDelDS1()

	runtime.GC()

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	soltados := stamp.SoltarLosTilesDelDS1()

	pisos, paredes, crc = censo(engine)

	println("JUEGO112: MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crc), "ms=", int(d2cMs()-t0), "ds1enArena=", enArena, "bloque=", bloque,
		"ds1Soltado=", soltados, "w=", tamano.Width, "h=", tamano.Height)

	return pisos, paredes, crc, true
}

// soltarLoQueElCuadroNoLee (DC-002.105): devuelve al monton de C las dos piezas que el CUADRO NO USA -- las
// filas ya parseadas de las tablas del compositor y las dos Source de consola -- porque es de ahi de donde
// tiene que salir la cache de tiles (2.723.840 B).
func soltarLoQueElCuadroNoLee(am *d2asset.AssetManager) int {
	if soltarDespuesDeComponer == "nada" {
		println("JUEGO112: no se suelta nada despues de componer")

		return 0
	}

	total := 0

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		precargarLoQueElCuadroLee(am)
	}

	if soltarDespuesDeComponer == "tablas" || soltarDespuesDeComponer == "todo" {
		memoria("antes-de-soltar-las-tablas")

		res := am.SoltarFilasDeLasTablas(tablasCargadas)

		println("JUEGO112: TABLAS SOLTADAS tablas=", res.Tablas, "tramos=", res.Tramos, "bytes=", res.Bytes,
			"vacias=", len(res.Vacias), "noSoportadas=", len(res.NoSoportadas))

		total += res.Bytes

		memoria("despues-de-soltar-las-tablas")
	}

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		for _, f := range fuentesDeConsola {
			r := f.Soltar(func(tramo uintptr, bytes int) int { return d2cSoltarTramo(tramo, bytes) })

			println("JUEGO112: FUENTE SOLTADA", r.Ruta, "tablas=", r.Tablas.Total,
				"enElMontonDeC=", r.Tablas.Externas, "bytesTablas=", r.BytesTablas,
				"bytesTrabajo=", r.BytesTrabajo, "bytes=", r.Bytes, "cedido=", r.Cedido,
				"yaSoltada=", r.YaSoltada)

			total += r.Bytes
		}

		memoria("despues-de-soltar-las-fuentes")
	}

	println("JUEGO112: SOLTADO despues de componer bytes=", total, "libre=", d2cMontonLibre(),
		"sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()))

	return total
}

// precargarLoQueElCuadroLee pide al AssetManager la paleta del acto y su PL2 de sombreado MIENTRAS las fuentes
// estan abiertas: son los dos archivos que el renderer lee del MPQ al armar la cache.
func precargarLoQueElCuadroLee(am *d2asset.AssetManager) {
	usado := d2cMontonUsado()

	paleta := d2resource.PaletteAct1
	pl2 := paleta[:len(paleta)-len(".dat")] + ".pl2"

	if _, err := am.LoadPalette(paleta); err != nil {
		println("JUEGO112: no se pudo precargar la paleta del acto:", err.Error())
	}

	if _, err := am.LoadPaletteTransform(pl2); err != nil {
		println("JUEGO112: no se pudo precargar el PL2 del acto:", err.Error())
	}

	println("JUEGO112: PRECARGADO lo que el cuadro lee del MPQ", paleta, pl2, "delta=", d2cMontonUsado()-usado)
}

// registrarFuente abre el MPQ con la Source de consola, le inyecta el `free` del shim y la pone PRIMERA en el
// loader del motor (DC-002.96/.99).
func registrarFuente(am *d2asset.AssetManager, ruta string) bool {
	f, err := fuenteconsola.Abrir(ruta)
	if err != nil {
		println("JUEGO112: MPQ-ABRIR FALLO:", ruta, err.Error())

		return false
	}

	f.UsarLiberador(fuenteconsola.Liberador(d2cLibre))
	f.RegistrarEn(am.Loader)

	println("JUEGO112: fuente registrada", ruta)
	fuentesDeConsola = append(fuentesDeConsola, f)

	return true
}

// censoDelTramo (DC-002.104): donde viven los pixeles de la cache.
func censoDelTramo(etapa string) {
	if rendererDelMapa == nil {
		return
	}

	e := rendererDelMapa.CensoDelTramo()
	println("JUEGO112: TRAMO", etapa, "activa=", e.Activa, "origen=", e.Origen, "bytes=", e.Bytes, "usado=", e.Usado,
		"libre=", e.Libre, "imagenes=", e.Imagenes, "derrames=", e.Derrames, "bytesDerramados=", e.BytesDerramados,
		"enPedazos=", e.EnPedazos, "pedazos=", e.Pedazos, "pedazoMenor=", e.PedazoMenor,
		"pedazoMayor=", e.PedazoMayor, "techo=", e.Techo)
}

// instrumentoDeLasFuentes: el censo de la fuente (pedidos, liberados, vivos, prestamo) para el log.
func instrumentoDeLasFuentes(etapa string) {
	for _, f := range fuentesDeConsola {
		e := f.Estado()
		println("JUEGO112: FUENTE", etapa, f.Path(), "abiertos=", e.Abiertos, "enBuffer=", e.LecturasEnBuffer,
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
		println("JUEGO112: TABLA FALLO (fuente):", ruta, err.Error())

		return false
	}

	println("JUEGO112: TABLA fuente", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// cargarTablaPorStream: el camino de HOY (`LoadDataDictionary` por stream del MPQ), para el A/B.
func cargarTablaPorStream(am *d2asset.AssetManager, ruta string) bool {
	usado := d2cMontonUsado()
	tamano, _ := am.TamanoDeArchivo(ruta)

	dict, err := am.LoadDataDictionaryPorStream(ruta)
	if err != nil {
		println("JUEGO112: TABLA FALLO (stream):", ruta, err.Error())

		return false
	}

	if err := am.Records.Load(ruta, dict); err != nil {
		println("JUEGO112: TABLA FALLO (stream, records):", ruta, err.Error())

		return false
	}

	println("JUEGO112: TABLA stream", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// dibujar: EL CUADRO. Primero el mapa por el camino del motor, DESPUES el jugador encima.
//
// LAS TRES MEDIDAS DEL CRITERIO salen de este orden (y cada una se imprime):
//
//	mapa solo          el crc32 del lienzo recien pintado por el renderer. Tiene que ser 1437022778.
//	con el jugador     el crc32 del lienzo con el jugador encima, mas los pixeles de su caja que cambian.
//	mapa restaurado    se devuelve la caja del jugador a los pixeles del mapa y el crc32 tiene que volver a
//	                   1437022778: eso prueba que el jugador no toco un pixel fuera de su caja.
//
// Despues de medir se vuelve a pintar al jugador (es el mismo dibujo, dos veces: la primera para medir contra
// el mapa, la segunda para que el cuadro que se presenta lo tenga).
func dibujar(superficie d2interface.Surface) error {
	if rendererDelMapa == nil {
		return nil
	}

	cuadro++

	if cuadro > cuadros {
		return nil
	}

	println("JUEGO112: CUADRO", cuadro, "ARRANCA libre=", d2cMontonLibre())
	vram("cuadro-antes")

	malla, esPvr := superficie.(*pvr.Surface)
	if !esPvr {
		println("JUEGO112: el Renderer no entrego una Surface del PVR")

		return nil
	}

	t0 := d2cMs()
	rendererDelMapa.Render(superficie)
	msMapa := int(d2cMs() - t0)

	pixeles := malla.Imagen().Pixeles()

	llenos := contarLlenos(pixeles)
	crcMapa := crc32.ChecksumIEEE(pixeles)

	if cuadro == 1 {
		crcDelMapaSolo = crcMapa
	}

	println("JUEGO112: CUADRO", cuadro, "SOLO-MAPA pixelesNoVacios=", llenos, "de", ancho*alto,
		"crc32=", int(crcMapa), "crc32RGB=", int(crcSoloRGB(pixeles)), "hostPixeles=", pixelesHost,
		"hostCrc=", int(crcHost), "ms=", msMapa)

	msJugador := 0

	if jugadorOK && jugador != nil && len(jugador.Cuadros) > 0 {
		cu := &jugador.Cuadros[cuadroDelJugador%len(jugador.Cuadros)]

		anclaX, anclaY := AnclaDelJugador(rendererDelMapa.WorldToScreenF, celdaX, celdaY)
		caja := CajaDe(anclaX, anclaY, cu)

		if cuadro == 1 {
			println("JUEGO112: JUGADOR ancla=", anclaX, anclaY, "caja=", caja.X0, caja.Y0, caja.X1, caja.Y1,
				"tamano=", caja.Ancho(), "x", caja.Alto(), "compuesto=", cu.Ancho, "x", cu.Alto,
				"esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios, "crc32=", int(cu.CRC),
				"capaDibujadas=", cu.CapasDibujadas, "sinGrafico=", cu.CapasSinGrafico,
				"fueraDeCaja=", cu.CapasFueraDeCaja)
			memoria("cuadro-con-el-jugador")
			semiespacio("cuadro-con-el-jugador")
		}

		// 1) se guarda lo que hay DEBAJO de la caja (el mapa solo) y se pinta al jugador
		guardarCaja(pixeles, caja)

		t1 := d2cMs()
		PintarJugador(malla, cu, anclaX, anclaY, palJugador[:])
		msJugador = int(d2cMs() - t1)

		crcConJugador := crc32.ChecksumIEEE(pixeles)
		llenosConJugador := contarLlenos(pixeles)

		// los pixeles de la caja que el jugador cambio: es "el area del personaje" medida sobre el lienzo
		cambiados := pixelesCambiados(pixeles, caja)

		println("JUEGO112: CUADRO", cuadro, "CON-JUGADOR pixelesNoVacios=", llenosConJugador,
			"de", ancho*alto, "crc32=", int(crcConJugador), "crc32RGB=", int(crcSoloRGB(pixeles)),
			"pixelesDelJugador=", cambiados, "delArea=", caja.Ancho()*caja.Alto(),
			"msJugador=", msJugador)

		// 2) se devuelve la caja al mapa y se mide otra vez: tiene que volver al crc32 del mapa solo
		restaurarCaja(pixeles, caja)

		crcRestaurado := crc32.ChecksumIEEE(pixeles)

		println("JUEGO112: CUADRO", cuadro, "MAPA-RESTAURADO crc32=", int(crcRestaurado),
			"igualAlMapaSolo=", crcRestaurado == crcMapa, "mapaSolo=", int(crcMapa),
			"host=", int(crcHost))

		// 3) y se pinta de nuevo, que es lo que se presenta
		t2 := d2cMs()
		PintarJugador(malla, cu, anclaX, anclaY, palJugador[:])
		msJugador += int(d2cMs() - t2)

		if cuadro == 1 {
			println("JUEGO112: CUADRO 1 PRESENTADO con el jugador crc32=", int(crc32.ChecksumIEEE(pixeles)),
				"igualAlDeLaMedida=", crc32.ChecksumIEEE(pixeles) == crcConJugador)
		}
	}

	cache := rendererDelMapa.EstadoDeLaCache()
	println("JUEGO112: CUADRO", cuadro, "cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes,
		"cacheEnIndices=", cache.EnIndices, "msMapa=", msMapa, "msJugador=", msJugador)

	censoDelTramo("despues-del-cuadro")
	vram("despues-del-cuadro")
	memoria("despues-del-cuadro")
	semiespacio("despues-del-cuadro")

	if cuadro == 1 && orden != "tarjeta" {
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("JUEGO112: ARENA DE BLOQUES SOLTADA (despues de la cache) tramos=", tramos, "bytes=", bytes,
			"libre=", d2cMontonLibre())
		memoria("despues-de-soltar-el-arena")
	}

	return nil
}

func actualizar() error { return nil }

// ============================== LOS INSTRUMENTOS DEL LIENZO ==============================

// contarLlenos: los pixeles no negros del lienzo (el mismo numero que imprimen DC-002.103/.106).
func contarLlenos(pixeles []byte) int {
	llenos := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	return llenos
}

// guardarCaja: copia lo que hay en el lienzo dentro de la caja. Es el mapa que el jugador tapa, y es lo que
// permite devolverlo sin volver a pintar el cuadro (y sin pedir un buffer del tamano de la pantalla).
func guardarCaja(pixeles []byte, caja CajaDelJugador) {
	necesario := caja.Ancho() * caja.Alto() * 4

	if cajaMapa == nil || len(cajaMapa) < necesario {
		cajaMapa = make([]byte, necesario)
	}

	n := 0

	for y := caja.Y0; y < caja.Y1; y++ {
		for x := caja.X0; x < caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(cajaMapa[n:n+4], pixeles[j:j+4])

			n += 4
		}
	}
}

// restaurarCaja: devuelve la caja a los pixeles guardados (el mapa).
func restaurarCaja(pixeles []byte, caja CajaDelJugador) {
	n := 0

	for y := caja.Y0; y < caja.Y1; y++ {
		for x := caja.X0; x < caja.X1; x++ {
			j := (y*ancho + x) * 4
			copy(pixeles[j:j+4], cajaMapa[n:n+4])

			n += 4
		}
	}
}

// pixelesCambiados: cuantos pixeles de la caja NO son los del mapa. Es "el area del personaje" contada sobre
// el lienzo, no sobre el compuesto: si el dibujado se corriera un pixel, este numero cambia.
func pixelesCambiados(pixeles []byte, caja CajaDelJugador) int {
	n := 0
	cambiados := 0

	for y := caja.Y0; y < caja.Y1; y++ {
		for x := caja.X0; x < caja.X1; x++ {
			j := (y*ancho + x) * 4

			if pixeles[j] != cajaMapa[n] || pixeles[j+1] != cajaMapa[n+1] ||
				pixeles[j+2] != cajaMapa[n+2] || pixeles[j+3] != cajaMapa[n+3] {
				cambiados++
			}

			n += 4
		}
	}

	return cambiados
}

// crcSoloRGB (DC-002.106): el crc32 del cuadro mirando SOLO los canales de color, sin el alfa. Se arma por
// tandas en un buffer chico: 640x480x3 en el monton no entra.
func crcSoloRGB(pixeles []byte) uint32 {
	var tanda [3 * 1024]byte

	dig := crc32.NewIEEE()
	n := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		tanda[n], tanda[n+1], tanda[n+2] = pixeles[j], pixeles[j+1], pixeles[j+2]
		n += 3

		if n == len(tanda) {
			_, _ = dig.Write(tanda[:n])

			n = 0
		}
	}

	_, _ = dig.Write(tanda[:n])

	return dig.Sum32()
}

// sondaDelTramo (DC-002.106): la funcion que le pregunta al monton cuanto puede servir, o nil para pedir los
// bloques de `tramoEnPedazos` a tamano fijo.
func sondaDelTramo() []func(int) int {
	if !pedazoLoDiceElMonton {
		return nil
	}

	return []func(int) int{func(tope int) int { return int(d2cServible(uint(tope))) }}
}

// sondaDelTramoDeVram (DC-002.108): la misma idea, pero le pregunta al pool de texturas del PVR.
func sondaDelTramoDeVram() []func(int) int {
	if !pedazoLoDiceElMonton {
		return nil
	}

	return []func(int) int{func(tope int) int { return pvr.VramServible(tope) }}
}

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
