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

	// tablasCargadas (DC-002.105): las tablas que se cargaron de verdad, por nombre. Es la lista que se le
	// pasa a `SoltarFilasDeLasTablas` (se suelta lo que se cargo, no "todo lo que haya").
	tablasCargadas []string

	// tablasYFuentesSoltadas (DC-002.111): si las fuentes MPQ y las tablas se devolvieron al monton despues de
	// componer. Con `soltarDespuesDeComponer` distinto de "nada" lo estan, y entonces la fase siguiente tiene
	// que reabrirlas; con "nada" siguen vivas y no hay nada que reabrir (ver `reabrirFuentesYTablas`).
	tablasYFuentesSoltadas bool
)

func memoria(etapa string) {
	usado := d2cMontonUsado()
	if usado > picoMonton {
		picoMonton = usado
	}

	// DC-002.109: el pico de la fase que se esta midiendo (el de la corrida entera es `picoMonton`)
	if usado > picoEnLaFase {
		picoEnLaFase = usado
	}

	// DC-002.111: el pico del monton de C DURANTE la carga de los `.dt1` (la sonda `muestrearElMonton` de
	// d2asset: el POC no alcanza a muestrear adentro del bucle de `ResetMapParaStamp`). Es el mismo numero
	// (`mallinfo().uordblks`), asi que se compara y se informa junto.
	if picoDeLaCarga := d2asset.PicoDelMontonDeC(); picoDeLaCarga > picoMonton {
		picoMonton = picoDeLaCarga

		if picoDeLaCarga > picoEnLaFase {
			picoEnLaFase = picoDeLaCarga
		}
	}

	println("MAPA103: MEMORIA", etapa, "usado=", usado, "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena(), "sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()),
		"pico=", picoMonton)
}

