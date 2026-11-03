//go:build dreamcast

// DC-002.120 -- EL CICLO CON LOS CUATRO: EL MUNDO CAMBIANDO CON LA GENTE ADENTRO.
//
// ESTE POC ES EL DE DC-002.117 (cuatro jugadores en el mismo cuadro de 640x480, uno por PUERTO del bus maple, con
// el LEASH DE GRUPO) MAS EL CICLO DE NIVELES DE DC-002.111 (`fases_120.go`): despues del guion de los cuatro se
// corren las TRES FASES del cambio de nivel -- ENTRAR / TRANSICION / VOLVER -- con los cuatro dibujados encima y
// el leash puesto, y se mide el pico del monton de C y del semiespacio POR FASE, los ms por cuadro por fase y el
// crc32 del cuadro de la VUELTA (que tiene que seguir siendo 1437022778 en las zonas sin jugador).
//
// LO QUE HACE FALTA PARA QUE SEA UNA SOLA CORRIDA, y no se toca mas que esto:
//  1. `gestorDeAssets` y `motorDelMapa` globales, para que la fase pueda reiniciar la arena de los `.dt1`,
//     soltar las casillas del nivel que se va y colgar el nivel nuevo en el renderer (`SetMapEngine`).
//  2. Con `faseKnob=ciclo` NO se sueltan las fuentes ni las tablas despues de componer el pueblo: la fase
//     siguiente no tiene nada que reabrir y esos vivos no se piden en cada fase (es la palanca que hizo entrar
//     la VUELTA de DC-002.111; ver `soltarEnElCiclo` en knobs.go).
//  3. `dibujar` le pasa los cuadros de despues del guion al ciclo (`dibujarElCiclo`).
//
// Lo de abajo es la historia de DC-002.117/.115/.113, que es el camino que este POC lleva adentro.
//
// DC-002.117 -- TRES Y CUATRO JUGADORES: LA TABLA DE MEMORIA Y MS POR CUADRO.
//
// ESTE POC ES EL DE DC-002.116 (el split para dos) con la palanca `JUGADORES` llevada hasta CUATRO: los cuatro
// puertos del bus maple, cada jugador con su control, su estado y su reloj de animacion, y los cuatro dibujados en
// el MISMO cuadro de 640x480 con el LEASH DE GRUPO (`leash.go`). NO HAY SPLIT para 3 y 4: el split de 3 y 4 es otra
// tarjeta, y aca lo que se mide es si el semiespacio y el cuadro de una pantalla aguantan cuatro.
//
// LO QUE SE MIDE (y es la tabla de la tarjeta): los cuatro casos -- 1, 2, 3 y 4 jugadores -- con el MISMO binario,
// cambiando solo `JUGADORES`. De cada corrida salen la memoria (estado por jugador, cuadros compartidos, la
// estructura del participante) y los ms por cuadro (mapa, jugadores, presentacion y cuadro de punta a punta).
//
// Lo de abajo es la historia de DC-002.115/.113, que es el camino que este POC mide.
//
// DC-002.115 -- EL LEASH: DOS JUGADORES EN UNA PANTALLA, NINGUNO SE SALE DEL LIENZO.
//
// ESTE POC ES EL DE DC-002.114 (dos jugadores, uno por PUERTO del bus maple, con su camara en el centro del
// grupo) MAS EL LEASH: la distancia maxima entre jugadores, MEDIDA al arranque con la proyeccion del motor y los
// cuadros compuestos (ver `leash.go`, que es el archivo de esta tarjeta), aplicada cuadro a cuadro y vigilada en
// cada cuadro (`dentroDelLienzo`). Lo de abajo es la historia de DC-002.113, que es el camino que este POC mide.
//
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
// LO QUE NO ENTRA (la tarjeta lo dice): dos jugadores (DC-002.113) y tocar `d2mapentity.Player` (arrastra 2,11 MB):
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
	// la que dibujo DC-002.112 (ancla del jugador en 320,283). La celda paso a PALANCA en DC-002.127 (abajo).
	ancho, alto = 640, 480

	// ============================== LA CELDA DEL ARRANQUE Y DE LA CAMARA (DC-002.127) ==============================
	// Era una constante (28,20 = el centro del townN1, que mide 57x41 celdas) hasta esta tarjeta. Deja de serlo
	// porque la ventana del POC cubre muy poco del pueblo: 640 px / 80 px por celda = 8 celdas en el eje de la
	// pantalla y 480 / 40 = 12 en el otro (medido: `AnclaDelJugador` da 80 y 40 px por celda, no 16 y 8 -- el
	// mundo del renderer esta en SUBTILES). O sea que la camara ve un rombo de ~8x12 celdas y los NPCs del pueblo
	// estan a 10-15 celdas del centro: sin mover la camara NO se puede ver a Warriv, Charsi y Kashya. Con estas
	// dos palancas se apunta la camara a la celda de cada NPC y se mide su dibujo (y su dialogo) uno por uno.

	// los dos numeros del cuadro de referencia del HOST (DC-002.94/.101/.106): el mapa solo.
	pixelesHost = 307196
	crcHost     = 1437022778

	// DC-002.120: el crc32 del cuadro de la TRANSICION en el host (TownE1, medido en DC-002.109), que es contra
	// el que se compara la fase del medio. La VUELTA se compara contra `crcHost` (el pueblo otra vez).
	crcHostTransicion = 584380401

	// DC-002.112: el cuadro del pueblo CON el jugador quieto (NU dir 0) encima.
	crcConJugador112 = 1363871674

	// pausaEntreTramos: lo que se suelta la cruceta entre tramo y tramo del guion inyectado (el teclado virtual
	// de la corrida del criterio suelta la tecla y espera lo mismo: `PAUSA=` de run-flycast-113.sh).
	pausaEntreTramos = 1.0
)

// celdaX/celdaY: la celda del arrranque del jugador 1 y de la camara, desde las palancas vistaX/vistaY.
var (
	celdaX = vistaX
	celdaY = vistaY

	// el bicho 1 nace una celda al lado del jugador (ver la nota en knobs.go)
	celdaBichoX = celdaX + 1.0
	celdaBichoY = celdaY + 1.0
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

	// rutaDelPueblo: el `.ds1` que el compositor acaba de poner en el mapa (DC-002.127). Lo deja `componerNivel`
	// desde `stamp.RegionPath()`: es lo que le permite a los NPCs leer SUS monstruos del pueblo que se dibujo, sin
	// adivinar cual de los cuatro `.ds1` del acto 1 fue.
	rutaDelPueblo string

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

	// DC-002.136: la fila de la TABLA DE CONSUMO DEL ARRANQUE. Va ANTES del corte por `verboso` a proposito: la
	// tabla es la evidencia de esta tarjeta y tiene que salir aunque las lineas por cuadro esten apagadas.
	anotarLaCarga(etapa, usado)

	// DC-002.134: el PICO se sigue tomando (es estado, no evidencia), pero la linea es evidencia POR CUADRO y
	// son ~150 B por el puerto serie: con `VERBOSO=no` no se imprime.
	if !verboso134() {
		return
	}

	println("JUEGO121: MEMORIA", etapa, "usado=", usado, "libre=", d2cMontonLibre(),
		"arena=", d2cMontonArena(), "sbrk=", int(d2cSbrkActual()), "techo=", int(d2cMontonTecho()),
		"pico=", picoMonton)
}

// semiespacio (DC-002.109): el censo del semiespacio del recolector -- lo que ocupa, cuanto mide y cuantas
// recolecciones van. Es el instrumento que separa "no hay memoria" de "no hay memoria DONDE se pide".
//
// DC-002.131: ADEMAS mide lo VIVO. `usado` aca es el puntero de asignacion (basura incluida, ver medidor_131.go);
// el numero que decide si el semiespacio alcanza es lo que sobrevive a una recoleccion. Se imprime al lado, con el
// mismo `usado` leido DESPUES de recolectar, y por eso las dos lineas van juntas: una dice "asignado", la otra
// "vivo", y la diferencia es la basura del tramo.
func semiespacio(etapa string) {
	usado, total := int(d2cGcUsado()), int(d2cGcTotal())
	pct := 0

	if total > 0 {
		pct = usado * 100 / total
	}

	println("JUEGO121: SEMIESPACIO", etapa, "usado=", usado, "total=", total, "pct=", pct,
		"colecciones=", int(d2cGcColecciones()))

	vivoYAnotar(etapa)
}

