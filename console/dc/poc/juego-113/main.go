//go:build dreamcast

// DC-002.113 -- EL CONTROL MUEVE AL JUGADOR: EL MAPLE EN EL POC Y LA CAMARA SIGUIENDOLO.
//
// QUE MIDE. El control ya existe y esta probado en el host (DC-002.71: 9 botones, 12 pruebas, 8 mutantes) y el
// pueblo con el jugador encima ya se dibuja (DC-002.112). Lo que faltaba es lo que hace que sea un juego: que el
// boton MUEVA al personaje por el mapa y que la camara lo siga. Esta corrida hace las dos cosas en el mismo
// cuadro del PVR y las mide:
//
//  1. LEE EL MAPLE CADA CUADRO por `d2interface.InputService` (el backend de la consola, `d2core/d2input/maple`):
//     del bit del control a la tecla del motor, y de ahi a los ejes de la pantalla. El POC no pregunta por el bus
//     maple ni por Ebitengine.
//  2. MUEVE al jugador con la velocidad del exe (`personaje.go`: 9,375 subtiles/s caminando = 1,875 celdas/s) y
//     el reloj de 25 Hz del motor, en las 8 direcciones de la cruceta, con la animacion de caminar del pueblo
//     (`AMTWhth.COF`, 8 cuadros por direccion a 40,00 ms).
//  3. LA CAMARA SIGUE al jugador con la cuenta del motor (`WorldToOrtho` + `Camera.MoveTo`, lo mismo que
//     `d2gamescreen` con el heroe). Con `CAMARA=fija` la camara se queda y el jugador se VE cruzar la pantalla.
//  4. MIDE, por tramo y en total: celdas por segundo con el boton mantenido, cuadros pintados y ms por cuadro.
//  5. REPITE el cuadro del origen al final (con la camara en la misma posicion) y compara los crc32: el cuadro
//     tiene que ser el MISMO numero. Eso es la repetibilidad del criterio.
//
// EL CUADRO 1 SIGUE SIENDO EL DE DC-002.112: el jugador quieto (NU dir 0) sobre el pueblo y la camara en la celda
// de arranque (28,20) dan el mismo ancla (320,283) y el mismo crc32 (1363871674 con el jugador, 1437022778 el
// mapa solo). Si eso cambiara, el camino del mapa se habria movido, y esta tarjeta no lo toca.
//
// LO QUE NO ENTRA (la tarjeta lo dice): dos jugadores (DC-002.114) y tocar `d2mapentity.Player` (arrastra 2,11 MB):
// el jugador es el Composite liviano de DC-002.112.
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2input/maple"
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

//extern d2c_gc_percent
func d2cGcPercent(nuevo int) int

func init() { d2cSerialInit() }

const (
	// El disco: los TRES MPQ del juego que hacen falta (el pueblo y las paletas en d2data, las tablas en
	// patch_D2, el COF y los `.dcc` de la amazona en d2char).
	rutaParche  = "/cd/JUEGO/PATCH_D2.MPQ"
	rutaDatos   = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChar = "/cd/JUEGO/D2CHAR.MPQ"

	// la paleta de las unidades (personajes): 768 B, B,G,R.
	palUnidadesMPQ = "data/global/palette/Units/pal.dat"

	regionPueblo = d2enum.RegionAct1Town
	presetA      = 1
	archivoA     = 0

	// la pantalla de la consola y la celda del pueblo del arranque: 640x480 centrado en la celda (28,20), que es
	// la que dibujo DC-002.112 (ancla del jugador en 320,283).
	ancho, alto = 640, 480
	celdaX      = 28.0
	celdaY      = 20.0

	// los dos numeros del cuadro de referencia del HOST (DC-002.94/.101/.106): el mapa solo.
	pixelesHost = 307196
	crcHost     = 1437022778

	// DC-002.112: el cuadro del pueblo CON el jugador quieto (NU dir 0) encima.
	crcConJugador112 = 1363871674

	// pausaEntreTramos: lo que se suelta la cruceta entre tramo y tramo del guion inyectado (el teclado virtual
	// de la corrida del criterio suelta la tecla y espera lo mismo: `PAUSA=` de run-flycast-113.sh).
	pausaEntreTramos = 1.0
)

// tablasDelCompositor: las dos primeras tablas del compositor (LvlTypes + LvlPrest), las que pide de verdad.
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

	// tablasCargadas: las tablas que se cargaron de verdad, por nombre.
	tablasCargadas []string

	// ============================== EL JUGADOR (DC-002.112) Y SU MOVIMIENTO (DC-002.113) ==============================
	//
	// jugadorQuieto es el ciclo NU de la direccion 0 (el cuadro de DC-002.112) y jugadoresCaminando son los ciclos
	// de TW, uno por direccion de la cruceta. palJugador es la paleta de las unidades en RGBA.
	jugadorQuieto         *CicloDelJugador
	jugadoresCaminando    map[int]*CicloDelJugador
	direccionesCompuestas []int
	palJugador            [256][4]uint8
	jugadorOK             bool

	// EL CONTROL, por el contrato del motor (DC-002.71): el POC pregunta por teclas, no por bits del maple.
	servicio                          = maple.NuevoServicio(ancho, alto)
	entrada  d2interface.InputService = servicio

	// mov es el jugador que camina (posicion, direccion, reloj de la animacion, ticks de 25 Hz).
	mov *movimiento

	// cajaMapa: lo que habia en el lienzo debajo del jugador, para devolverlo y poder medir el mapa sin el.
	cajaMapa []byte

	// crcDelMapaSolo: el crc32 del cuadro antes de dibujar al jugador (el numero que tiene que seguir siendo
	// 1437022778 en la celda de arranque).
	crcDelMapaSolo uint32

	// ---- el reloj y las medidas del criterio ----
	msAnterior   int64   // el reloj de KOS (d2c_ms) del cuadro anterior
	primerCuadro bool    // el cuadro 1 no cobra paso (el arranque del video no es tiempo de juego)
	reloj        float64 // segundos desde el cuadro 1 (el reloj del guion inyectado)
	msDelCuadro  float64 // lo que duro el cuadro anterior, en ms (para el promedio)
	msCuadros    int     // cuantos cuadros entraron en ese promedio

	// el cuadro 1, guardado para la prueba de repetibilidad del final
	crcCuadro1Mapa    uint32
	crcCuadro1Jugador uint32
	anclaCuadro1X     int
	anclaCuadro1Y     int

	// el cuadro del ORIGEN en el cierre (cuadros+1), contra el que se compara el siguiente (cuadros+2)
	crcOrigenMapa    uint32
	crcOrigenJugador uint32
	llenosCuadro1    int
	llenosOrigen     int

	// el estado al que llego el guion, para volver a el despues de la prueba de repetibilidad
	celdaFinalX, celdaFinalY float64
	dirFinal                 int
	msAnimFinal              float64
	caminandoFinal           bool

	// el tramo de caminata que se esta midiendo ahora (una direccion mantenida)
	tramo      *tramoMedido
	tramos     int
	reportados int
)