// pruebaDeEscrituraEnVram (DC-002.108): escribe un patron conocido en un bloque de la VRAM y lo vuelve a leer.
// Es el instrumento que separa dos causas cuando la cache puesta en VRAM no da el mismo numero que en el monton:
// ¿la memoria no se puede usar como RAM (se pierden las escrituras) o el dato se corrompe en otra parte?
//
// Va con `pruebaDeVram` y se suelta el bloque en el acto, asi que no le quita VRAM a la cache.
func pruebaDeEscrituraEnVram() {
	const bytes = 348160

	b := pvr.VramReservar(bytes)
	if b == nil {
		println("MAPA103: PRUEBA VRAM no se pudo reservar", bytes, "B")

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

	println("MAPA103: PRUEBA VRAM bytes=", len(b), "malos=", malos, "primeroMal=", primero,
		"leido=", int(b[0]), "esperado=", 3)

	pvr.VramSoltar(b)
}

// vram (DC-002.108): el censo de la VRAM del PVR en cada etapa. `total` es el pool de texturas del chip
// (8 MB), `disponible` lo que el PVR puede repartir AHORA (pvr_mem_available: arena - usado + el top), `pedida`
// lo que este proceso tiene tomado por el paquete pvr (los bloques de la cache) y `lienzo` el framebuffer RGBA
// del cuadro -- que es el que ocupa la VRAM antes que nada y con el que hay que negociar.
//
// ES EL INSTRUMENTO DEL PUNTO 1 DE LA TARJETA: "medir la VRAM disponible con el framebuffer de 640x480 puesto".
// Con esta linea por etapa, la resta sale del log: disponible(al arrancar la pantalla) - disponible(con el
// lienzo) - disponible(con la cache) y no hay que suponer cuanto tomó cada cosa.
func vram(etapa string) {
	c := pvr.FotoDeLaVram()

	println("MAPA103: VRAM", etapa, "total=", c.Total, "disponible=", c.Disponible, "pedida=", c.Pedida,
		"pedidos=", c.Pedidos, "pedidoMayor=", c.PedidoMayor, "lienzo=", c.LienzoDelCuadro)
}

func main() {
	println("MAPA103: arranca DC-002.103 -- el pueblo en pantalla de punta a punta; orden=", orden)
	println("MAPA103: cuadros=", cuadros, "modoTablas=", modoTablas, "soltarLecturas=", soltarLecturas,
		"tramoDeLaCache=", tramoDeLaCache, "tramoEnPedazos=", tramoEnPedazos, "sonda=", pedazoLoDiceElMonton,
		"tramoAlArranque=", tramoAlArranque, "cacheEnVram=", cacheEnVram,
		"soltarDespuesDeComponer=", soltarDespuesDeComponer)
	memoria("al-arrancar")
	vram("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("MAPA103: el video no arranco:", err.Error())

		return
	}

	println("MAPA103: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla())
	memoria("despues-del-video")
	vram("despues-del-video")

	if pruebaDeVram {
		pruebaDeEscrituraEnVram()
	}

	// ============================== 0.b EL TRAMO DE LA CACHE (DC-002.104) ==============================
	// Se reserva ACA y no al crear el renderer: el monton de C (dlmalloc sobre `mm_sbrk`) se fragmenta con el
	// mapa y despues un pedido de 2,7 MB de un solo golpe ya no entra aunque "haya" memoria libre.
	//
	// DC-002.106: en PEDAZOS no se reserva nada aca -- los bloques se piden en el primer cuadro, a medida que
	// las imagenes llegan, y asi el monton los sirve de los huecos que dejo lo que se solto. Reservar temprano
	// era la cura del pedido unico (que necesitaba contiguidad), no la del pedido por bloques.
	if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		println("MAPA103: el tramo de la cache se pide EN PEDAZOS (techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "): no se reserva nada al arranque, se pide en el primer cuadro (DC-002.106)")
		memoria("despues-del-tramo-de-la-cache")
	} else if tramoDeLaCache > 0 && tramoAlArranque {
		tramoDePixeles = d2maprenderer.ReservarTramoDeLaCache(tramoDeLaCache)

		println("MAPA103: TRAMO DE LA CACHE reservado AL ARRANQUE bytes=", tramoDePixeles.Bytes())
		memoria("despues-del-tramo-de-la-cache")
	}

	// ============================== 1. EL MOTOR ==============================
	d2asset.ArrancarComoConsola()
	d2asset.UsarLiberadorDeExternos(func(p uintptr) { d2cLibre(p) })

	// DC-002.110: las celdas de las capas de un `.ds1` se piden en UN bloque del monton de C (mas de 64 KB: en
	// libgodc `gc_external_alloc`) y se devuelven con el liberador del shim. Es lo que deja el semiespacio libre
	// para la vuelta del cambio de nivel (ver knobs.go y d2ds1/arena_110.go).
	if arenaDelDS1 == "encendido" {
		d2ds1.ActivarArenaDelDS1()
		d2ds1.UsarLiberadorDeLaArena(func(p uintptr, bytes int) int { return d2cSoltarBloque(p, bytes) })
	}

	arena := d2ds1.EstadoDeLaArena()
	println("MAPA110: ARENA DEL DS1 palanca=", arenaDelDS1, "encendida=", arena.Encendida,
		"conLiberador=", arena.ConLiberador, "bytesPorCelda=", d2ds1.BytesPorCelda())

	// DC-002.111: el buffer prestado de lectura de los `.dt1`, por archivo (ver knobs.go y
	// `d2asset/arena_soltar_101.go`). Es lo que saca el `.dt1` mas grande del pico del monton de C.
	d2asset.UsarScratchPorArchivo(scratchPorArchivo)

	println("MAPA111: SCRATCH DE LECTURA porArchivo=", d2asset.ScratchPorArchivoActivo(),
		"liberador=", d2asset.HayLiberadorDeExternos())

	// DC-002.111: el tamano de los tramos del arena de bloques lo dice el monton (la misma sonda de DC-002.106
	// que ya usa la cache) y el pico de lo usado durante la carga se mide adentro de la carga.
	d2asset.UsarMedidorDelMontonDeC(func() int { return d2cMontonUsado() })

	if tramoDelArenaPorElMonton {
		d2asset.UsarSondaDelMontonDeC(func(tope int) int { return int(d2cServible(uint(tope))) })
	}

	println("MAPA111: TRAMO DEL ARENA DE BLOQUES porElMonton=", tramoDelArenaPorElMonton,
		"sondaInstalada=", d2asset.HaySondaDelMontonDeC())

	// DC-002.111: al tirar un nivel, los tramos del arena de bloques vuelven al monton de C (ver knobs.go).
	d2asset.UsarSoltarTramosAlReiniciar(soltarTramosAlCambiarDeNivel)

	println("MAPA111: TRAMOS AL CAMBIAR DE NIVEL seDevuelven=", d2asset.SoltarTramosAlReiniciarActivo())

	// DC-002.105: las filas de las tablas no son rebanadas (el tablero de un mapa se lee en C) y las tablas del
	// MPQ se devuelven con el `free` del shim. Los dos liberadores son los del shim, no un `free` suelto desde
	// Go (que en el semiespacio del recolector corrompe el monton).
	d2asset.UsarLiberadorDeFilas(&d2records.LiberadorDeFilas{
		SoltarMapa:  func(mapa unsafe.Pointer) int { return d2cSoltarFilas(uintptr(mapa)) },
		SoltarTramo: func(bloque unsafe.Pointer, bytes int) int { return d2cSoltarTramo(uintptr(bloque), bytes) },
	})

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

	// DC-002.109: el ciclo de las tres fases reabre las fuentes y las tablas sobre ESTE AssetManager
	gestorDeAssets = am

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
		var ok bool

		if modoTablas == "stream" {
			ok = cargarTablaPorStream(am, tablasDelCompositor[i])
		} else {
			ok = cargarTablaPorLaFuente(am, tablasDelCompositor[i])
		}

		if ok {
			cargadas++
			// DC-002.105: la lista de lo que se cargo es lo que despues se suelta (por nombre, no "todo")
			tablasCargadas = append(tablasCargadas, tablasDelCompositor[i])
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

	// DC-002.109: el ciclo de las tres fases compone los niveles siguientes sobre ESTE motor
	motorDelMapa = engine

	// DC-002.109: la composicion va por `componerNivel` para que el `Stamp` (y con el el `.ds1` parseado, que
	// vive en el SEMIESPACIO del recolector) NO se quede pegado en la pila de `main`: es el objeto que la
	// TRANSICION necesita tener libre para parsear el `.ds1` nuevo.
	if _, _, _, ok := componerNivel(engine, presetA, archivoA, ""); !ok {
		println("MAPA103: LoadStamp devolvio nil")

		return
	}

	estado := d2asset.EstadoDeLosTiles()
	println("MAPA103: ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
		"tramos=", estado.BloquesTramos, "capacidad=", estado.BloquesCapacidad, "tramoMenor=", estado.TramoMenor,
		"scratch=", estado.Scratch, "scratchPorArchivo=", estado.ScratchPorArchivo,
		"scratchSoltados=", estado.ScratchSoltados, "scratchBytes=", estado.ScratchBytes,
		"picoDelMonton=", estado.PicoDelMonton, "tramosDevueltos=", estado.TramosDevueltos,
		"tramosBytes=", estado.TramosBytes)
	instrumentoDeLasFuentes("despues-de-componer")
	memoria("despues-de-componer")

	if soltarLecturas {
		tramos := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()

		println("MAPA103: LECTURAS soltadas despues de componer; tramo=", tramos.BytesDelTramo,
			"lecturas=", tramos.Lecturas, "bytes=", int(tramos.Bytes))
		memoria("despues-de-soltar-las-lecturas-del-mapa")
	}

	// ============================== 3.b LO QUE EL CUADRO NO LEE (DC-002.105) ==============================
	// Con el pueblo compuesto, estas dos piezas ya no las mira nadie: las filas de las tablas (el compositor las
	// leyo para armar las casillas y el renderer usa la copia del levelType) y las dos fuentes MPQ (el cuadro no
	// lee un byte de ahi). Se devuelven al monton de C AHORA, que es de donde tiene que salir la cache.
	soltarLoQueElCuadroNoLee(am)

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
	vram("antes-del-renderer")

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
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 && cacheEnVram {
		// DC-002.108: el tramo en BLOQUES, pero de la VRAM DEL PVR. Es el mismo mecanismo de DC-002.106 con el
		// origen cambiado: el monton de C no pide ni un byte de la cache y la pared de la contiguidad
		// desaparece. La sonda es la de la VRAM (`pvr_mem_malloc`/`free`), no la del monton de C.
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnBloquesDe(tramoDeLaCache, tramoEnPedazos,
			pvr.VramReservar, "vram", sondaDelTramoDeVram()...))
		println("MAPA103: TRAMO DE LA CACHE EN BLOQUES DE VRAM techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "pedazoLoDiceLaVram=", pedazoLoDiceElMonton,
			"vramDisponible=", pvr.VramDisponible(),
			"vramServiriaDeUnGolpe=", pvr.VramServible(tramoDeLaCache))
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		// DC-002.106: el tramo en BLOQUES -- cada bloque se pide cuando una imagen no entra en los que hay, y
		// el monton lo sirve de sus huecos (el pedido unico necesitaba contiguidad y no la hay).
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramoEnPedazos(tramoDeLaCache, tramoEnPedazos,
			sondaDelTramo()...))
		println("MAPA103: TRAMO DE LA CACHE EN PEDAZOS techo=", tramoDeLaCache, "pedazo=", tramoEnPedazos,
			"pedazoLoDiceElMonton=", pedazoLoDiceElMonton)
	} else if tramoDeLaCache > 0 && cacheEnVram {
		// DC-002.108: el pedido ENTERO de la cache, de un solo golpe, a la VRAM. Es el mismo pedido que en el
		// monton de C muere por CONTIGUIDAD (DC-002.104/.105) y aca tiene que entrar o devolver nil.
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramoDe(tramoDeLaCache, pvr.VramReservar, "vram"))
		println("MAPA103: TRAMO DE LA CACHE EN UN GOLPE DE VRAM bytes=", tramoDeLaCache,
			"vramDisponible=", pvr.VramDisponible(),
			"vramServiriaDeUnGolpe=", pvr.VramServible(tramoDeLaCache))
	} else if tramoDeLaCache > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramo(tramoDeLaCache))
		println("MAPA103: TRAMO DE LA CACHE pedido TARDE (en el renderer) bytes=", tramoDeLaCache)
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY, opciones...)

	// DC-002.109: el tramo que se REUSA al cambiar de nivel se deja limpio con el `memset` de la VRAM (un bucle
	// de Go sobre VRAM cuesta mas para lo mismo). Solo tiene sentido si los bloques viven ahi.
	if tramoDeLaCache > 0 && cacheEnVram && tramoDePixeles == nil {
		rendererDelMapa.PonerEnCeroLosBloquesDelTramoCon(pvr.VramPonerEnCero)
	}

	d2maprenderer.DiagnosticoDeLasImagenes = diagnosticoDeLasImagenes

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