// pruebaDeEscrituraEnVram (DC-002.108): escribe un patron conocido en un bloque de la VRAM y lo vuelve a leer.
func pruebaDeEscrituraEnVram() {
	const bytes = 348160

	b := pvr.VramReservar(bytes)
	if b == nil {
		println("JUEGO121: PRUEBA VRAM no se pudo reservar", bytes, "B")

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

	println("JUEGO121: PRUEBA VRAM bytes=", len(b), "malos=", malos, "primeroMal=", primero,
		"leido=", int(b[0]), "esperado=", 3)

	pvr.VramSoltar(b)
}

// vram (DC-002.108): el censo de la VRAM del PVR en cada etapa.
func vram(etapa string) {
	c := pvr.FotoDeLaVram()

	println("JUEGO121: VRAM", etapa, "total=", c.Total, "disponible=", c.Disponible, "pedida=", c.Pedida,
		"pedidos=", c.Pedidos, "pedidoMayor=", c.PedidoMayor, "lienzo=", c.LienzoDelCuadro)
}

func main() {
	println("JUEGO122: arranca DC-002.122 -- el golpe: el jugador le pega al monstruo y lo mata")
	println("JUEGO122: cuadros=", cuadros, "combate=", combateKnob, "golpeAuto=", golpeAuto, "alcance=", alcanceGolpe,
		"dirsAtaque=", dirsAtaque, "monstruos=", monstruosKnob, "jugadores=", jugadoresKnob, "celdaP1=", celdaP1X,
		celdaP1Y, "celdaP2=", celdaP2X, celdaP2Y, "celdaP3=", celdaP3X, celdaP3Y, "celdaP4=", celdaP4X, celdaP4Y,
		"guionPoc=", guionPoc, "patrulla=", patrullaDx, patrullaDy, patrullaLargo)
	println("JUEGO121: cuadros=", cuadros, "fase=", faseKnob, "cuadrosPorFase=", cuadrosPorFase, "orden=",
		ordenDeLasFases, "jugador=", jugadorKnob, "camara=", camara, "modoCaminando=",
		modoCaminando, "msCaminando=", msCaminando, "dirsCaminando=", dirsCaminando, "inyectarGuion=",
		inyectarGuion, "tramoDeLaCache=", tramoDeLaCache, "cacheEnVram=", cacheEnVram)

	// DC-002.132: el camino rapido del tile (o el viejo, para el A/B) se elige ANTES de dibujar un cuadro.
	activarElBlitRapido()

	// DC-002.134: y el camino de la PALABRA (un deposito de 32 bits por pixel), tambien antes del primer cuadro.
	activarElBlitDePalabras()

	// DC-002.135: el camino de la PRESENTACION (la conversion y la subida) y el costo del reloj del instrumento.
	activarLaPresentacion135()

	// DC-002.135 (el mapa): el cronometro por bloque DENTRO del renderer del mapa y, si se pide, la corrida de
	// diagnostico del bucle de pixeles. Tambien antes del primer cuadro (el motor lo lee en cada tile).
	activarElCronometroDeLosTiles()

	println("JUEGO132: arranca -- medirCuadro=", medirCuadro, "blitRapido=", blitRapido,
		"cuadrosPorInforme=", cuadrosDelInforme132)
	println("JUEGO134: arranca -- etapas=", etapasKnob, "verboso=", verbosoEtapas, "blitPalabras=",
		blitPalabras134, "pintarUnaVez=", pintarUnaVez)

	// DC-002.137: las palancas del minimapa, en el arranque (una palanca que no se ve en el log no se puede
	// discutir: el numero de la corrida hay que poder atribuirlo a un camino).
	println("JUEGO137: arranca -- minimapa=", minimapaKnob, "cuando=", minimapaCuando, "todo=", minimapaTodo,
		"rapido=", minimapaRapido, "diag=", minimapaDiag)

	// DC-002.136: LO QUE QUEDO APAGADO, DICHO EN EL ARRANQUE. Una feature que no entra no desaparece en silencio: la
	// palanca que la apago queda en la consola serie con el nombre de lo que dejo de cargarse (ver instrumento_136.go).
	avisarLoQueQuedaApagado()

	memoria("al-arrancar")
	semiespacio("al-arrancar")
	vram("al-arrancar")

	// ============================== 0. EL VIDEO PRIMERO ==============================
	// El PVR necesita el monton de C casi vacio para arrancar (DC-002.50): es el mismo orden de DC-002.103.
	renderer := pvr.NuevoRenderer()

	// DC-002.116: el ciclo se guarda para poder ponerle el split (superficie propia + presentador) antes de Run.
	rendererDelCiclo = renderer

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("JUEGO121: el video no arranco:", err.Error())

		return
	}

	println("JUEGO121: PVR andando; vram de la textura del cuadro=", pvr.VramDeLaPantalla())
	memoria("despues-del-video")
	vram("despues-del-video")

	if pruebaDeVram {
		pruebaDeEscrituraEnVram()
	}

	if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		println("JUEGO121: el tramo de la cache se pide EN PEDAZOS (techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "): no se reserva nada al arranque (DC-002.106)")
	} else if tramoDeLaCache > 0 && tramoAlArranque {
		tramoDePixeles = d2maprenderer.ReservarTramoDeLaCache(tramoDeLaCache)
		println("JUEGO121: TRAMO DE LA CACHE reservado AL ARRANQUE bytes=", tramoDePixeles.Bytes())
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
	println("JUEGO121: ARENA DEL DS1 palanca=", arenaDelDS1, "encendida=", arena.Encendida,
		"conLiberador=", arena.ConLiberador, "bytesPorCelda=", d2ds1.BytesPorCelda())

	d2asset.UsarScratchPorArchivo(scratchPorArchivo)
	println("JUEGO121: SCRATCH DE LECTURA porArchivo=", d2asset.ScratchPorArchivoActivo(),
		"liberador=", d2asset.HayLiberadorDeExternos())

	d2asset.UsarMedidorDelMontonDeC(func() int { return d2cMontonUsado() })

	if tramoDelArenaPorElMonton {
		d2asset.UsarSondaDelMontonDeC(func(tope int) int { return int(d2cServible(uint(tope))) })
	}

	println("JUEGO121: TRAMO DEL ARENA DE BLOQUES porElMonton=", tramoDelArenaPorElMonton,
		"sondaInstalada=", d2asset.HaySondaDelMontonDeC())

	d2asset.UsarSoltarTramosAlReiniciar(soltarTramosAlCambiarDeNivel)
	println("JUEGO121: TRAMOS AL CAMBIAR DE NIVEL seDevuelven=", d2asset.SoltarTramosAlReiniciarActivo())

	d2asset.UsarLiberadorDeFilas(&d2records.LiberadorDeFilas{
		SoltarMapa:  func(mapa unsafe.Pointer) int { return d2cSoltarFilas(uintptr(mapa)) },
		SoltarTramo: func(bloque unsafe.Pointer, bytes int) int { return d2cSoltarTramo(uintptr(bloque), bytes) },
	})

	if modoTablas != "stream" {
		d2asset.ActivarTablasPorLaFuente()
	}

	println("JUEGO121: tablasPorLaFuente=", d2asset.TablasPorLaFuenteActivas(),
		"liberadorDeExternos=", d2asset.HayLiberadorDeExternos())
	memoria("despues-de-arrancar-como-consola")

	am, err := d2asset.NewAssetManager(d2util.LogLevelError)
	if err != nil {
		println("JUEGO121: NewAssetManager FALLO:", err.Error())

		return
	}

	println("JUEGO121: assetmanager listo")
	memoria("despues-del-assetmanager")

	// DC-002.120: el gestor queda a mano -- el ciclo de fases lo necesita para reiniciar la arena de los `.dt1`
	// del nivel que se deja.
	gestorDeAssets = am

	for _, ruta := range []string{rutaDatos, rutaParche} { // el parche queda PRIMERO: se registra al final
		if !registrarFuente(am, ruta) {
			return
		}
	}

	println("JUEGO121: fuentes en el loader=", len(am.Sources))
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

	println("JUEGO121: tablas cargadas=", cargadas, "de", tablasQueSeCargan)
	instrumentoDeLasFuentes("despues-de-las-tablas")
	memoria("despues-de-las-tablas")

	if soltarLecturas {
		tramosAntes := d2asset.EstadoDeLosTramos()
		d2asset.SoltarLecturasDeConsola()
		tramosDespues := d2asset.EstadoDeLosTramos()

		println("JUEGO121: LECTURAS soltadas; tramoAntes=", tramosAntes.BytesDelTramo, "pedidos=", tramosAntes.Pedidos,
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
		println("JUEGO121: JUGADOR apagado (la corrida de referencia del pueblo solo)")
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

	println("JUEGO121: UMBRAL DEL RECOLECTOR antes=", anterior, "ahora=", umbralDelRecolector,
		"semiespacio=", espacioGc, "umbralBytes=", umbralBytes, "usado=", int(d2cGcUsado()),
		"colecciones=", int(d2cGcColecciones()))

	runtime.GC()

	println("JUEGO121: RECOLECCION A PEDIDO usado=", int(d2cGcUsado()), "colecciones=", int(d2cGcColecciones()))
	memoria("despues-de-recolectar-a-pedido")

	engine := d2mapengine.CreateMapEngine(d2util.LogLevelError, am)

	// DC-002.120: el motor queda a mano igual que el gestor -- la fase le tira las casillas al nivel que se deja
	// y le compone el nuevo encima.
	motorDelMapa = engine

	// LAS CASILLAS SE REUSAN EN SU LUGAR (DC-002.120). Con el ciclo encendido el nivel cambia dos veces con los
	// cuatro jugadores dibujados encima, y las casillas de cada nivel son ~650.000 B de semiespacio que el camino
	// de siempre tira y vuelve a pedir. Reusandolas, el nivel nuevo se escribe encima del que se deja: la VUELTA
	// no pide memoria para las casillas (el crc32 del cuadro lo verifica: 1437022778 / 584380401). Con el ciclo
	// apagado la politica queda apagada y esta corrida es la de DC-002.117 byte a byte.
	if faseEncendida() {
		d2mapengine.ReusarLasCasillasEnSuLugar(reusarLasCasillas)
	}

	println("JUEGO121: CASILLAS reusarEnSuLugar=", d2mapengine.ReusarLasCasillas(), "palanca=", reusarLasCasillas)

	if _, _, _, ok := componerNivel(engine, presetA, archivoA); !ok {
		println("JUEGO121: LoadStamp devolvio nil")

		return
	}

	// EL CONJUNTO VIVO, MEDIDO (DC-002.113). Componer el pueblo deja mucha basura; una recoleccion A PEDIDO
	// deja el semiespacio en lo que de verdad esta vivo. Ese numero es el que dice si el POC entra o no: el
	// espacio copia a un semiespacio del mismo tamano (1.835.008 B), asi que el vivo tiene que quedarle bien
	// abajo o la copia se queda sin lugar ("GC to-space overflow", medido en esta tarjeta con 95 % de umbral).
	runtime.GC()
	semiespacio("despues-de-componer-y-recolectar")

	estado := d2asset.EstadoDeLosTiles()
	println("JUEGO121: ARENA DE BLOQUES usados=", estado.BloquesUsados, "bytes=", estado.BloquesBytes,
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

		println("JUEGO121: LECTURAS soltadas despues de componer; tramo=", tramos.BytesDelTramo,
			"lecturas=", tramos.Lecturas, "bytes=", int(tramos.Bytes))
		memoria("despues-de-soltar-las-lecturas-del-mapa")
	}

	// ============================== 4a. (el HUD va en 4e, ver abajo) ==============================

	// ============================== 4b. EL MONSTRUO (DC-002.121) ==============================
	// VA ACA Y EL LUGAR ES EL TRABAJO: el `.cof` y las capas `.dcc` del bicho salen del `d2data.mpq`, que es la
	// fuente que el paso 5 suelta cuando el ciclo de niveles esta apagado -- despues de esa linea el monstruo ya
	// no se puede componer. Y va DESPUES de componer el pueblo para no sumarle al pico de la composicion (que es
	// lo que midio DC-002.113) los ~50 KB de indices del bicho ni la arena del decodificador.
	memoria("bicho-antes")
	semiespacio("bicho-antes")

	cargarBichos()

	memoria("bicho-despues")
	semiespacio("bicho-despues")

	// ============================== 4c. EL DROP (DC-002.123) ==============================
	// VA ACA, Y EL LUGAR ES EL TRABAJO, por los mismos dos motivos que el bicho: las tablas del drop
	// (monstats/TreasureClassEx/weapons/armor/misc del parche) y el DC6 del objeto salen de los MPQ, que el paso 5
	// suelta; y va DESPUES de componer el pueblo para no sumarle al pico de la composicion (que es lo que midio
	// DC-002.113) ni la lectura de las tablas ni la decodificacion del flippy. Todo eso se devuelve en el acto: lo
	// que queda vivo del drop es el unico objeto que salio, en indices.
	cargarElDrop()

	// DC-002.124: la grilla del inventario (y la del alijo) con el grafico del objeto en la grilla. Va aca, con
	// las fuentes abiertas, por lo mismo que el drop: el `invfile` sale del MPQ.
	cargarElInventario()
	cargarElAlijo()

	// ============================== 4g. LA QUEST (DC-002.128) ==============================
	// VA ANTES DE LOS NPCs, Y ESO ES LO QUE LA HACE ENTRAR. La quest no pide un solo byte de grafico (lo caro, 96 B
	// del registro del guardado, no pide RAM): lo unico que lee de la fuente son los TEXTOS de las paginas de su log
	// (`string.tbl`, 390.570 B, `qstsa1q1<page>`), los mismos que lee el panel de los NPCs. Cargada DESPUES de ellos,
	// la tabla ya estaba soltada y habia que volver a leerla: una SEGUNDA copia de 390 KB en el monton de C, que es
	// justo lo que no hay (medido: la corrida moria en `antes-del-renderer` con `Out of memory. Requested sbrk_base
	// ... diff 446464` y `montonC_libre= 734240`, sin llegar al primer cuadro). Cargada ANTES, la quest es el
	// primero que pide la tabla, el cargador de los NPCs la encuentra puesta (`tablaDeStrings != nil` -> no lee nada)
	// y la suelta el al terminar, igual que en DC-002.127: el perfil de memoria de la corrida queda como el de la
	// tarjeta anterior mas los 18.460 B de indices de Akara.
	memoria("quest-antes")
	semiespacio("quest-antes")
	cargarLaQuest()
	memoria("quest-despues")
	semiespacio("quest-despues")

	// ============================== 4f. LOS NPCs Y SU DIALOGO (DC-002.127) ==============================
	// VA ANTES DEL HUD, por lo mismo que el HUD va ultimo: el semiespacio del recolector esta al filo y el HUD se
	// queda vivo el resto de la corrida. Los NPCs tambien se quedan (sus cuadros compuestos), pero van ANTES para
	// que el pico del HUD sea el ultimo. Los `.cof`, las `.dcc` y `string.tbl` salen del `d2data.mpq`, que el paso 5
	// suelta cuando el ciclo esta apagado: despues de esa linea no se puede componer ninguno.
	memoria("npcs-antes")
	cargarLosNPCs()
	memoria("npcs-despues")

	// ============================== 4f. EL MINIMAPA (DC-002.130) ==============================
	// VA ACA, EN LA MISMA VENTANA QUE EL HUD (las fuentes y las tablas siguen abiertas: el paso 5 las suelta) y
	// ANTES del HUD, que es la diferencia con el orden que pide el sentido comun.
	// POR QUE ANTES DEL HUD (medido en esta tarjeta, no supuesto): el HUD se lleva el ultimo tramo del semiespacio
	// (en DC-002.125 dejo el recolector al 96 %) y su fuente de 35.214 B tarda 16 s en decodificarse con el heap
	// casi lleno. Con el minimapa DELANTE, lo que crece es su tabla FILTRADA (6.176 B, medido) y su modelo (43.800 B
	// en el host), y el arte va al monton de C (7.552 B): el minimapa se carga con el semiespacio mas holgado y el
	// HUD, que es el que se queda vivo con menos bytes, cierra la carga.
	cargarElMinimapa()

	// ============================== 4e. EL HUD (DC-002.125) ==============================
	// VA ACA, EN EL ULTIMO MOMENTO EN QUE LAS FUENTES ESTAN ABIERTAS Y DESPUES DE TODO LO DEMAS. Las dos cosas son
	// el trabajo:
	//   - DESPUES DE TODO LO DEMAS. El HUD se queda vivo (~23 KB con las piezas esenciales, medido) y el POC esta al
	//     filo del semiespacio: cargado antes del bicho, que necesita ~280 KB de transitorios, la corrida moria con
	//     `fatal error: out of memory` en el inventario (semiespacio 1.689.880 de 1.835.008 B = 92 %). Cargado aca,
	//     los picos del bicho, del drop y del inventario ya pasaron.
	//   - ANTES DEL PASO 5. Con `faseKnob=apagado` el paso 5 suelta las fuentes y las tablas, y despues de esa
	//     linea el HUD se queda sin un solo grafico (medido: "HUD SIN GRAFICO" en las diez piezas y cuadros de 0
	//     pixeles).
	cargarElHUD()

	// ============================== 5. LO QUE EL CUADRO NO LEE (DC-002.105) ==============================
	// DC-002.120: CON EL CICLO ENCENDIDO NO SE SUELTA NADA. La fase siguiente tendria que reabrir las dos fuentes
	// y recargar las tablas, y eso son +120.520 B de vivos en el semiespacio por fase (medido en DC-002.111: la
	// VUELTA moria ahi). Con las fuentes y las tablas vivas la fase no reabre nada y el ciclo entra. La corrida de
	// referencia (`faseKnob=apagado`) sigue soltando todo, como DC-002.117.
	if soltarEnElCiclo || !faseEncendida() {
		soltarLoQueElCuadroNoLee(am)
	} else {
		println("JUEGO121: CICLO encendido -- NO se sueltan las tablas ni las fuentes despues de componer (la fase",
			"siguiente no tiene nada que reabrir; es la palanca de DC-002.111)")
	}

	// ============================== 6. LO QUE SE SUELTA ANTES DE DIBUJAR ==============================
	switch orden {
	case "tarjeta":
		println("JUEGO121: ORDEN tarjeta -- se suelta el arena ENTERO antes de armar la cache (contra el contrato)")
		tramos, bytes := d2asset.SoltarArenaDeBloques()
		println("JUEGO121: ARENA DE BLOQUES SOLTADA (antes del renderer) tramos=", tramos, "bytes=", bytes)
		memoria("despues-de-soltar-el-arena")
	case "prestamo":
		println("JUEGO121: ORDEN prestamo -- se suelta SOLO el buffer prestado de lectura (el arena de bloques queda)")
		bytes := d2asset.SoltarPrestamoDeLecturaDeTiles()
		println("JUEGO121: PRESTAMO DE LECTURA SOLTADO bytes=", bytes)
		memoria("despues-de-soltar-el-prestamo")
	default:
		println("JUEGO121: ORDEN contrato -- el arena queda vivo hasta que la cache este armada")
	}

	// ============================== 7. EL RENDERER (CACHE EN INDICES) ==============================
	memoria("antes-del-term")

	term, err := d2term.NewTerminal()
	if err != nil {
		println("JUEGO121: NewTerminal FALLO:", err.Error())

		return
	}

	memoria("despues-del-term")

	memoria("antes-del-renderer")
	vram("antes-del-renderer")

	opciones := []d2maprenderer.Opcion{
		d2maprenderer.ComoConsola(ancho, alto),
		d2maprenderer.CacheDeTilesPerezosa(),
		d2maprenderer.CacheDeTilesEnIndices(),
	}

	if tramoDePixeles != nil {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramo(tramoDePixeles))
		println("JUEGO121: TRAMO DE LA CACHE en uso (reservado al arranque) bytes=", tramoDePixeles.Bytes())
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 && cacheEnVram {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnBloquesDe(tramoDeLaCache, tramoEnPedazos,
			pvr.VramReservar, "vram", sondaDelTramoDeVram()...))
		println("JUEGO121: TRAMO DE LA CACHE EN BLOQUES DE VRAM techo=", tramoDeLaCache, "pedazo=",
			tramoEnPedazos, "pedazoLoDiceLaVram=", pedazoLoDiceElMonton, "vramDisponible=", pvr.VramDisponible(),
			"vramServiriaDeUnGolpe=", pvr.VramServible(tramoDeLaCache))
	} else if tramoDeLaCache > 0 && tramoEnPedazos > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnTramoEnPedazos(tramoDeLaCache, tramoEnPedazos,
			sondaDelTramo()...))
		println("JUEGO121: TRAMO DE LA CACHE EN PEDAZOS techo=", tramoDeLaCache, "pedazo=", tramoEnPedazos,
			"pedazoLoDiceElMonton=", pedazoLoDiceElMonton)
	} else if tramoDeLaCache > 0 && cacheEnVram {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramoDe(tramoDeLaCache, pvr.VramReservar, "vram"))
		println("JUEGO121: TRAMO DE LA CACHE EN UN GOLPE DE VRAM bytes=", tramoDeLaCache,
			"vramDisponible=", pvr.VramDisponible())
	} else if tramoDeLaCache > 0 {
		opciones = append(opciones, d2maprenderer.CacheDeTilesEnUnTramo(tramoDeLaCache))
		println("JUEGO121: TRAMO DE LA CACHE pedido TARDE (en el renderer) bytes=", tramoDeLaCache)
	}

	rendererDelMapa = d2maprenderer.CreateMapRenderer(am, renderer, engine, term, d2util.LogLevelError,
		celdaX, celdaY, opciones...)

	// DC-002.135: el atajo de la cache del mapa: con la imagen ya armada no se le piden los datos del tile al
	// motor. Es la palanca del A/B contra el camino de DC-002.134 (que pedia `GetTiles` en cada tile y cuadro).
	rendererDelMapa.ActivarCacheSinGetTiles(cacheRapida135 == "si")

	espacio := rendererDelMapa.TamanoDelCuadro()
	println("JUEGO121: renderer listo; cuadro=", espacio.W, espacio.H,
		"cachePerezosa=", rendererDelMapa.CachePerezosa(), "cacheEnIndices=", rendererDelMapa.CacheDeIndices(),
		"cacheSinGetTiles=", rendererDelMapa.CacheSinGetTiles())
	censoDelTramo("despues-del-renderer")
	memoria("despues-del-renderer")
	semiespacio("despues-del-renderer")

	// ============================== 8. EL JUGADOR Y SU VELOCIDAD ==============================
	mov = NuevoMovimiento(celdaX, celdaY)

	println("JUEGO121: MOTOR velocidad=subtilesPorSegundo=", mov.Est.Velocidad,
		"celdasPorSegundo=", mov.Est.Velocidad/float64(subtilesPorCelda),
		"ticksPorSegundo=", 25, "segundosPorTick=", segundosPorTick)

	direcciones := DireccionesDelControl()
	println("JUEGO121: DIRECCIONES de la cruceta", len(direcciones), "lista=", len(direcciones))

	for _, d := range direcciones {
		println("JUEGO121: DIRECCION de la cruceta dir=", d)
	}

	println("JUEGO121: CONTROL hay=", maple.HayControl(), "servicio=", servicio != nil,
		"inyectarGuion=", inyectarGuion, "guionTramos=", len(guionInyectado), "guionSegundos=",
		SegundosDelGuion(pausaEntreTramos))

	if inyectarGuion {
		println("JUEGO121: GUION INYECTADO -- el estado del maple lo pone el SHIM (d2c_control_inyectar):",
			"NO hay mando fisico en el bus, la evidencia lo dice")
	}

	// ============================== 8b. LOS JUGADORES DEL COOPERATIVO ==============================
	// El jugador 1 es `mov` (el de DC-002.113, en el puerto 0 del bus) y el jugador 2, si la palanca lo pide, el del
	// puerto 1. Aca se decide, PUERTO POR PUERTO, si hay aparato o si el estado lo pone el guion, y se imprime.
	elegirGuion() // DC-002.115: el guion de la corrida (coop | leash), antes de imprimir sus segundos

	println("JUEGO121: COOP jugadores=", jugadoresKnob, "puertos=", puertoJugador1, puertoJugador2,
		puertoJugador3, puertoJugador4, "celdasDeArranque=", celdaX, celdaY, celdaX+celdaP2X, celdaY+celdaP2Y,
		celdaX+celdaP3X, celdaY+celdaP3Y, celdaX+celdaP4X, celdaY+celdaP4Y,
		"compartirJugador2=", compartirJugador2, "guionCoopSegundos=", segundosDelGuionCoop(),
		"guionPoc=", guionEnUso, "guionJugador2=", guionJugador2, "cuadros=", cuadros,
		"leashDeGrupo=", leashDeGrupo, "split=", splitKnob)

	prepararJugadores()

	// DC-002.138: EL ESCENARIO DEL MVP JUGABLE. Va aca: despues de que el pueblo, el bicho, el drop, el inventario,
	// el alijo, la quest y los NPCs estan cargados (necesita sus celdas) y antes de que el ciclo del PVR pida el
	// primer cuadro (las palancas de las otras tarjetas tienen que estar armadas antes). Con `MVP=no` no hace nada.
	arrancarElMVP()

	// DC-002.115: la medida del leash, con los cuadros ya compuestos y los dos jugadores armados (necesita la caja
	// peor de los ciclos, que recien existe aca).
	//
	// DC-002.116: el SPLIT se arma antes, y con el split armado el leash no se mide ni se aplica: son los dos
	// caminos del cooperativo (una pantalla con leash | dos pantallas) y esta corrida dice cual corrio.
	if prepararElSplit() {
		println("JUEGO121: LEASH no se aplica con el split (dos camaras, un lienzo por jugador);",
			"el leash es el camino de SPLIT=no")
	} else {
		medirElLeashAlArranque()
	}

	msAnterior = int64(d2cMs())

	// DC-002.136: EL CIERRE DE LA TABLA DE CONSUMO, justo antes del primer cuadro: aca ya se cargo TODO (pueblo,
	// jugador, bicho, drop, quest, NPCs, minimapa, HUD, term, renderer) y el monton de C esta en su punto mas alto.
	tablaDeLaCarga()

	// (el HUD ya se cargo en 4a, con las fuentes todavia abiertas)

	// ============================== 9. EL CUADRO EN PANTALLA ==============================
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast) DC-002.117"); err != nil {
		println("JUEGO121: el ciclo de cuadros salio con error:", err.Error())
	}

	println("JUEGO121: FIN pico del monton=", picoMonton, "de arena=", d2cMontonArena(),
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
	rutaDelPueblo = stamp.RegionPath()
	enArena := stamp.TilesDelDS1EnArena()
	bloque := stamp.BytesDeLaArenaDelDS1()

	// DC-002.120: el tramo de las casillas se pide UNA vez, aca, que es la primera vez que hay un `.ds1` en la mano
	// para saber cuantas capas pide cada casilla.
	reservarElArenaDeLasCasillas(stamp, tamano.Width, tamano.Height)

	runtime.GC()

	engine.ResetMapParaStamp(regionPueblo, tamano.Width, tamano.Height, stamp)
	engine.PlaceStampClippedWhere(stamp, 0, 0, tamano.Width, tamano.Height, func(int) bool { return false }, false)

	soltados := stamp.SoltarLosTilesDelDS1()

	pisos, paredes, crc = censo(engine)

	println("JUEGO121: MAPA tiles=", len(*engine.Tiles()), "pisos=", pisos, "paredes=", paredes,
		"crcGrilla=", int(crc), "ms=", int(d2cMs()-t0), "ds1enArena=", enArena, "bloque=", bloque,
		"ds1Soltado=", soltados, "w=", tamano.Width, "h=", tamano.Height)

	return pisos, paredes, crc, true
}