// tramoMedido: un tramo del guion, medido con el reloj de la consola.
type tramoMedido struct {
	Nombre    string
	Dir       int
	Segundos  float64 // segundos con la cruceta apretada (reloj de la consola)
	Subtiles  float64 // subtiles caminados
	Cuadros   int
	CuadroIni int
	CeldaIniX float64
	CeldaIniY float64
}

func memoria(etapa string) {
	usado := d2cMontonUsado()
	if usado > picoMonton {
		picoMonton = usado
	}

	// DC-002.111: el pico del monton de C DURANTE la carga de los `.dt1` (la sonda de d2asset).
	if picoDeLaCarga := d2asset.PicoDelMontonDeC(); picoDeLaCarga > picoMonton {
		picoMonton = picoDeLaCarga
	}

	println("JUEGO113: MEMORIA", etapa, "usado=", usado, "libre=", d2cMontonLibre(),
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

	println("JUEGO113: SEMIESPACIO", etapa, "usado=", usado, "total=", total, "pct=", pct,
		"colecciones=", int(d2cGcColecciones()))
}

// pruebaDeEscrituraEnVram (DC-002.108): escribe un patron conocido en un bloque de la VRAM y lo vuelve a leer.
func pruebaDeEscrituraEnVram() {
	const bytes = 348160

	b := pvr.VramReservar(bytes)
	if b == nil {
		println("JUEGO113: PRUEBA VRAM no se pudo reservar", bytes, "B")

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

	println("JUEGO113: PRUEBA VRAM bytes=", len(b), "malos=", malos, "primeroMal=", primero,
		"leido=", int(b[0]), "esperado=", 3)

	pvr.VramSoltar(b)
}

// vram (DC-002.108): el censo de la VRAM del PVR en cada etapa.
func vram(etapa string) {
	c := pvr.FotoDeLaVram()

	println("JUEGO113: VRAM", etapa, "total=", c.Total, "disponible=", c.Disponible, "pedida=", c.Pedida,
		"pedidos=", c.Pedidos, "pedidoMayor=", c.PedidoMayor, "lienzo=", c.LienzoDelCuadro)
}

func main() {
	println("JUEGO113: arranca DC-002.113 -- el control mueve al jugador y la camara lo sigue")
	println("JUEGO113: cuadros=", cuadros, "jugador=", jugadorKnob, "camara=", camara, "modoCaminando=",
		modoCaminando, "msCaminando=", msCaminando, "dirsCaminando=", dirsCaminando, "inyectarGuion=",
		inyectarGuion, "tramoDeLaCache=", tramoDeLaCache, "cacheEnVram=", cacheEnVram, "orden=", orden)

	memoria("al-arrancar")
	semiespacio("al-arrancar")
	vram("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	// El PVR necesita el monton de C casi vacio para arrancar (DC-002.50): es el mismo orden de DC-002.103.
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("JUEGO113: el video no arranco:", err.Error())

		return
	}

	println("JUEGO113: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla())
	memoria("despues-del-video")
	vram("despues-del-video")

	if pruebaDeVram {
		pruebaDeEscrituraEnVram()
	}

	if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		println("JUEGO113: el tramo de la cache se pide EN PEDAZOS (techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "): no se reserva nada al arranque (DC-002.106)")
	} else if tramoDeLaCache > 0 && tramoAlArranque {
		tramoDePixeles = d2maprenderer.ReservarTramoDeLaCache(tramoDeLaCache)
		println("JUEGO113: TRAMO DE LA CACHE reservado AL ARRANQUE bytes=", tramoDePixeles.Bytes())
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
	println("JUEGO113: ARENA DEL DS1 palanca=", arenaDelDS1, "encendida=", arena.Encendida,
		"conLiberador=", arena.ConLiberador, "bytesPorCelda=", d2ds1.BytesPorCelda())

	d2asset.UsarScratchPorArchivo(scratchPorArchivo)
	println("JUEGO113: SCRATCH DE LECTURA porArchivo=", d2asset.ScratchPorArchivoActivo(),
		"liberador=", d2asset.HayLiberadorDeExternos())

	d2asset.UsarMedidorDelMontonDeC(func() int { return d2cMontonUsado() })

	if tramoDelArenaPorElMonton {
		d2asset.UsarSondaDelMontonDeC(func(tope int) int { return int(d2cServible(uint(tope))) })
	}

	println("JUEGO113: TRAMO DEL ARENA DE BLOQUES porElMonton=", tramoDelArenaPorElMonton,
		"sondaInstalada=", d2asset.HaySondaDelMontonDeC())

	d2asset.UsarSoltarTramosAlReiniciar(soltarTramosAlCambiarDeNivel)
	println("JUEGO113: TRAMOS AL CAMBIAR DE NIVEL seDevuelven=", d2asset.SoltarTramosAlReiniciarActivo())

	d2asset.UsarLiberadorDeFilas(&d2records.LiberadorDeFilas{
		SoltarMapa:  func(mapa unsafe.Pointer) int { return d2cSoltarFilas(uintptr(mapa)) },
		SoltarTramo: func(bloque unsafe.Pointer, bytes int) int { return d2cSoltarTramo(uintptr(bloque), bytes) },
	})

	if modoTablas != "stream" {
		d2asset.ActivarTablasPorLaFuente()
	}

	println("JUEGO113: tablasPorLaFuente=", d2asset.TablasPorLaFuenteActivas(),
		"liberadorDeExternos=", d2asset.HayLiberadorDeExternos())
	memoria("despues-de-arrancar-como-consola")

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		println("JUEGO113: NewAssetManager FALLO:", err.Error())

		return
	}

	println("JUEGO113: assetmanager listo")
	memoria("despues-del-assetmanager")

	for _, ruta := range []string{rutaDatos, rutaParche} { // el parche queda PRIMERO: se registra al final
		if !registrarFuente(am, ruta) {
			return
		}
	}

	println("JUEGO113: fuentes en el loader=", len(am.Sources))
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

	println("JUEGO113: tablas cargadas=", cargadas, "de", tablasQueSeCargan)
	instrumentoDeLasFuentes("despues-de-las-tablas")
	memoria("despues-de-las-tablas")

	if soltarLecturas {
		tramosAntes := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()
		tramosDespues := d2asset.EstadoDeLosTramos()

		println("JUEGO113: LECTURAS soltadas; tramoAntes=", tramosAntes.BytesDelTramo, "pedidos=", tramosAntes.Pedidos,
			"reusos=", tramosAntes.Reusos, "liberados=", tramosDespues.Liberados)
		memoria("despues-de-soltar-las-lecturas-de-las-tablas")
	}

	// ============================== 3. EL JUGADOR (DC-002.112) Y SUS CICLOS DE CAMINAR (DC-002.113) ==============================
	// VA ACA, Y EL LUGAR ES LA MITAD DEL TRABAJO: antes de componer el pueblo y antes de soltar las fuentes MPQ
	// (los `.cof`/`.dcc` y la paleta salen de los MPQ), y antes del pico de componer el pueblo (la decodificacion
	// del `.dcc` pide su arena al monton de C y la devuelve en el acto).
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
		println("JUEGO113: JUGADOR apagado (la corrida de referencia del pueblo solo)")
	}

	// ============================== 4. EL PUEBLO ==============================
	// EL UMBRAL DEL RECOLECTOR, JUSTO ACA (DC-002.113). El jugador de esta tarjeta deja 8 direcciones de TW
	// vivas (148.720 B de indices) y el conjunto vivo queda por encima del 75 % del semiespacio que dispara el
	// recolector con el default de DC-002.112: a partir de ahi recolecta en CADA asignacion y la composicion
	// del pueblo no avanza (medido: 2.333 recolecciones clavadas despues de leer el `.ds1`). Se sube el umbral
	// ANTES de componer y se dice con que numero quedo.
	anterior := d2cGcPercent(umbralDelRecolector)
	espacioGc := int(d2cGcTotal())
	umbralBytes := 0

	if umbralDelRecolector == 100 { // la escala de gc_heap.c: 100 quiere decir "al 75 %"
		umbralBytes = espacioGc * 3 / 4
	} else {
		umbralBytes = espacioGc * umbralDelRecolector / 100
	}

	println("JUEGO113: UMBRAL DEL RECOLECTOR antes=", anterior, "ahora=", umbralDelRecolector,
		"semiespacio=", espacioGc, "umbralBytes=", umbralBytes, "usado=", int(d2cGcUsado()),
		"colecciones=", int(d2cGcColecciones()))

	runtime.GC()

	println("JUEGO113: RECOLECCION A PEDIDO usado=", int(d2cGcUsado()), "colecciones=", int(d2cGcColecciones()))
	memoria("despues-de-recolectar-a-pedido")

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	if _, _, _, ok := componerNivel(engine, presetA, archivoA); !ok {
		println("JUEGO113: LoadStamp devolvio nil")

		return
	}

	// EL CONJUNTO VIVO, MEDIDO (DC-002.113). Componer el pueblo deja mucha basura; una recoleccion A PEDIDO
	// deja el semiespacio en lo que de verdad esta vivo. Ese numero es el que dice si el POC entra o no: el
	// espacio copia a un semiespacio del mismo tamano (1.835.008 B), asi que el vivo tiene que quedarle bien
	// abajo o la copia se queda sin lugar ("GC to-space overflow", medido en esta tarjeta con 95 % de umbral).
	runtime.GC()
	semiespacio("despues-de-componer-y-recolectar")

	estado := d2asset.EstadoDeLosTiles()
	println("JUEGO113: ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
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

		println("JUEGO113: LECTURAS soltadas despues de componer; tramo=", tramos.BytesDelTramo,
			"lecturas=", tramos.Lecturas, "bytes=", int(tramos.Bytes))
		memoria("despues-de-soltar-las-lecturas-del-mapa")
	}

	// ============================== 5. LO QUE EL CUADRO NO LEE (DC-002.105) ==============================
	soltarLoQueElCuadroNoLee(am)

	// ============================== 6. LO QUE SE SUELTA ANTES DE DIBUJAR ==============================
	switch orden {
	case "tarjeta":
		println("JUEGO113: ORDEN tarjeta -- se suelta el arena ENTERO antes de armar la cache (contra el contrato)")
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("JUEGO113: ARENA DE BLOQUES SOLTADA (antes del renderer) tramos=", tramos, "bytes=", bytes)
		memoria("despues-de-soltar-el-arena")
	case "prestamo":
		println("JUEGO113: ORDEN prestamo -- se suelta SOLO el buffer prestado de lectura (el arena de bloques queda)")
		bytes := d2asset.SoltarPrestamoDeLecturaDeTiles()
		println("JUEGO113: PRESTAMO DE LECTURA SOLTADO bytes=", bytes)
		memoria("despues-de-soltar-el-prestamo")
	default:
		println("JUEGO113: ORDEN contrato -- el arena queda vivo hasta que la cache este armada")
	}

	// ============================== 7. EL RENDERER (CACHE EN INDICES) ==============================
	term, err := d2term.NewTerminal()
	if err != nil {
		println("JUEGO113: NewTerminal FALLO:", err.Error())

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
		println("JUEGO113: TRAMO DE LA CACHE en uso (reservado al arranque) bytes=", tramoDePixeles.Bytes())
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 && cacheEnVram {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnBloquesDe(tramoDeLaCache, tramoEnPedazos,
			pvr.VramReservar, "vram", sondaDelTramoDeVram()...))
		println("JUEGO113: TRAMO DE LA CACHE EN BLOQUES DE VRAM techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "pedazoLoDiceLaVram=", pedazoLoDiceElMonton, "vramDisponible=", pvr.VramDisponible(),
			"vramServiriaDeUnGolpe=", pvr.VramServible(tramoDeLaCache))
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramoEnPedazos(tramoDeLaCache, tramoEnPedazos,
			sondaDelTramo()...))
		println("JUEGO113: TRAMO DE LA CACHE EN PEDAZOS techo=", tramoDeLaCache, "pedazo=", tramoEnPedazos,
			"pedazoLoDiceElMonton=", pedazoLoDiceElMonton)
	} else if tramoDeLaCache > 0 && cacheEnVram {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramoDe(tramoDeLaCache, pvr.VramReservar, "vram"))
		println("JUEGO113: TRAMO DE LA CACHE EN UN GOLPE DE VRAM bytes=", tramoDeLaCache,
			"vramDisponible=", pvr.VramDisponible())
	} else if tramoDeLaCache > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramo(tramoDeLaCache))
		println("JUEGO113: TRAMO DE LA CACHE pedido TARDE (en el renderer) bytes=", tramoDeLaCache)
	}

	// DC-002.113b -- LA POLITICA CUANDO EL TRAMO SE LLENA. Las dos son ADITIVAS y se eligen con las palancas de
	// esta tarjeta: con `desalojoEnCuadros > 0` el tramo suelta lo que no se uso en N cuadros y reusa esos bytes;
	// con `componerAlVuelo` lo que no entra se compone sobre un buffer de trabajo y NO se cachea. Con las dos
	// apagadas el POC es el de DC-002.113 (la cache de solo agregar y los derrames al monton de Go).
	if desalojoEnCuadros > 0 {
		opciones = append(opciones, d2maprenderer.ConDesalojoDeLaCache(desalojoEnCuadros))
		println("JUEGO113: DESALOJO DE LA CACHE encendido: una imagen que no se uso en", desalojoEnCuadros,
			"cuadros se suelta y sus bytes vuelven al tramo")
	}

	if componerAlVuelo {
		opciones = append(opciones, d2maprenderer.ConComposicionAlVuelo(bufferDelVuelo))
		println("JUEGO113: COMPOSICION AL VUELO encendida: lo que no entra en el tramo se compone en un buffer de",
			bufferDelVuelo, "B y NO se cachea")
	}

	if desalojoEnCuadros == 0 && !componerAlVuelo {
		println("JUEGO113: la cache de tiles es de SOLO AGREGAR (sin desalojo y sin composicion al vuelo):",
			"lo que no entra en el tramo se derrama al monton de Go, como en DC-002.113")
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY, opciones...)

	espacio := rendererDelMapa.TamanoDelCuadro()
	println("JUEGO113: renderer listo; cuadro=", espacio.W, espacio.H,
		"cachePerezosa=", rendererDelMapa.CachePerezosa(), "cacheEnIndices=", rendererDelMapa.CacheDeIndices(),
		"desalojoEnCuadros=", rendererDelMapa.DesalojoEnCuadros(),
		"composicionAlVuelo=", rendererDelMapa.ComposicionAlVuelo())
	censoDelTramo("despues-del-renderer")
	memoria("despues-del-renderer")
	semiespacio("despues-del-renderer")

	// ============================== 8. EL JUGADOR Y SU VELOCIDAD ==============================
	mov = NuevoMovimiento(celdaX, celdaY)

	println("JUEGO113: MOTOR velocidad=subtilesPorSegundo=", mov.Est.Velocidad,
		"celdasPorSegundo=", mov.Est.Velocidad/float64(subtilesPorCelda),
		"ticksPorSegundo=", 25, "segundosPorTick=", segundosPorTick)

	direcciones := DireccionesDelControl()
	println("JUEGO113: DIRECCIONES de la cruceta", len(direcciones), "lista=", len(direcciones))

	for _, d := range direcciones {
		println("JUEGO113: DIRECCION de la cruceta dir=", d)
	}

	println("JUEGO113: CONTROL hay=", maple.HayControl(), "servicio=", servicio != nil,
		"inyectarGuion=", inyectarGuion, "guionTramos=", len(guionInyectado), "guionSegundos=",
		SegundosDelGuion(pausaEntreTramos))

	if inyectarGuion {
		println("JUEGO113: GUION INYECTADO -- el estado del maple lo pone el SHIM (d2c_control_inyectar):",
			"NO hay mando fisico en el bus, la evidencia lo dice")
	}

	msAnterior = int64(d2cMs())

	// ============================== 9. EL CUADRO EN PANTALLA ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) DC-002.113"); err != nil {
		println("JUEGO113: el ciclo de cuadros salio con error:", err.Error())
	}

	println("JUEGO113: FIN pico del monton=", picoMonton, "de arena=", d2cMontonArena(),
		"libre=", d2cMontonLibre(), "crcDelMapaSolo=", int(crcDelMapaSolo))
}