// soltarLoQueElCuadroNoLee (DC-002.105): devuelve al monton de C las dos piezas que el CUADRO NO USA -- las
// filas ya parseadas de las tablas del compositor y las dos Source de consola (las tablas del MPQ y la memoria
// de trabajo de la lectura) -- en el orden que dice el knob `soltarDespuesDeComponer`. Devuelve los bytes.
//
// POR QUE ACA Y NO DESPUES DEL PRIMER CUADRO: la cache de tiles se arma en el primer Render y pide 2.723.840 B
// (DC-002.103/.104); si se suelta despues, esos bytes ya no estan para la cache, que es para lo que se juntan.
//
// LO QUE EL CUADRO SI LEE, y por eso se precarga: cuando el renderer arma la cache lee del MPQ la paleta del acto
// (`pal.dat`, cacheada por el AssetManager) y su tabla de sombreado (`pal.pl2`, ahora por el cache de
// transformaciones, ver `d2maprenderer/lighting.go`). Las dos se piden ANTES de cerrar las fuentes.
func soltarLoQueElCuadroNoLee(am *d2asset.AssetManager) int {
	if soltarDespuesDeComponer == "nada" {
		println("MAPA103: no se suelta nada despues de componer (el camino de DC-002.103)")
		return 0
	}

	// DC-002.111: en el CICLO no se sueltan ni las fuentes ni las tablas.
	//
	// POR QUE. Soltarlas obliga a la fase siguiente a REABRIRLAS (es lo que media DC-002.109), y cada reapertura
	// deja una copia mas de sus estructuras de Go en el semiespacio que la recoleccion no puede tirar (el barrido
	// de pila de libgodc es conservador). Medido con la reapertura puesta: la base de la VUELTA quedaba 409.096 B
	// arriba de la de la TRANSICION (1.309.520 contra 934.464 B) y la fase pedia 1.824.256 B de 1.835.008 y no
	// entraba. Sin reapertura la base queda en 1.022.784 B, la fase pide 1.600.984 B (87 %) y entra con el
	// recolector de DC-002.109 sin tocarle el umbral.
	//
	// Lo que se paga en el monton de C son 679.440 B que quedan tomados toda la corrida, y con las palancas de
	// esta tarjeta (scratch por archivo, tramo por el monton, tramos devueltos al cambiar de nivel) el pico del
	// ciclo es 10.199.016 B de 10.227.936: entran. Con el ciclo apagado la corrida de referencia no cambia --
	// sigue soltando todo despues del unico cuadro, como en DC-002.106.
	if !soltarEnElCiclo && cicloDeFases != "apagado" {
		println("MAPA111: en el CICLO no se sueltan las fuentes ni las tablas: reabrirlas en cada fase deja " +
			"copias vivas en el semiespacio y la VUELTA no tiene ese lugar (ver DC-002.111)")
		return 0
	}

	total := 0

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		precargarLoQueElCuadroLee(am)
	}

	if soltarDespuesDeComponer == "tablas" || soltarDespuesDeComponer == "todo" {
		memoria("antes-de-soltar-las-tablas")

		res := am.SoltarFilasDeLasTablas(tablasCargadas)

		println("MAPA103: TABLAS SOLTADAS tablas=", res.Tablas, "tramos=", res.Tramos, "bytes=", res.Bytes,
			"vacias=", len(res.Vacias), "noSoportadas=", len(res.NoSoportadas))

		total += res.Bytes

		memoria("despues-de-soltar-las-tablas")
	}

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		for _, f := range fuentesDeConsola {
			r := f.Soltar(func(tramo uintptr, bytes int) int { return d2cSoltarTramo(tramo, bytes) })

			println("MAPA103: FUENTE SOLTADA", r.Ruta, "tablas=", r.Tablas.Total,
				"enElMontonDeC=", r.Tablas.Externas, "bytesTablas=", r.BytesTablas,
				"bytesTrabajo=", r.BytesTrabajo, "bytes=", r.Bytes, "cedido=", r.Cedido,
				"yaSoltada=", r.YaSoltada)

			total += r.Bytes
		}

		memoria("despues-de-soltar-las-fuentes")
	}

	println("MAPA103: SOLTADO despues de componer bytes=", total, "libre=", d2cMontonLibre(),
		"sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()))

	// DC-002.111: lo que se solto hay que volver a abrirlo en la fase siguiente (y lo que no, no)
	if total > 0 {
		tablasYFuentesSoltadas = true
	}

	return total
}