// soltarLoQueElCuadroNoLee (DC-002.105): devuelve al monton de C las dos piezas que el CUADRO NO USA.
func soltarLoQueElCuadroNoLee(am *d2asset.AssetManager) int {
	if soltarDespuesDeComponer == "nada" {
		println("JUEGO121: no se suelta nada despues de componer")

		return 0
	}

	total := 0

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		precargarLoQueElCuadroLee(am)
	}

	if soltarDespuesDeComponer == "tablas" || soltarDespuesDeComponer == "todo" {
		memoria("antes-de-soltar-las-tablas")

		res := am.SoltarFilasDeLasTablas(tablasCargadas)

		println("JUEGO121: TABLAS SOLTADAS tablas=", res.Tablas, "tramos=", res.Tramos, "bytes=", res.Bytes,
			"vacias=", len(res.Vacias), "noSoportadas=", len(res.NoSoportadas))

		total += res.Bytes

		memoria("despues-de-soltar-las-tablas")
	}

	if soltarDespuesDeComponer == "fuentes" || soltarDespuesDeComponer == "todo" {
		for _, f := range fuentesDeConsola {
			r := f.Soltar(func(tramo uintptr, bytes int) int { return d2cSoltarTramo(tramo, bytes) })

			println("JUEGO121: FUENTE SOLTADA", r.Ruta, "tablas=", r.Tablas.Total,
				"enElMontonDeC=", r.Tablas.Externas, "bytesTablas=", r.BytesTablas,
				"bytesTrabajo=", r.BytesTrabajo, "bytes=", r.Bytes, "cedido=", r.Cedido,
				"yaSoltada=", r.YaSoltada)

			total += r.Bytes
		}

		memoria("despues-de-soltar-las-fuentes")
	}

	println("JUEGO121: SOLTADO despues de componer bytes=", total, "libre=", d2cMontonLibre(),
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
		println("JUEGO121: no se pudo precargar la paleta del acto:", err.Error())
	}

	if _, err := am.LoadPaletteTransform(pl2); err != nil {
		println("JUEGO121: no se pudo precargar el PL2 del acto:", err.Error())
	}

	println("JUEGO121: PRECARGADO lo que el cuadro lee del MPQ", paleta, pl2, "delta=", d2cMontonUsado()-usado)
}