// componerNivel (DC-002.109): compone `preset/archivo` en el motor y devuelve el censo de la grilla.
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

	println("JUEGO113: MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crc), "ms=", int(d2cMs()-t0), "ds1enArena=", enArena, "bloque=", bloque,
		"ds1Soltado=", soltados, "w=", tamano.Width, "h=", tamano.Height)

	return pisos, paredes, crc, true
}

// soltarLoQueElCuadroNoLee (DC-002.105): devuelve al monton de C las dos piezas que el CUADRO NO USA.
func soltarLoQueElCuadroNoLee(am *d2asset.AssetManager) int {
	if soltarDespuesDeComponer == "nada" {
		println("JUEGO113: no se suelta nada despues de componer")

		return 0
	}

	total := 0

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		precargarLoQueElCuadroLee(am)
	}

	if soltarDespuesDeComponer == "tablas" || soltarDespuesDeComponer == "todo" {
		memoria("antes-de-soltar-las-tablas")

		res := am.SoltarFilasDeLasTablas(tablasCargadas)

		println("JUEGO113: TABLAS SOLTADAS tablas=", res.Tablas, "tramos=", res.Tramos, "bytes=", res.Bytes,
			"vacias=", len(res.Vacias), "noSoportadas=", len(res.NoSoportadas))

		total += res.Bytes

		memoria("despues-de-soltar-las-tablas")
	}

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		for _, f := range fuentesDeConsola {
			r := f.Soltar(func(tramo uintptr, bytes int) int { return d2cSoltarTramo(tramo, bytes) })

			println("JUEGO113: FUENTE SOLTADA", r.Ruta, "tablas=", r.Tablas.Total,
				"enElMontonDeC=", r.Tablas.Externas, "bytesTablas=", r.BytesTablas,
				"bytesTrabajo=", r.BytesTrabajo, "bytes=", r.Bytes, "cedido=", r.Cedido,
				"yaSoltada=", r.YaSoltada)

			total += r.Bytes
		}

		memoria("despues-de-soltar-las-fuentes")
	}

	println("JUEGO113: SOLTADO despues de componer bytes=", total, "libre=", d2cMontonLibre(),
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
		println("JUEGO113: no se pudo precargar la paleta del acto:", err.Error())
	}

	if _, err := am.LoadPaletteTransform(pl2); err != nil {
		println("JUEGO113: no se pudo precargar el PL2 del acto:", err.Error())
	}

	println("JUEGO113: PRECARGADO lo que el cuadro lee del MPQ", paleta, pl2, "delta=", d2cMontonUsado()-usado)
}