// precargarLoQueElCuadroLee pide al AssetManager la paleta del acto y su PL2 de sombreado MIENTRAS las fuentes
// estan abiertas: son los dos archivos que el renderer lee del MPQ al armar la cache. Despues de `Soltar` el MPQ
// no se puede leer, asi que sin esta precarga el cuadro saldria con sombreado lineal (y el crc32 no seria el del
// host) o sin paleta.
func precargarLoQueElCuadroLee(am *d2asset.AssetManager) {
	usado := d2cMontonUsado()

	paleta := d2resource.PaletteAct1
	pl2 := paleta[:len(paleta)-len(".dat")] + ".pl2"

	if _, err := am.LoadPalette(paleta); err != nil {
		println("MAPA103: no se pudo precargar la paleta del acto:", err.Error())
	}

	if _, err := am.LoadPaletteTransform(pl2); err != nil {
		println("MAPA103: no se pudo precargar el PL2 del acto:", err.Error())
	}

	println("MAPA103: PRECARGADO lo que el cuadro lee del MPQ", paleta, pl2, "delta=", d2cMontonUsado()-usado)
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
	println("MAPA103: TRAMO", etapa, "activa=", e.Activa, "origen=", e.Origen, "bytes=", e.Bytes, "usado=", e.Usado,
		"libre=", e.Libre, "imagenes=", e.Imagenes, "derrames=", e.Derrames, "bytesDerramados=", e.BytesDerramados,
		"enPedazos=", e.EnPedazos, "pedazos=", e.Pedazos, "pedazoMenor=", e.PedazoMenor,
		"pedazoMayor=", e.PedazoMayor, "techo=", e.Techo)
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
	vram("cuadro-antes")

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

	// DC-002.106: ademas del crc32 del RGBA, el del RGB solo. Si los dos cuadros (consola y host) coinciden en
	// este y no en el otro, lo unico que difiere es el canal ALFA -- que es lo que paso hasta que se midio.
	crudoRGB := crcSoloRGB(pixeles)

	cache := rendererDelMapa.EstadoDeLaCache()
	println("MAPA103: CUADRO", cuadro, "pixelesNoVacios=", llenos, "de", ancho*alto, "crc32=", int(crudo),
		"crc32RGB=", int(crudoRGB),
		"hostPixeles=", pixelesHost, "hostCrc=", int(crcHost), "ms=", ms,
		"cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes, "cacheEnIndices=", cache.EnIndices)
	censoDelTramo("despues-del-cuadro")
	huella, imagenes, bytesDeIndices := rendererDelMapa.HuellaDeLaCache()
	println("MAPA103: HUELLA DE LA CACHE fnv=", int(huella), "imagenes=", imagenes, "bytes=", bytesDeIndices)
	vram("despues-del-cuadro")
	memoria("despues-del-cuadro")

	if cuadro == 1 && orden != "tarjeta" {
		// la cache del cuadro esta armada: el arena de bloques ya no hace falta (contrato de DC-002.101)
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("MAPA103: ARENA DE BLOQUES SOLTADA (despues de la cache) tramos=", tramos, "bytes=", bytes,
			"libre=", d2cMontonLibre())
		memoria("despues-de-soltar-el-arena")
	}

	// DC-002.109: el ciclo de las tres fases (ENTRAR / TRANSICION / VOLVER) corre en el cuadro 2, con el arena de
	// bloques ya soltado (el estado que dejo medido DC-002.106).
	//
	// POR QUE EN EL 2 Y NO EN EL 1. El cuadro se PRESENTA cuando `dibujar` vuelve, y el ciclo se lleva ~60 s
	// (reabrir fuentes, componer, dibujar): colgado del cuadro 1 la pantalla se queda en negro todo el rato
	// (medido: las capturas de Flycast salian negras). Con el 2, el pueblo queda presentado y el cuadro de la
	// TRANSICION se presenta al volver de la fase.
	//
	// Con la palanca en `apagado` (el default de knobs.go) no se ejecuta una sola linea y la corrida de
	// referencia de DC-002.106/.108 queda igual.
	if cuadro == 2 && cicloDeFases != "apagado" {
		guardarCuadro1(crudo, llenos)
		cicloDeNiveles(superficie)
	}

	return nil
}

func actualizar() error { return nil }

// crcSoloRGB (DC-002.106): el crc32 del cuadro mirando SOLO los canales de color, sin el alfa.
//
// POR QUE ES UN INSTRUMENTO Y NO UN CAPRICHO. El crc32 del RGBA es la comparacion contra el host, pero cuando
// los dos numeros no coinciden no dice QUE cambio. Con este, si el RGB coincide y el RGBA no, lo unico que
// difiere es el canal ALFA -- que fue exactamente lo que paso en esta tarjeta: 565415037 contra 1437022778 con
// el RGB identico (2507235743 en los dos lados) y la diferencia en los 4 pixeles que ningun tile pinta (el
// lienzo de la consola se rellenaba opaco y el del host arranca transparente). Se arma por tandas en un buffer
// chico: 640x480x3 en el monton no entra.
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
//
// POR QUE ES UNA PALANCA Y NO EL UNICO CAMINO. Con el tamano fijo se MIDIO donde se corta (Flycast: el ultimo
// bloque de 286.720 B con 159.744 de break y las migas libres sin poderse coser). Con la sonda el mismo monton
// sirve lo que puede y la cache entra igual. Tener las dos deja el A/B en el mismo binario.
func sondaDelTramo() []func(int) int {
	if !pedazoLoDiceElMonton {
		return nil
	}

	return []func(int) int{func(tope int) int { return int(d2cServible(uint(tope))) }}
}

// sondaDelTramoDeVram (DC-002.108): la misma idea que `sondaDelTramo`, pero le pregunta al pool de texturas del
// PVR cuanto puede servir. Es una palanca aparte y no un `if` adentro de la otra a proposito: son dos montones
// distintos (dlmalloc sobre `mm_sbrk` en RAM contra el dlmalloc recortado de la VRAM) y el log tiene que decir
// cual de los dos decidio el tamano del bloque.
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