// registrarFuente abre el MPQ con la Source de consola, le inyecta el `free` del shim y la pone PRIMERA en el
// loader del motor (DC-002.96/.99).
func registrarFuente(am *d2asset.AssetManager, ruta string) bool {
	f, err := fuenteconsola.Abrir(ruta)
	if err != nil {
		println("JUEGO121: MPQ-ABRIR FALLO:", ruta, err.Error())

		return false
	}

	f.UsarLiberador(fuenteconsola.Liberador(d2cLibre))
	f.RegistrarEn(am.Loader)

	println("JUEGO121: fuente registrada", ruta)
	fuentesDeConsola = append(fuentesDeConsola, f)

	return true
}

// censoDelTramo (DC-002.104): donde viven los pixeles de la cache.
func censoDelTramo(etapa string) {
	if rendererDelMapa == nil {
		return
	}

	// DC-002.134: la linea del tramo es evidencia POR CUADRO y son ~200 B por el puerto serie: con `VERBOSO=no`
	// se calla (los tres primeros cuadros y los del cierre la siguen teniendo).
	if !verboso134() {
		return
	}

	e := rendererDelMapa.CensoDelTramo()
	println("JUEGO121: TRAMO", etapa, "activa=", e.Activa, "origen=", e.Origen, "bytes=", e.Bytes, "usado=", e.Usado,
		"libre=", e.Libre, "imagenes=", e.Imagenes, "derrames=", e.Derrames, "bytesDerramados=", e.BytesDerramados,
		"enPedazos=", e.EnPedazos, "pedazos=", e.Pedazos, "pedazoMenor=", e.PedazoMenor,
		"pedazoMayor=", e.PedazoMayor, "techo=", e.Techo)
}