// registrarFuente abre el MPQ con la Source de consola, le inyecta el `free` del shim y la pone PRIMERA en el
// loader del motor (DC-002.96/.99).
func registrarFuente(am *d2asset.AssetManager, ruta string) bool {
	f, err := fuenteconsola.Abrir(ruta)
	if err != nil {
		println("JUEGO113: MPQ-ABRIR FALLO:", ruta, err.Error())

		return false
	}

	f.UsarLiberador(fuenteconsola.Liberador(d2cLibre))
	f.RegistrarEn(am.Loader)

	println("JUEGO113: fuente registrada", ruta)
	fuentesDeConsola = append(fuentesDeConsola, f)

	return true
}

// censoDelTramo (DC-002.104): donde viven los pixeles de la cache.
func censoDelTramo(etapa string) {
	if rendererDelMapa == nil {
		return
	}

	e := rendererDelMapa.CensoDelTramo()
	println("JUEGO113: TRAMO", etapa, "activa=", e.Activa, "origen=", e.Origen, "bytes=", e.Bytes, "usado=", e.Usado,
		"libre=", e.Libre, "imagenes=", e.Imagenes, "derrames=", e.Derrames, "bytesDerramados=", e.BytesDerramados,
		"enPedazos=", e.EnPedazos, "pedazos=", e.Pedazos, "pedazoMenor=", e.PedazoMenor,
		"pedazoMayor=", e.PedazoMayor, "techo=", e.Techo,
		// DC-002.113b: el desalojo y la composicion al vuelo, que es lo que esta tarjeta mide
		"desalojos=", e.Desalojos, "bytesDesalojados=", e.BytesDesalojados, "huecos=", e.Huecos,
		"alVuelo=", e.AlVuelo, "bytesAlVuelo=", e.BytesAlVuelo, "agrandados=", e.Agrandados,
		"cuadroDeLaCache=", rendererDelMapa.CuadroDeLaCache())
}

// instrumentoDeLasFuentes: el censo de la fuente (pedidos, liberados, vivos, prestamo) para el log.
func instrumentoDeLasFuentes(etapa string) {
	for _, f := range fuentesDeConsola {
		e := f.Estado()
		println("JUEGO113: FUENTE", etapa, f.Path(), "abiertos=", e.Abiertos, "enBuffer=", e.LecturasEnBuffer,
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
		println("JUEGO113: TABLA FALLO (fuente):", ruta, err.Error())

		return false
	}

	println("JUEGO113: TABLA fuente", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// cargarTablaPorStream: el camino de HOY (`LoadDataDictionary` por stream del MPQ), para el A/B.
func cargarTablaPorStream(am *d2asset.AssetManager, ruta string) bool {
	usado := d2cMontonUsado()
	tamano, _ := am.TamanoDeArchivo(ruta)

	dict, err := am.LoadDataDictionaryPorStream(ruta)
	if err != nil {
		println("JUEGO113: TABLA FALLO (stream):", ruta, err.Error())

		return false
	}

	if err := am.Records.Load(ruta, dict); err != nil {
		println("JUEGO113: TABLA FALLO (stream, records):", ruta, err.Error())

		return false
	}

	println("JUEGO113: TABLA stream", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// actualizar: UN CUADRO DEL JUEGO. Lee el control por la interfaz del motor, mueve al jugador con la velocidad del
// exe en ticks de 25 Hz, elige la direccion y el cuadro de la animacion, y pone la camara en el jugador.
//
// EL ORDEN NO ES DECORACION: el movimiento va ANTES de dibujar porque la camara y el ancla del jugador son la
// entrada del cuadro que se pinta despues.
func actualizar() error {
	if rendererDelMapa == nil || mov == nil {
		return nil
	}

	ahora := int64(d2cMs())
	paso := float64(ahora-msAnterior) / 1000.0
	msAnterior = ahora

	if !primerCuadro {
		primerCuadro = true
		paso = 0
	}

	if paso < 0 {
		paso = 0
	}

	// ============================== 1. EL CONTROL ==============================
	// La corrida del criterio lee el control DEL EMULADOR (el teclado virtual del host aprieta la cruceta y Flycast
	// arma el estado del maple). Con `inyectarGuion` el estado lo pone el shim, y se dice.
	if inyectarGuion {
		// el estado del maple lo pone el shim: el servicio lo lee igual que a un control de verdad
		servicio.InyectarBotones(BotonesDelGuion(reloj, pausaEntreTramos))
	}

	hayLectura := servicio.ActualizarDesdeElControl()

	ejeX, ejeY := ejesDelControl(entrada)

	// ============================== 2. EL MOVIMIENTO ==============================
	// La celda de ANTES se toma antes de mover: el tramo tiene que arrancar donde estaba el jugador, no donde
	// quedo (si no, `celdaIni` y `celdaFin` de un tramo salen iguales y el tramo no dice nada del camino).
	celdaAntesX, celdaAntesY := mov.Est.Celda()

	ticks, caminado := mov.Paso(ejeX, ejeY, paso)

	if caminado > 0 {
		medirTramo(ejeX, ejeY, paso, caminado, celdaAntesX, celdaAntesY)
	} else {
		cerrarTramo()
	}

	// ============================== 3. LA CAMARA ==============================
	seguirConLaCamara()

	// ============================== 4. EL RELOJ DEL GUION ==============================
	reloj += paso

	// LA RECOLECCION A PEDIDO NO VA EN EL CUADRO. Medido en esta tarjeta: pedirle un runtime.GC() al
	// recolector desde el callback del ciclo del PVR CLAVA el ciclo (dos corridas quedaron en el cuadro 7 y no
	// volvieron a presentar). El recolector entra por umbral solo, que es lo que ya hace, y con el conjunto
	// vivo medido (1.328.120 B, 72 %) le queda lugar de sobra. El umbral alto de knobs.go es la palanca.

	if cuadro <= 1 || caminoCambio(ticks, caminado) {
		camX, camY := 0.0, 0.0

		if p := rendererDelMapa.Camera.GetPosition(); p != nil {
			camX, camY = p.X(), p.Y()
		}

		cx, cy := mov.Est.Celda()

		println("JUEGO113: CONTROL cuadro=", cuadro+1, "lectura=", hayLectura, "botones=", int(servicio.Botones()),
			"ejes=", int(ejeX), int(ejeY), "dir=", mov.Direccion, "caminando=", mov.Caminando,
			"celda=", cx, cy, "ticks=", ticks, "subtiles=", caminado, "camara=", camX, camY,
			"reloj=", reloj, "guionBotones=", int(BotonesDelGuion(reloj, pausaEntreTramos)))
	}

	return nil
}

// caminoCambio: si este cuadro hubo movimiento o si es el primero (para no imprimir 60 lineas iguales).
func caminoCambio(ticks int, caminado float64) bool { return ticks > 0 || caminado > 0 }

// seguirConLaCamara: la camara del motor se pone en el jugador (lo que hace `d2gamescreen` con el heroe:
// `MoveCameraTo(WorldToOrtho(posicion del heroe))`). Con `camara=fija` no se toca: es la corrida que deja ver al
// jugador cruzando la pantalla.
func seguirConLaCamara() {
	if camara != "siguiendo" || mov == nil || mov.Est == nil {
		return
	}

	w := mov.Est.Pos.World()
	rx, ry := rendererDelMapa.WorldToOrtho(w.X(), w.Y())
	pos := d2vector.NewPosition(rx, ry)

	rendererDelMapa.MoveCameraTo(&pos)
}

// medirTramo: suma el tramo de caminata en curso. Si la direccion cambio, cierra el anterior y abre otro: cada una
// de las 8 direcciones tiene su propio numero.
func medirTramo(ejeX, ejeY, paso, caminado, celdaAntesX, celdaAntesY float64) {
	if tramo == nil || tramo.Dir != mov.Direccion {
		cerrarTramo()

		tramo = &tramoMedido{Nombre: nombreDeLosEjes(ejeX, ejeY), Dir: mov.Direccion, CuadroIni: cuadro,
			CeldaIniX: celdaAntesX, CeldaIniY: celdaAntesY}
		tramos++
	}

	tramo.Segundos += paso
	tramo.Subtiles += caminado
	tramo.Cuadros++
}

// cerrarTramo: imprime el tramo medido. El numero del criterio es el de esta linea: celdas por segundo con el
// boton mantenido.
func cerrarTramo() {
	if tramo == nil {
		return
	}

	t := tramo
	tramo = nil
	reportados++

	celdas := t.Subtiles / float64(subtilesPorCelda)
	porSegundo := 0.0
	celdasPorSegundo := 0.0

	if t.Segundos > 0 {
		porSegundo = t.Subtiles / t.Segundos
		celdasPorSegundo = celdas / t.Segundos
	}

	cx, cy := mov.Est.Celda()

	println("JUEGO113: TRAMO", t.Nombre, "dir=", t.Dir, "cuadros=", t.Cuadros, "desdeElCuadro=", t.CuadroIni,
		"segundos=", t.Segundos, "subtiles=", t.Subtiles, "celdas=", celdas,
		"celdasPorSegundo=", celdasPorSegundo, "subtilesPorSegundo=", porSegundo,
		"teoricoSubtilesPorSegundo=", mov.Est.Velocidad, "celdaIni=", t.CeldaIniX, t.CeldaIniY, "celdaFin=", cx, cy)
}

// nombreDeLosEjes: "DERECHA", "ABAJO+DERECHA", ... para el log.
func nombreDeLosEjes(ejeX, ejeY float64) string {
	n := ""

	if ejeY < 0 {
		n = "ARRIBA"
	} else if ejeY > 0 {
		n = "ABAJO"
	}

	if ejeX < 0 {
		if n != "" {
			n += "+"
		}

		n += "IZQUIERDA"
	} else if ejeX > 0 {
		if n != "" {
			n += "+"
		}

		n += "DERECHA"
	}

	return n
}

// dibujar: EL CUADRO. Primero el mapa por el camino del motor, DESPUES el jugador encima, en el ancla que da la
// posicion del motor y el `WorldToScreenF` del renderer.
//
// Los tres cuadros del cierre:
//
//	cuadro cuadros+1  el jugador y la camara vuelven al origen (la celda del cuadro 1).
//	cuadro cuadros+2  LA REPETIBILIDAD: el MISMO estado otra vez, y el crc32 tiene que ser el de cuadros+1 (con la
//	                  camara en la misma posicion el cuadro es estable). Se compara contra cuadros+1 y no contra el
//	                  cuadro 1 porque la cache de tiles es perezosa: al cuadro 1 todavia no estaban cacheados los
//	                  tiles que se cachean al caminar (medido: 307.196 pixeles al cuadro 1, 307.200 al volver).
//	cuadro cuadros+3  se devuelve el jugador a donde lo dejo el guion: es el cuadro que queda en pantalla.
func dibujar(superficie d2interface.Surface) error {
	if rendererDelMapa == nil {
		return nil
	}

	cuadro++

	if cuadro > cuadros+3 {
		return nil
	}

	// los cuadros del cierre se preparan ACA, que es donde ya se sabe que numero de cuadro es
	switch cuadro {
	case cuadros + 1:
		volverAlOrigen() // la camara y el jugador vuelven a la celda de arranque
	case cuadros + 3:
		volverAlFinal() // y se devuelve el jugador a donde lo dejo el guion
	}

	println("JUEGO113: CUADRO", cuadro, "ARRANCA libre=", d2cMontonLibre())

	malla, esPvr := superficie.(*pvr.Surface)
	if !esPvr {
		println("JUEGO113: el Renderer no entrego una Surface del PVR")

		return nil
	}

	if cuadro == 1 {
		println("JUEGO113: PRIMER-CUADRO cuadro=1 celda=", celdaX, celdaY, "camara=", camara)
	}

	t0 := d2cMs()
	rendererDelMapa.Render(superficie)
	msMapa := int(d2cMs() - t0)

	pixeles := malla.Imagen().Pixeles()

	llenos := contarLlenos(pixeles)
	crcMapa := crc32.ChecksumIEEE(pixeles)

	if cuadro == 1 {
		crcDelMapaSolo = crcMapa
		crcCuadro1Mapa = crcMapa
		llenosCuadro1 = llenos
	}

	println("JUEGO113: CUADRO", cuadro, "SOLO-MAPA pixelesNoVacios=", llenos, "de", ancho*alto,
		"crc32=", int(crcMapa), "hostPixeles=", pixelesHost, "hostCrc=", int(crcHost), "ms=", msMapa,
		"igualAlHost=", crcMapa == crcHost)

	msJugador := 0
	crcConJugador := crcMapa
	cambiados := 0
	anclaX, anclaY := 0, 0
	dirDibujada := mov.Direccion
	cuadroAnim := -1

	if jugadorOK {
		ciclo := cicloDelJugador(mov.Direccion, mov.Caminando)

		if ciclo != nil {
			cu := mov.CuadroDeLaAnimacion(ciclo)
			if cu != nil {
				cuadroAnim = indiceDelCuadro(ciclo, cu)

				cx, cy := mov.Est.Celda()

				anclaX, anclaY = AnclaDelJugador(rendererDelMapa.WorldToScreenF, cx, cy)
				caja := CajaDe(anclaX, anclaY, cu)

				if cuadro == 1 {
					anclaCuadro1X, anclaCuadro1Y = anclaX, anclaY
					println("JUEGO113: JUGADOR ancla=", anclaX, anclaY, "caja=", caja.X0, caja.Y0, caja.X1, caja.Y1,
						"tamano=", caja.Ancho(), "x", caja.Alto(), "compuesto=", cu.Ancho, "x", cu.Alto,
						"esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios, "crc32=", int(cu.CRC),
						"modo=", ciclo.Modo, "dir=", ciclo.Dir, "cuadroDelCiclo=", cuadroAnim,
						"capasDibujadas=", cu.CapasDibujadas, "sinGrafico=", cu.CapasSinGrafico)
				}

				// 1) se guarda lo que hay DEBAJO de la caja (el mapa solo) y se pinta al jugador
				guardarCaja(pixeles, caja)

				t1 := d2cMs()
				PintarJugador(malla, cu, anclaX, anclaY, palJugador[:])
				msJugador = int(d2cMs() - t1)

				crcConJugador = crc32.ChecksumIEEE(pixeles)
				cambiados = pixelesCambiados(pixeles, caja)

				// 2) se devuelve la caja al mapa y se mide otra vez: tiene que volver al crc32 del mapa solo
				restaurarCaja(pixeles, caja)

				crcRestaurado := crc32.ChecksumIEEE(pixeles)

				if crcRestaurado != crcMapa {
					println("JUEGO113: CUADRO", cuadro, "MAPA-RESTAURADO DISTINTO crc32=", int(crcRestaurado),
						"mapaSolo=", int(crcMapa))
				}

				// 3) y se pinta de nuevo, que es lo que se presenta
				PintarJugador(malla, cu, anclaX, anclaY, palJugador[:])
			}
		}
	}

	println("JUEGO113: CUADRO", cuadro, "CON-JUGADOR pixelesNoVacios=", contarLlenos(pixeles), "de", ancho*alto,
		"crc32=", int(crcConJugador), "pixelesDelJugador=", cambiados, "ancla=", anclaX, anclaY,
		"dir=", dirDibujada, "cuadroDelCiclo=", cuadroAnim, "modo=", modoDelCuadro(mov.Caminando),
		"msMapa=", msMapa, "msJugador=", msJugador, "celda=", celdaDelJugadorX(), celdaDelJugadorY())

	if cuadro == 1 {
		crcCuadro1Jugador = crcConJugador

		println("JUEGO113: CUADRO 1 REFERENCIA igualAlDeDC002112=", crcConJugador == crcConJugador112,
			"crc32=", int(crcConJugador), "esperado=", crcConJugador112, "ancla=", anclaX, anclaY)
	}

	// ============================== LA REPETIBILIDAD (los cuadros del cierre) ==============================
	//
	// SE COMPARA EL ORIGEN CONTRA SI MISMO, Y NO CONTRA EL CUADRO 1, POR UNA RAZON MEDIDA: la cache de tiles del
	// renderer es PEREZOSA y de SOLO AGREGAR, asi que el cuadro 1 no tiene cacheados todos los tiles que si estan
	// al volver (en la corrida del criterio, cuadro 1 = 307.196 pixeles no vacios y el origen al final = 307.200:
	// los cuatro pixeles que al principio quedaban en negro son tiles que se cachearon al caminar). El criterio que
	// esta tarjeta puede sostener es el otro: con la camara y el jugador en la MISMA celda, dos cuadros seguidos
	// dan el MISMO crc32. Eso se mide en cuadros+2 contra cuadros+1, y ademas se deja anotada la diferencia con el
	// cuadro 1 (`igualAlCuadro1`), que es la del cache perezoso.
	if cuadro == cuadros+1 {
		crcOrigenMapa = crcMapa
		crcOrigenJugador = crcConJugador
		llenosOrigen = contarLlenos(pixeles)
	}

	if cuadro == cuadros+2 {
		igualOrigenMapa := crcMapa == crcOrigenMapa
		igualOrigenJugador := crcConJugador == crcOrigenJugador

		println("JUEGO113: REPETIBILIDAD cuadro=", cuadro, "camara=", camara, "crcMapa=", int(crcMapa),
			"crcMapaDelCuadroAnterior=", int(crcOrigenMapa), "igualAlCuadroAnterior=", igualOrigenMapa,
			"crcConJugador=", int(crcConJugador), "crcConJugadorDelCuadroAnterior=", int(crcOrigenJugador),
			"igualConJugador=", igualOrigenJugador, "ancla=", anclaX, anclaY, "celda=", celdaDelJugadorX(),
			celdaDelJugadorY(), "llenos=", contarLlenos(pixeles), "llenosDelCuadroAnterior=", llenosOrigen,
			"crcMapaDelCuadro1=", int(crcCuadro1Mapa), "igualAlCuadro1=", crcMapa == crcCuadro1Mapa,
			"llenosDelCuadro1=", llenosCuadro1, "cachePerezosa=", rendererDelMapa.CachePerezosa())
	}

	// ============================== EL RESUMEN DEL MOVIMIENTO ==============================
	if cuadro >= cuadros+1 {
		cache := rendererDelMapa.EstadoDeLaCache()
		println("JUEGO113: CUADRO", cuadro, "cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes,
			"cacheEnIndices=", cache.EnIndices, "msMapa=", msMapa, "msJugador=", msJugador)
	}

	censoDelTramo("despues-del-cuadro")
	memoria("despues-del-cuadro")

	if cuadro == 1 && orden != "tarjeta" {
		tramosSoltados, bytes := d2asset.SoltarArenaDeBloques()
		println("JUEGO113: ARENA DE BLOQUES SOLTADA (despues de la cache) tramos=", tramosSoltados, "bytes=", bytes,
			"libre=", d2cMontonLibre())
		memoria("despues-de-soltar-el-arena")
	}

	// el resumen va ULTIMO y una sola vez: el ciclo del PVR no vuelve nunca
	if cuadro == cuadros+3 {
		resumen()
	}

	return nil
}

// resumen: los numeros del criterio, en una linea (y el detalle del movimiento en las de arriba).
func resumen() {
	celdas := mov.Subtiles / float64(subtilesPorCelda)
	celdasPorSegundo := 0.0

	if mov.Segundos > 0 {
		celdasPorSegundo = celdas / mov.Segundos
	}

	cx, cy := mov.Est.Celda()

	println("JUEGO113: RESUMEN cuadrosPintados=", cuadro, "tramos=", tramos, "tramosReportados=", reportados,
		"segundosCaminados=", mov.Segundos, "subtiles=", mov.Subtiles, "celdas=", celdas,
		"celdasPorSegundo=", celdasPorSegundo, "teoricoCeldasPorSegundo=", mov.Est.Velocidad/float64(subtilesPorCelda),
		"ticks=", mov.Ticks, "recortes=", mov.Recortes, "pasoMayor=", mov.PasoMayor,
		"celdasFinales=", cx, cy, "direcciones=", len(direccionesCompuestas), "camara=", camara,
		"inyectarGuion=", inyectarGuion, "reloj=", reloj)
}

// volverAlOrigen: deja el mundo en el estado exacto del cuadro 1 (celda de arranque, direccion y cuadro de
// animacion del ciclo quieto, camara en el arranque). Es la preparacion de la prueba de repetibilidad.
func volverAlOrigen() {
	if mov == nil {
		return
	}

	guardarElFinal()

	mov.Est = NuevoEstado(celdaX, celdaY, false)
	mov.Direccion = dirQuieto
	mov.Caminando = false
	mov.MsAnim = 0
	mov.Resto = 0

	if camara == "siguiendo" {
		w := mov.Est.Pos.World()
		rx, ry := rendererDelMapa.WorldToOrtho(w.X(), w.Y())
		pos := d2vector.NewPosition(rx, ry)

		rendererDelMapa.MoveCameraTo(&pos)
	} else {
		// la camara fija tiene que estar TAMBIEN en la celda de arranque para que el cuadro sea el mismo
		rx, ry := rendererDelMapa.WorldToOrtho(celdaX, celdaY)
		pos := d2vector.NewPosition(rx, ry)

		rendererDelMapa.MoveCameraTo(&pos)
	}

	println("JUEGO113: VUELTA AL ORIGEN celda=", celdaX, celdaY, "dir=", mov.Direccion, "camara=", camara)
}

// guardarElFinal: guarda a donde llego el guion (la posicion, la direccion, el cuadro de la animacion y si estaba
// caminando) para poder volver a ese estado despues de la prueba de repetibilidad.
func guardarElFinal() {
	if mov == nil || mov.Est == nil {
		return
	}

	celdaFinalX, celdaFinalY = mov.Est.Celda()
	dirFinal = mov.Direccion
	msAnimFinal = mov.MsAnim
	caminandoFinal = mov.Caminando
}

// volverAlFinal: devuelve el jugador a donde lo dejo el guion. Es el cuadro que queda en pantalla.
func volverAlFinal() {
	if mov == nil {
		return
	}

	mov.Est = NuevoEstado(celdaFinalX, celdaFinalY, false)
	mov.Direccion = dirFinal
	mov.MsAnim = msAnimFinal
	mov.Caminando = caminandoFinal

	if camara == "siguiendo" {
		w := mov.Est.Pos.World()
		rx, ry := rendererDelMapa.WorldToOrtho(w.X(), w.Y())
		pos := d2vector.NewPosition(rx, ry)

		rendererDelMapa.MoveCameraTo(&pos)
	}

	println("JUEGO113: VUELTA AL FINAL celda=", celdaFinalX, celdaFinalY, "dir=", mov.Direccion,
		"caminando=", mov.Caminando)
}

// cicloDelJugador: el ciclo que corresponde al estado (caminando: la direccion que mira; quieto: NU dir 0).
func cicloDelJugador(dir int, caminando bool) *CicloDelJugador {
	if caminando {
		if c := jugadoresCaminando[dir]; c != nil {
			return c
		}

		return jugadorQuieto
	}

	return jugadorQuieto
}

// indiceDelCuadro: en que cuadro del ciclo esta (para el log).
func indiceDelCuadro(ciclo *CicloDelJugador, cu *CuadroDelJugador) int {
	for i := range ciclo.Cuadros {
		if &ciclo.Cuadros[i] == cu {
			return i
		}
	}

	return -1
}

// modoDelCuadro: "TW" caminando, "NU" quieto (el log dice cual de los dos modos se dibujo).
func modoDelCuadro(caminando bool) string {
	if caminando {
		return modoCaminando
	}

	return modoQuieto
}

// celdaDelJugadorX / celdaDelJugadorY: la celda del motor donde esta parado, con decimales (la consola no tiene
// fmt: van como dos numeros en el mismo println).
func celdaDelJugadorX() float64 {
	if mov == nil || mov.Est == nil {
		return 0
	}

	cx, _ := mov.Est.Celda()

	return cx
}

func celdaDelJugadorY() float64 {
	if mov == nil || mov.Est == nil {
		return 0
	}

	_, cy := mov.Est.Celda()

	return cy
}

// ============================== LOS INSTRUMENTOS DEL LIENZO ==============================

// contarLlenos: los pixeles no negros del lienzo.
func contarLlenos(pixeles []byte) int {
	llenos := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	return llenos
}

// guardarCaja: copia lo que hay en el lienzo dentro de la caja.
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

// pixelesCambiados: cuantos pixeles de la caja NO son los del mapa.
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

// sondaDelTramo (DC-002.106): la funcion que le pregunta al monton cuanto puede servir, o nil.
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