// instrumentoDeLasFuentes: el censo de la fuente (pedidos, liberados, vivos, prestamo) para el log.
func instrumentoDeLasFuentes(etapa string) {
	for _, f := range fuentesDeConsola {
		e := f.Estado()
		println("JUEGO121: FUENTE", etapa, f.Path(), "abiertos=", e.Abiertos, "enBuffer=", e.LecturasEnBuffer,
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
		println("JUEGO121: TABLA FALLO (fuente):", ruta, err.Error())

		return false
	}

	println("JUEGO121: TABLA fuente", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

	return true
}

// cargarTablaPorStream: el camino de HOY (`LoadDataDictionary` por stream del MPQ), para el A/B.
func cargarTablaPorStream(am *d2asset.AssetManager, ruta string) bool {
	usado := d2cMontonUsado()
	tamano, _ := am.TamanoDeArchivo(ruta)

	dict, err := am.LoadDataDictionaryPorStream(ruta)
	if err != nil {
		println("JUEGO121: TABLA FALLO (stream):", ruta, err.Error())

		return false
	}

	if err := am.Records.Load(ruta, dict); err != nil {
		println("JUEGO121: TABLA FALLO (stream, records):", ruta, err.Error())

		return false
	}

	println("JUEGO121: TABLA stream", ruta, "bytes=", tamano, "montonDelta=", d2cMontonUsado()-usado)

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

	// DC-002.134: lo que cuesta ACTUALIZAR (control, movimiento, camara y sus lineas de evidencia). Es la otra
	// mitad del "trabajo" del PVR: `trabajo = actualizar + dibujar`.
	tInicio134 := cronometro134()

	defer func() { anotarActualizar134(cronometro134() - tInicio134) }()

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

	// DC-002.120: LOS CUADROS DEL CICLO SON QUIETOS A PROPOSITO. En las fases los cuatro se ponen en la formacion
	// alrededor de la celda del POC y la camara en ESA celda (es la unica con la que el cuadro del mapa da el
	// crc32 del host). Si el guion siguiera empujando, los cuatro se irian de la formacion y la camara del centro
	// del grupo correria el cuadro: no se moveria nada de lo que la fase mide. Se avanza el reloj y nada mas.
	if faseEncendida() && cuadro >= cuadros {
		reloj += paso

		return nil
	}

	// ============================== 1. EL CONTROL ==============================
	// DC-002.138: EL GUION DEL MVP JUGABLE, ANTES DE TODO. Lo que arme esta etapa tiene que valer en ESTE cuadro:
	// las palancas de las otras tarjetas las leen sus pasos (`pasoDelInventario`, `pasoDelAlijo`, `pasoDeLaQuest`)
	// unas lineas mas abajo, y los bits de la cruceta los lee `leerYAvanzarJugadores`. Con la palanca apagada no
	// hace nada.
	pasoDelMVP()

	// La corrida del criterio lee el control DEL EMULADOR (el teclado virtual del host aprieta la cruceta y Flycast
	// arma el estado del maple). Con `inyectarGuion` el estado lo pone el shim, y se dice.
	if inyectarGuion {
		// el estado del maple lo pone el shim: el servicio lo lee igual que a un control de verdad
		servicio.InyectarBotones(BotonesDelGuion(reloj, pausaEntreTramos))
	}

	// ============================== 2. EL MOVIMIENTO (UN JUGADOR POR PUERTO) ==============================
	// La celda de ANTES se toma antes de mover: el tramo tiene que arrancar donde estaba el jugador, no donde
	// quedo (si no, `celdaIni` y `celdaFin` de un tramo salen iguales y el tramo no dice nada del camino).
	celdaAntesX, celdaAntesY := mov.Est.Celda()

	movidoP1, caminando := leerYAvanzarJugadores(paso)

	// ============================== 2b. EL LEASH (DC-002.115) ==============================
	// Va DESPUES de mover y ANTES de la camara: el que se alejo mas de la distancia medida se queda en el borde,
	// y recien despues la camara se pone en el centro del grupo que quedo. Con el leash la camara no necesita
	// ninguna cuenta nueva: si nadie se aleja mas de lo que entra en el lienzo, el centro del grupo los deja a
	// los dos adentro.
	//
	// Y VA ANTES DE LAS MEDIDAS: el leash es parte del movimiento, asi que los tramos tienen que ver la posicion
	// y los subtiles DE DESPUES del recorte (si no, un tramo cierra con la posicion que el jugador queria y no con
	// la que el leash le permitio, y la separacion de ese renglon no seria la real).
	//
	// DC-002.116: con el split NO se aplica. El split es la alternativa al leash cuando la separacion no entra
	// en una pantalla: cada jugador se va con su camara y no hay distancia que recortar.
	if !estaElSplitActivo() {
		aplicarLeash()
	}

	if movidoP1 {
		medirTramo(jugadores[0].EjeX, jugadores[0].EjeY, paso, jugadores[0].Caminado, celdaAntesX, celdaAntesY)
	} else {
		cerrarTramo()
	}

	// la medida del cooperativo: los ticks y las celdas de LOS DOS en cada tramo del guion (es lo que dice si el
	// input de un jugador movio al otro)
	medirTramoDeLosDos(paso)

	// ============================== 2c. LOS MONSTRUOS (DC-002.121) ==============================
	// VA DESPUES DE MOVER A LA GENTE Y ANTES DE LA CAMARA: el bicho patrulla con su propio reloj y su velocidad
	// (la de su fila de monstats), y su celda tiene que estar puesta cuando el cuadro se pinta.
	//
	// DC-002.122: EL GOLPE VA ANTES DEL PASO DEL BICHO. El ataque se resuelve con las posiciones de este cuadro
	// (despues de mover a la gente), y si el golpe lo mata, el bicho ya no patrulla en este mismo cuadro (`pasoDeUnBicho`
	// se planta con el modo DT/DD) y el cadaver deja de frenar a los jugadores a partir del proximo.
	if len(bichos) > 0 {
		if combateKnob == "si" && combateOK {
			combateDelCuadro(ahora)
		}

		pasoDeLosBichos(paso)
		contarSolapamientos()

		// DC-002.123: EL DROP Y LA RECOGIDA. Va DESPUES del combate (el objeto aparece cuando el golpe deja al
		// bicho sin vida) y DESPUES de mover a la gente (la recogida se mide sobre el paso que acaba de dar cada
		// jugador, no sobre una posicion vieja). El cadaver ya no ocupa celda (DC-002.122), asi que el jugador
		// puede caminar hasta donde cayo.
		soltarLoQueDejoElBicho()
	}

	// la recogida va FUERA del if del bicho: el objeto puede quedar en el piso de una corrida a la siguiente (con
	// MONSTRUOS=0 y DROP=si el objeto no existe, pero el camino tiene que estar cerrado igual).
	if dropKnob == "si" && len(suelo) > 0 {
		recogerLosItems()
	}

	// ============================== 2d. EL INVENTARIO Y EL ALIJO (DC-002.124) ==============================
	// VAN DESPUES DE LA RECOGIDA: el objeto que se recoge en este cuadro entra a la grilla en este mismo cuadro
	// (el jugador lo ve en su celda cuando el panel se abre), y el alijo se abre solo si el jugador esta cerca.
	pasoDelInventario()
	pasoDelAlijo()

	// ============================== 2e. LOS NPCs Y EL DIALOGO (DC-002.127) ==============================
	// VA DESPUES DE MOVER A LA GENTE: la distancia que decide si se le puede hablar es la de este cuadro, no la
	// de la celda de antes.
	pasoDelNPC()

	// ============================== 2f. LA QUEST (DC-002.128) ==============================
	// VA DESPUES DE LOS NPCs, porque el aceptar y el cobrar son HABLAR: la distancia que decide si se le puede
	// hablar es la de este cuadro. Y va ANTES DE LA CAMARA porque el guion de la quest pone al jugador en la celda
	// de Akara o al lado del bicho (el instrumento que reemplaza al teclado) y la camara tiene que seguirlo en el
	// mismo cuadro.
	pasoDeLaQuest()

	// ============================== 3. LA CAMARA ==============================
	// DC-002.116: con el split hay UNA camara por jugador (la del grupo no sirve para dos pantallas). Se ponen
	// aca, despues del movimiento y antes de dibujar; y el que las usa de verdad es cada mitad al dibujarse.
	if estaElSplitActivo() {
		ponerLasCamarasDelSplit()
	} else {
		seguirConLaCamara()
	}

	// ============================== 4. EL RELOJ DEL GUION ==============================
	reloj += paso

	// LA RECOLECCION A PEDIDO NO VA EN EL CUADRO. Medido en esta tarjeta: pedirle un runtime.GC() al
	// recolector desde el callback del ciclo del PVR CLAVA el ciclo (dos corridas quedaron en el cuadro 7 y no
	// volvieron a presentar). El recolector entra por umbral solo, que es lo que ya hace, y con el conjunto
	// vivo medido (1.328.120 B, 72 %) le queda lugar de sobra. El umbral alto de knobs.go es la palanca.

	if cuadro <= 1 || movidoP1 || caminando > 0 {
		camX, camY := 0.0, 0.0

		if p := rendererDelMapa.Camera.GetPosition(); p != nil {
			camX, camY = p.X(), p.Y()
		}

		cx, cy := mov.Est.Celda()

		// DC-002.134: estas tres lineas son ~600 B por cuadro en los cuadros que el guion se mueve, que son la
		// mayoria. Con `VERBOSO=no` se callan (los tres primeros cuadros las siguen teniendo).
		if verboso134() {
			println("JUEGO121: CONTROL cuadro=", cuadro+1, "lectura=", jugadores[0].Servicio.HayLectura(),
				"botones=", int(jugadores[0].Botones),
				"ejes=", int(jugadores[0].EjeX), int(jugadores[0].EjeY), "dir=", mov.Direccion,
				"caminando=", mov.Caminando, "celda=", cx, cy, "ticks=", jugadores[0].Ticks,
				"subtiles=", jugadores[0].Caminado, "camara=", camX, camY, "reloj=", reloj,
				"guionBotones=", int(BotonesDelGuion(reloj, pausaEntreTramos)))

			println("JUEGO121: COOP CONTROL cuadro=", cuadro+1, "jugadores=", len(jugadores),
				leerElControlDeLosDos())

			// DC-002.121: lo que hizo cada bicho en este cuadro (celda, direccion, cuadro del ciclo, extremos).
			if len(bichos) > 0 {
				println("JUEGO121: BICHO cuadro=", cuadro+1, "bichos=", len(bichos), resumenDeLosBichos())
			}
		}
	}

	return nil
}

// caminoCambio: si este cuadro hubo movimiento o si es el primero (para no imprimir 60 lineas iguales).
// (DC-002.114: la condicion paso a estar en el cuerpo de `actualizar`, porque ahora son DOS jugadores los que
// pueden estar caminando; se deja la funcion porque el tramo del jugador 1 la sigue usando como criterio.)
func caminoCambio(ticks int, caminado float64) bool { return ticks > 0 || caminado > 0 }

// seguirConLaCamara: la camara del motor se pone en el CENTRO de los jugadores (con uno solo, en el jugador 1: es lo
// mismo que hace `d2gamescreen` con el heroe, `MoveCameraTo(WorldToOrtho(posicion))`). Con `camara=fija` no se toca:
// es la corrida que deja ver al jugador cruzando la pantalla.
func seguirConLaCamara() {
	if camara != "siguiendo" || mov == nil || mov.Est == nil {
		return
	}

	// DC-002.114: el centro del grupo. Es lo que mantiene a los DOS en una pantalla de 640x480 sin partirla.
	cx, cy := centroDeLosJugadores()

	rx, ry := rendererDelMapa.WorldToOrtho(cx, cy)
	pos := d2vector.NewPosition(rx, ry)

	rendererDelMapa.MoveCameraTo(&pos)

	camaraCeldaX, camaraCeldaY = cx, cy
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

	println("JUEGO121: TRAMO", t.Nombre, "dir=", t.Dir, "cuadros=", t.Cuadros, "desdeElCuadro=", t.CuadroIni,
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

	// DC-002.116: el cuadro partido en dos es otro camino (dos camaras, dos lienzos); el de abajo es el de
	// DC-002.115 y no cambia una linea.
	if estaElSplitActivo() {
		return dibujarElSplit(superficie)
	}

	cuadro++

	// DC-002.132: el reloj del cuadro para el informe de los cuadros por segundo (ver instrumento_132.go).
	tInicio132 := int64(d2cMs())

	// DC-002.134: el mismo cuadro en microsegundos, para el cronometro por etapa (ver instrumento_134.go). La
	// resta de `dibujar` menos la suma de las etapas es lo que el cuadro paga en lineas de evidencia.
	tInicio134 := cronometro134()

	// DC-002.120: con el ciclo encendido, los cuadros de DESPUES del guion son las tres fases del cambio de nivel
	// (`fases_120.go`). El camino de siempre (los tres cuadros del cierre) queda intacto para `faseKnob=apagado`.
	if faseEncendida() && cuadro > cuadros {
		// DC-002.133: EL RESUMEN DE LA ANIMACION SALE ANTES DEL CICLO. Con el ciclo de niveles encendido, el cuadro
		// de despues del guion se lo lleva la transicion y el cierre de siempre (`cuadro == cuadros+3`, donde estaban
		// los resumenes de las otras tarjetas) no llega nunca. El guion ya termino: los numeros estan completos.
		if !resumenDeLaAnimacionHecho {
			resumenDeLaAnimacionHecho = true
			resumenDeLaAnimacion133()
		}

		return dibujarElCiclo(superficie)
	}

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

	// DC-002.134: las lineas de evidencia POR CUADRO se apagan con `VERBOSO=no` (verboso134). Los tres
	// primeros cuadros las llevan siempre: ahi vive la comparacion del mundo contra el host.
	if verboso134() {
		println("JUEGO121: CUADRO", cuadro, "ARRANCA libre=", d2cMontonLibre())
	}

	malla, esPvr := superficie.(*pvr.Surface)
	if !esPvr {
		println("JUEGO121: el Renderer no entrego una Surface del PVR")

		return nil
	}

	if cuadro == 1 {
		println("JUEGO121: PRIMER-CUADRO cuadro=1 celda=", celdaX, celdaY, "camara=", camara)
	}

	t0 := d2cMs()
	tEtapa134 := cronometro134()

	// DC-002.134: cuantos pixeles de este cuadro van por cada camino del backend (ver `anotarLosPixeles134`).
	pvr.ReiniciarContadoresDePixeles()

	rendererDelMapa.Render(superficie)

	pxDeIndices, pxDeCopiar, pxDeSombras := pvr.ContadoresDePixeles()

	anotarLosPixeles134(pxDeIndices, pxDeCopiar, pxDeSombras)

	anotarEtapa134(e134Mapa, tEtapa134)

	msMapa := int(d2cMs() - t0)
	anotarLaComposicion132(int64(msMapa))

	// DC-002.132: lo que el mapa DIBUJO en este cuadro (pisos, paredes, paredes de arriba, techos, sombras y
	// entidades). Es el otro numero que dice donde se va el cuadro: sin el, "el mapa cuesta 700 ms" no dice si
	// son 200 tiles de 160x160 o 600 chicos.
	if cuadro <= 3 || cuadro%cuadrosDelInforme132 == 0 {
		println("JUEGO132: MAPA cuadro=", cuadro, "ms=", msMapa, "stats=", rendererDelMapa.DrawStats().String(),
			"blitRapido=", blitRapido, "medirCuadro=", medirCuadro)

		// DC-002.135: el censo por bloque DENTRO del renderer del mapa (cache, vista, estado, pintado y las
		// pasadas). Va pegado a la linea del mapa a proposito: se lee como su desglose.
		informeDeLosTiles135()
	}

	pixeles := malla.Imagen().Pixeles()

	llenos := llenosLienzo(pixeles)
	crcMapa := crcLienzo(pixeles)

	if cuadro == 1 {
		crcDelMapaSolo = crcMapa
		crcCuadro1Mapa = crcMapa
		llenosCuadro1 = llenos
	}

	if verboso134() {
		println("JUEGO121: CUADRO", cuadro, "SOLO-MAPA pixelesNoVacios=", llenos, "de", ancho*alto,
			"crc32=", int(crcMapa), "hostPixeles=", pixelesHost, "hostCrc=", int(crcHost), "ms=", msMapa,
			"igualAlHost=", crcMapa == crcHost, "camaraCelda=", celdaDeLaCamaraX(), celdaDeLaCamaraY(),
			"jugadores=", len(jugadores))
	}

	msJugador := 0
	crcConJugador := crcMapa
	cambiados := 0
	anclaX, anclaY := 0, 0
	dirDibujada := mov.Direccion
	cuadroAnim := -1

	// ============================== LOS JUGADORES (DC-002.114) ==============================
	// Se guarda lo que hay debajo de la caja de CADA jugador, se pintan los dos en orden de profundidad, se mide el
	// crc32 del cuadro con los dos, se devuelve el mapa (en orden inverso) y se comprueba que vuelva al crc32 del
	// mapa solo. Con `JUGADORES=1` esto es el camino de DC-002.113 con un solo participante.
	if jugadorOK && len(jugadores) > 0 {
		var msDos int

		tEtapa134 = cronometro134()

		crcConJugador, msDos = dibujarAJugadores(malla, pixeles)
		msJugador = msDos

		// el crc del mapa tiene que volver: si no vuelve, las cajas se pisaron mal y el numero de abajo no vale
		crcRestaurado := crcLienzo(pixeles)

		if crcRestaurado != crcMapa {
			println("JUEGO121: CUADRO", cuadro, "MAPA-RESTAURADO DISTINTO crc32=", int(crcRestaurado),
				"mapaSolo=", int(crcMapa))
		}

		anclaX, anclaY = jugadores[0].AnclaX, jugadores[0].AnclaY
		cambiados = jugadores[0].Cambiados
		cuadroAnim = jugadores[0].CuadroDelCiclo

		if cuadro == 1 {
			anclaCuadro1X, anclaCuadro1Y = anclaX, anclaY

			ciclo := cicloDelJugador(mov.Direccion, mov.Caminando)
			if ciclo != nil {
				cu := mov.CuadroDeLaAnimacion(ciclo)
				if cu != nil {
					caja := jugadores[0].Caja

					println("JUEGO121: JUGADOR ancla=", anclaX, anclaY, "caja=", caja.X0, caja.Y0, caja.X1, caja.Y1,
						"tamano=", caja.Ancho(), "x", caja.Alto(), "compuesto=", cu.Ancho, "x", cu.Alto,
						"esquina=", cu.Izq, cu.Arriba, "pixeles=", cu.PixelesNoVacios, "crc32=", int(cu.CRC),
						"modo=", ciclo.Modo, "dir=", ciclo.Dir, "cuadroDelCiclo=", cuadroAnim,
						"capasDibujadas=", cu.CapasDibujadas, "sinGrafico=", cu.CapasSinGrafico)
				}
			}
		}

		// la evidencia POR JUGADOR: donde cayo cada uno, con que cuadro del ciclo y cuantos pixeles pinto
		if verboso134() {
			for i, p := range jugadores {
				cx, cy := p.Mov.Est.Celda()

				ciclo, cuadroP := cicloYCuadroDelJugador(p, i)
				crudelCiclo := uint32(0)

				if ciclo != nil && cuadroP != nil {
					crudelCiclo = cuadroP.CRC
				}

				println("JUEGO121: CUADRO", cuadro, "JUGADOR nombre=", p.Nombre, "puerto=", p.Puerto,
					"inyecta=", p.Inyecta, "ancla=", p.AnclaX, p.AnclaY,
					"caja=", p.Caja.X0, p.Caja.Y0, p.Caja.X1, p.Caja.Y1,
					"tamano=", p.Caja.Ancho(), "x", p.Caja.Alto(), "pixelesPintados=", p.Cambiados,
					"crc32DelCiclo=", int(crudelCiclo), "dir=", p.Mov.Direccion, "caminando=", p.Mov.Caminando,
					"cuadroDelCiclo=", p.CuadroDelCiclo, "ms=", p.MsDibujo, "celda=", cx, cy)
			}
		}

		// DC-002.115: LA VIGILANCIA DEL LIENZO. Va aca, con las anclas y las cajas que se acaban de calcular con
		// la camara de verdad: es el numero del criterio (que ninguno se salga) cuadro por cuadro.
		leashDelCuadro(cuadro)

		// y se vuelven a pintar los dos: es el cuadro que queda en pantalla
		volverAPintarAJugadores(malla)

		anotarEtapa134(e134Jugadores, tEtapa134)
	}

	// ============================== LOS MONSTRUOS (DC-002.121) ==============================
	// Se pintan DESPUES de medir el cuadro con los jugadores y antes del crc32 del cierre: lo que se mide aca es
	// (a) que el bicho pinta pixeles en su celda, (b) cuanto cuesta pintarlo y (c) que el mapa, una vez
	// devueltas TODAS las cajas, sigue dando el crc32 del host (1437022778).
	crcConBichos := crcConJugador
	msBicho, pxBicho := 0, 0

	if len(bichos) > 0 {
		var ms int

		tEtapa134 = cronometro134()

		// el crc32 del lienzo ANTES de tocar los bichos: es contra el que se comprueba que las cajas se
		// devolvieron bien (el mapa solo ya se midio arriba, en la linea SOLO-MAPA, y sigue siendo 1437022778).
		crcAntesDeLosBichos := crcLienzo(pixeles)

		crcConBichos, ms, pxBicho = dibujarLosBichos(malla, pixeles)
		msBicho = ms

		crcDespues := crcLienzo(pixeles)
		if crcDespues != crcAntesDeLosBichos {
			println("JUEGO121: CUADRO", cuadro, "LIENZO-RESTAURADO-TRAS-BICHOS DISTINTO crc32=",
				int(crcDespues), "antesDeLosBichos=", int(crcAntesDeLosBichos))
		}

		if verboso134() {
			println("JUEGO121: BICHO CUADRO", cuadro, "bichos=", len(bichos), "pixelesPintados=", pxBicho,
				"msBicho=", msBicho, "crcConBichos=", int(crcConBichos), "mapaSolo=", int(crcMapa),
				"lienzoRestaurado=", crcDespues == crcAntesDeLosBichos,
				"detalle=", detalleDelDibujoDeLosBichos())
		}

		// EL CUADRO QUE QUEDA EN PANTALLA: los bichos se pintan al final. Con la colision de celda un jugador y
		// un bicho nunca comparten celda, asi que el sprite de un bicho solo puede tapar el borde del vecino.
		volverAPintarLosBichos(malla)

		anotarEtapa134(e134Bichos, tEtapa134)
	}

	// ============================== LOS OBJETOS DEL SUELO (DC-002.123) ==============================
	// Se pintan DESPUES de los bichos y antes del crc32 del cierre, con el MISMO camino que ellos: se guarda lo
	// que hay debajo de la caja del objeto, se pinta, se miden los pixeles, se devuelve el mapa y se comprueba
	// que el lienzo volvio a su crc32. Con eso el numero del mapa solo sigue siendo el del host (1437022778)
	// aunque haya un objeto encima, que es lo que la tarjeta pide que no se mueva.
	crcConItems, msItems, pxItems, itemsDibujados := crcConBichos, 0, 0, 0

	if dropKnob == "si" && len(suelo) > 0 {
		crcAntesDeLosItems := crcLienzo(pixeles)

		var ms int

		tEtapa134 = cronometro134()

		crcConItems, ms, pxItems, itemsDibujados = dibujarLosItems(malla, pixeles)
		msItems = ms

		crcDespues := crcLienzo(pixeles)
		if crcDespues != crcAntesDeLosItems {
			println("JUEGO123: CUADRO", cuadro, "LIENZO-RESTAURADO-TRAS-ITEMS DISTINTO crc32=", int(crcDespues),
				"antesDeLosItems=", int(crcAntesDeLosItems))
		}

		anotarEtapa134(e134Items, tEtapa134)

		if verboso134() {
			println("JUEGO123: ITEM CUADRO", cuadro, "objetos=", len(suelo), "dibujados=", itemsDibujados,
				"pixelesPintados=", pxItems, "msItems=", msItems, "crcConItems=", int(crcConItems),
				"mapaSolo=", int(crcMapa), "igualAlDelHost=", crcMapa == crcHost,
				"lienzoRestaurado=", crcDespues == crcAntesDeLosItems, "detalle=", detalleDelDibujoDeLosItems())
		}

		// el cuadro que queda en pantalla: los objetos se pintan al final, con los bichos
		volverAPintarLosItems(malla)
	}

	// ============================== LOS NPCs DEL PUEBLO (DC-002.127) ==============================
	// Se pintan DESPUES de los bichos y de los objetos y ANTES de los paneles: el NPC esta en el mundo. El mismo
	// camino que ellos: se guarda lo que hay debajo de su caja, se pinta, se cuentan los pixeles cambiados, se
	// devuelve el mapa y se comprueba que el lienzo volvio a su crc32 (el del host si no hay nada encima).
	crcConNPCs, msNPC, pxNPC := crcConItems, 0, 0

	if npcKnob == "si" && len(npcsDelPueblo) > 0 {
		crcAntesDeLosNPCs := crcLienzo(pixeles)

		var ms int

		tEtapa134 = cronometro134()

		crcConNPCs, ms, pxNPC = dibujarLosNPCs(malla, pixeles)
		msNPC = ms

		crcDespues := crcLienzo(pixeles)

		anotarEtapa134(e134NPCs, tEtapa134)

		if verboso134() {
			println("JUEGO127: NPC CUADRO", cuadro, "npcs=", len(npcsDelPueblo), "dibujados=", pxNPC > 0,
				"pixelesPintados=", pxNPC, "msNPCs=", msNPC, "crcConNPCs=", int(crcConNPCs),
				"mapaSolo=", int(crcMapa), "igualAlDelHost=", crcMapa == crcHost,
				"lienzoRestaurado=", crcDespues == crcAntesDeLosNPCs, "detalle=", detalleDelDibujoDeLosNPCs())
		}

		// el cuadro que queda en pantalla: los NPCs, como los bichos y los objetos
		volverAPintarLosNPCs(malla)
	}

	// ============================== EL INVENTARIO Y EL ALIJO (DC-002.124) ==============================
	// Se pintan DESPUES de los objetos del piso y ANTES del crc32 del cierre, con el MISMO camino: se guarda lo
	// que hay debajo de cada panel, se pinta, se cuentan los pixeles y se devuelve el mapa. Con el panel CERRADO
	// no se pinta nada: el cuadro del pueblo tiene que quedar intacto, que es la mitad del criterio de la tarjeta.
	pxPaneles := 0

	if inventarioKnob == "si" {
		// LA MEDICION DEL CUADRO CERRADO ES DENTRO DE LA MISMA CORRIDA: se toma el crc32 del lienzo justo antes de
		// llamar al inventario y justo despues. Si no se pinto nada, los dos numeros tienen que ser el mismo. Es la
		// unica forma honesta de decirlo: dos corridas distintas del emulador no dan los mismos crc32 porque la
		// animacion avanza por reloj (`d2cMs`), no por cuadro.
		crcAntesDelPanel := crcLienzo(pixeles)

		tEtapa134 = cronometro134()

		pxPaneles, _ = dibujarElInventario(malla, pixeles)

		anotarEtapa134(e134Inventario, tEtapa134)

		crcDespuesDelPanel := crcLienzo(pixeles)

		if verboso134() {
			if pxPaneles == 0 {
				println("JUEGO124: INV CUADRO", cuadro, "abierto=no pixelesPintados=0",
					"crcAntes=", int(crcAntesDelPanel), "crcDespues=", int(crcDespuesDelPanel),
					"elCuadroNoCambio=", crcAntesDelPanel == crcDespuesDelPanel)
			} else {
				println("JUEGO124: INV CUADRO", cuadro, "elCuadroCambio=", crcAntesDelPanel != crcDespuesDelPanel)
			}
		}
	}

	// ============================== EL HUD (DC-002.125) ==============================
	// Se dibuja ULTIMO por dos motivos: (a) el HUD va ENCIMA de todo, como en el juego, y (b) los numeros del
	// cuadro (`CON-JUGADOR`, el crc32 del mapa solo, la repetibilidad) ya estan medidos cuando el HUD toca el
	// lienzo, asi que el HUD no ensucia ninguna de las comparaciones contra el host. Y se mide su costo DENTRO de
	// la misma corrida: el crc32 del lienzo justo antes y justo despues, para poder decir que el HUD cambio el
	// cuadro (y cuanto) sin depender de comparar dos corridas distintas.
	pxHUD, msHUD := 0, 0

	if hudKnob == "si" && elHUD != nil {
		crcAntesDelHUD := crcLienzo(pixeles)

		tEtapa134 = cronometro134()

		pxHUD, msHUD = elHUD.dibujar(malla, 0, ancho, alto)

		anotarEtapa134(e134HUD, tEtapa134)

		crcDespuesDelHUD := crcLienzo(pixeles)

		if verboso134() {
			println("JUEGO125: HUD CUADRO", cuadro, "elCuadroCambio=", crcAntesDelHUD != crcDespuesDelHUD,
				"crcAntes=", int(crcAntesDelHUD), "crcDespues=", int(crcDespuesDelHUD),
				"pixeles=", pxHUD, "ms=", msHUD, "lienzoLleno=", llenosLienzo(pixeles), "de", ancho*alto)
		}
	}

	// ============================== EL MINIMAPA (DC-002.130) ==============================
	// Va DESPUES del HUD y del panel: el minimapa se pinta ENCIMA de todo, como el HUD. Y como el HUD, su costo se
	// mide DENTRO de la misma corrida: el crc32 del lienzo justo antes y justo despues, para poder decir que el
	// minimapa pinto ALGO (y cuanto) sin comparar dos corridas distintas. El reveal se pide ANTES de dibujar, asi
	// el cuadro que se mide ya tiene las celdas del sitio donde el jugador acaba de llegar.
	pxMinimapa, msMinimapa := 0, 0

	if minimapaKnob == "si" && elMinimapa != nil {
		elMinimapa.revelarElMinimapa(0)

		crcaAntesDelMinimapa := crcLienzo(pixeles)

		tEtapa134 = cronometro134()

		pxMinimapa, msMinimapa = elMinimapa.dibujarElMinimapa(malla, 0, ancho, alto)

		anotarEtapa134(e134Minimapa, tEtapa134)

		crcDespuesDelMinimapa := crcLienzo(pixeles)

		elMinimapa.crcAntes, elMinimapa.crcDespues = crcaAntesDelMinimapa, crcDespuesDelMinimapa

		msDelCuadroDelMapa = msMapa

		if verboso134() {
			println("JUEGO130: MINIMAPA CUADRO", cuadro, "elCuadroCambio=", crcaAntesDelMinimapa != crcDespuesDelMinimapa,
				"crcAntes=", int(crcaAntesDelMinimapa), "crcDespues=", int(crcDespuesDelMinimapa),
				"pixeles=", pxMinimapa, "ms=", msMinimapa, "lienzoLleno=", llenosLienzo(pixeles), "de", ancho*alto)
		}
	}

	// ============================== EL PANEL DE DIALOGO (DC-002.127) ==============================
	// Va ULTIMO: el panel va ENCIMA de todo, como en el juego. Y con el panel CERRADO no se pinta un pixel: el
	// crc32 del lienzo antes y despues de la llamada tiene que ser el mismo, que es la mitad del criterio ("el
	// pueblo se restaura al cerrar"). Cuando el panel se cierra, este crc se guarda para el cierre.
	pxDialogo, msDialogo := 0, 0

	// DC-002.128: el panel del DIALOGO tambien es el de la QUEST (el de Akara), asi que el camino se abre con
	// cualquiera de las dos palancas. Con `NPC=no` y `QUEST=si` el panel es solo el de la quest.
	if npcKnob == "si" || questKnob == "si" {
		crcAntesDelDialogo := crcLienzo(pixeles)

		tEtapa134 = cronometro134()

		pxDialogo, msDialogo = dibujarElDialogo(malla, pixeles)

		anotarEtapa134(e134Dialogo, tEtapa134)

		// el costo del panel de la QUEST, que se pinta por este mismo camino
		if questKnob == "si" && pxDialogo > 0 {
			questPxDelPanel += pxDialogo
			questMsDelPanel = msDialogo
		}

		crcDespuesDelDialogo := crcLienzo(pixeles)

		if elDialogoSeCerro && lienzoTrasCerrar == 0 && !dialogoAbierto {
			lienzoTrasCerrar = crcDespuesDelDialogo

			println("JUEGO127: NPC LIENZO-TRAS-CERRAR cuadro=", cuadro, "crc32=", int(lienzoTrasCerrar),
				"mapaSolo=", int(crcMapa), "igualAlMapaSolo=", lienzoTrasCerrar == crcMapa)
		}

		if verboso134() {
			println("JUEGO127: DIALOGO CUADRO", cuadro, "abierto=", dialogoAbierto, "pixelesPintados=", pxDialogo,
				"msPanel=", msDialogo, "crcAntes=", int(crcAntesDelDialogo),
				"crcDespues=", int(crcDespuesDelDialogo),
				"elCuadroNoCambio=", crcAntesDelDialogo == crcDespuesDelDialogo)
		}
	}

	if verboso134() {
		println("JUEGO121: CUADRO", cuadro, "CON-JUGADOR pixelesNoVacios=", llenosLienzo(pixeles), "de", ancho*alto,
			"crc32=", int(crcConJugador), "pixelesDelJugador=", cambiados, "ancla=", anclaX, anclaY,
			"dir=", dirDibujada, "cuadroDelCiclo=", cuadroAnim, "modo=", modoDelCuadro(mov.Caminando),
			"msMapa=", msMapa, "msJugador=", msJugador, "jugadores=", len(jugadores),
			"celda=", celdaDelJugadorX(), celdaDelJugadorY(),
			"bichos=", len(bichos), "pixelesDeLosBichos=", pxBicho, "msBicho=", msBicho)
	}

	if cuadro == 1 {
		crcCuadro1Jugador = crcConJugador

		println("JUEGO121: CUADRO 1 REFERENCIA igualAlDeDC002112=", crcConJugador == crcConJugador112,
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
		llenosOrigen = llenosLienzo(pixeles)
	}

	if cuadro == cuadros+2 {
		igualOrigenMapa := crcMapa == crcOrigenMapa
		igualOrigenJugador := crcConJugador == crcOrigenJugador

		println("JUEGO121: REPETIBILIDAD cuadro=", cuadro, "camara=", camara, "crcMapa=", int(crcMapa),
			"crcMapaDelCuadroAnterior=", int(crcOrigenMapa), "igualAlCuadroAnterior=", igualOrigenMapa,
			"crcConJugador=", int(crcConJugador), "crcConJugadorDelCuadroAnterior=", int(crcOrigenJugador),
			"igualConJugador=", igualOrigenJugador, "ancla=", anclaX, anclaY, "celda=", celdaDelJugadorX(),
			celdaDelJugadorY(), "llenos=", llenosLienzo(pixeles), "llenosDelCuadroAnterior=", llenosOrigen,
			"crcMapaDelCuadro1=", int(crcCuadro1Mapa), "igualAlCuadro1=", crcMapa == crcCuadro1Mapa,
			"llenosDelCuadro1=", llenosCuadro1, "cachePerezosa=", rendererDelMapa.CachePerezosa())
	}

	// ============================== EL RESUMEN DEL MOVIMIENTO ==============================
	if cuadro >= cuadros+1 {
		cache := rendererDelMapa.EstadoDeLaCache()
		println("JUEGO121: CUADRO", cuadro, "cacheSuperficies=", cache.Superficies, "cacheBytes=", cache.Bytes,
			"cacheEnIndices=", cache.EnIndices, "msMapa=", msMapa, "msJugador=", msJugador)
	}

	censoDelTramo("despues-del-cuadro")
	memoria("despues-del-cuadro")

	if cuadro == 1 && orden != "tarjeta" {
		tramosSoltados, bytes := d2asset.SoltarArenaDeBloques()
		println("JUEGO121: ARENA DE BLOQUES SOLTADA (despues de la cache) tramos=", tramosSoltados, "bytes=", bytes,
			"libre=", d2cMontonLibre())
		memoria("despues-de-soltar-el-arena")
	}

	// el resumen va ULTIMO y una sola vez: el ciclo del PVR no vuelve nunca
	//
	// DC-002.123: el de los OBJETOS se imprime tambien en el ultimo cuadro de la corrida, y no solo en el cierre del
	// ciclo. Motivo medido: con el drop encendido la fase del ciclo (que compone el DS1 siguiente) se queda sin
	// monton de C ("gc_external_alloc: OOM" en el cuadro cuadros+2), y el cierre no llega nunca. El estado de los
	// objetos ya es final cuando termina el ultimo cuadro: el bicho murio, el objeto cayo y el jugador lo recogio.
	if cuadro == cuadros {
		resumenDeLosItems()
	}

	if cuadro == cuadros+3 {
		resumen()
		resumenDelCoop(cuadro, msMapa, msJugador, int(d2cMs()-t0))
		resumenDelLeash()         // DC-002.115: la distancia medida, la aplicada y los cuadros fuera del lienzo
		resumenFinalDeLosBichos() // DC-002.121: el bicho existio, se dibujo y ocupo su celda
		resumenDelCombate()       // DC-002.122: el golpe, la vida y la muerte
		resumenDeLosItems()       // DC-002.123: el drop, el dibujo en el piso y la recogida
		resumenDeLaMuerte()       // DC-002.126: la caida cuadro a cuadro, el cadaver y su expiracion
		resumenDelHUD()           // DC-002.125: las barras, el cinturon, la zona y su costo
		resumenDeLosNPCs()        // DC-002.127: los NPCs, su texto y el panel de dialogo
		resumenDeLaQuest()        // DC-002.128: la Den of Evil, su contador, su cobro y la supervivencia
		resumenDelMinimapa()      // DC-002.130: el minimapa, su costo por cuadro y la decision siempre/por tecla

		if !resumenDeLaAnimacionHecho {
			resumenDeLaAnimacionHecho = true
			resumenDeLaAnimacion133() // DC-002.133: el control manejando la animacion (modos, giros, fases, saltos)
		}
	}

	// DC-002.133 -- EL ESTADO DE LA ANIMACION, POR PERSONAJE. Va aca, cuando el cuadro ya se dibujo: lo que la linea
	// dice es el modo, la direccion y el cuadro del ciclo que LLEGARON AL LIENZO, no lo que el reloj pidio.
	estadoDeLaAnimacion(cuadro, cuadrosDelInforme133)

	// DC-002.132 -- EL CUADRO, CONTADO. Va al final para que el numero incluya TODO lo que costo el cuadro (el
	// mapa, los sprites, el HUD, el minimapa y los instrumentos), que es lo que el jugador espera entre cuadro y
	// cuadro. Cada 30 cuadros se deja el promedio: los cuadros por segundo y los cuadros de animacion que
	// llegaron al lienzo.
	anotarElCuadro132(int64(d2cMs())-tInicio132, cuadroAnimDelHeroe132(), cuadroAnimDelBicho132())

	if cuadro%cuadrosDelInforme132 == 0 {
		resumenDelCuadro132(cuadro)
	}

	// DC-002.134: el cuadro al cronometro por etapas (ver instrumento_134.go). Va al final para que la resta
	// `dibujar - suma de etapas` sea exactamente lo que el cuadro paga en lineas de evidencia.
	//
	// DC-002.138: y el MVP JUGABLE anota aca lo que necesita de ESTE cuadro -- el crc32 del lienzo completo y del
	// mundo, los pixeles del HUD y del minimapa y, si la etapa se cerro, la evidencia de su marca. Va DESPUES de
	// todo lo que se pinta (paneles, HUD y minimapa incluidos): si fuera antes, el crc de la marca no seria el del
	// cuadro que el ojo ve.
	anotarElCuadroDelMVP(pixeles, crcLienzo(pixeles), crcMapa, pxHUD, pxMinimapa, pxDialogo, llenosLienzo(pixeles))

	anotarCuadro134(cronometro134() - tInicio134)

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

	println("JUEGO121: RESUMEN cuadrosPintados=", cuadro, "tramos=", tramos, "tramosReportados=", reportados,
		"segundosCaminados=", mov.Segundos, "subtiles=", mov.Subtiles, "celdas=", celdas,
		"celdasPorSegundo=", celdasPorSegundo, "teoricoCeldasPorSegundo=", mov.Est.Velocidad/float64(subtilesPorCelda),
		"ticks=", mov.Ticks, "recortes=", mov.Recortes, "pasoMayor=", mov.PasoMayor,
		"celdasFinales=", cx, cy, "direcciones=", len(direccionesCompuestas), "camara=", camara,
		"inyectarGuion=", inyectarGuion, "reloj=", reloj)

	// DC-002.124: el cierre del inventario y el alijo (el criterio de la tarjeta)
	resumenDelInventario()
}

// volverAlOrigen: deja el mundo en el estado exacto del cuadro 1 (cada jugador en su celda de arranque, direccion y
// cuadro de animacion del ciclo quieto, camara en el centro del grupo). Es la preparacion de la prueba de
// repetibilidad.
func volverAlOrigen() {
	if mov == nil {
		return
	}

	guardarElFinalDeLosJugadores() // DC-002.114: la vuelta es de TODOS los jugadores, no solo del 1
	volverAlOrigenLosJugadores()

	println("JUEGO121: VUELTA AL ORIGEN celda=", celdaX, celdaY, "dir=", mov.Direccion, "camara=", camara,
		"jugadores=", len(jugadores))
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

// volverAlFinal: devuelve a los jugadores a donde los dejo el guion. Es el cuadro que queda en pantalla.
func volverAlFinal() {
	if mov == nil {
		return
	}

	volverAlFinalLosJugadores() // DC-002.114: los dos, con su estado guardado

	x, y := jugadores[0].Mov.Est.Celda()

	println("JUEGO121: VUELTA AL FINAL p1Celda=", x, y, "dir=", mov.Direccion,
		"caminando=", mov.Caminando, "jugadores=", len(jugadores))
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
