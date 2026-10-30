//go:build dreamcast

// DC-002.76 -- CUANTO CUESTA CADA JUGADOR: 1, 2 Y 4 HEROES DEL MOTOR, MEDIDOS EN LA CONSOLA (Fase C.2).
//
// QUE PRUEBA. El fin del proyecto es cooperativo de 1 a 4 jugadores en una pantalla. DC-002.74 midio que
// el techo no es la imagen del binario sino lo que el motor se guarda (los caches), y DC-002.75 los acoto
// (2,81 MB de caches de datos). Falta el numero que decide: cuanto RAM y cuanto CPU cuesta CADA jugador
// extra. Este POC lo mide con el motor de verdad:
//
//   - el jugador es `d2mapentity.Player` (su `Render`/`Advance`, no una copia),
//   - su animacion es el `d2asset.Composite` real, cargado sobre un `AssetManager` real cuyas fuentes son
//     los dos MPQ del disco (`NuevoAssetManagerDeConsola`),
//   - el COF y las capas .dcc salen del MPQ por los caches con los presupuestos de consola,
//   - el mapa es el MISMO .ds1 + .dt1 del acto 1 que dibujaron DC-002.70 y DC-002.75, compuesto por el
//     mismo `nivel.go` (el archivo se copia al build y se compara su sha256), asi el LIENZO tiene que dar
//     el mismo crc32 que en esas dos tarjetas.
//
// COMO SE MIDE. Tres etapas en UNA corrida (N = 1, 2 y 4), acumulando jugadores: la etapa de N=2 tiene a
// los dos, la de N=4 a los cuatro, con direcciones distintas (0, 16, 32, 48) y en celdas distintas. Por
// etapa se informa:
//
//	monton libre (mallinfo().fordblks, el mismo instrumento de DC-002.51), colecciones del GC (el contador
//	`marca_gc_n` de libgodc, el mismo que escribe las marcas ARQ-MARCA gc_collect), duracion de una
//	recoleccion FORZADA (runtime.GC() medido con el reloj del PVR), microsegundos de trabajo por cuadro
//	(avance de los N jugadores + dibujado del mapa y los N), cuadros por segundo REALES (con la espera del
//	PVR adentro), y el peso/desalojos de cada cache.
//
// QUE NO ENTRA (declarado): la hoja de heroe (charstats/skills) y el equipamiento, que en la consola
// todavia no se cargan (Fase D): el constructor de consola del jugador los deja vacios, y esta dicho en
// d2mapentity/jugador_consola.go. Los controles maple, las camaras por jugador y la UI del coop son diseno
// posterior a este numero.
package main

import (
	"hash/crc32"
	"image/color"
	"runtime"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2animdata"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

// participante es lo unico que el POC necesita de un jugador: el motor lo cumple tal cual
// (`d2mapentity.Player` tiene Render(Surface) error y Advance(float64)). Con esta interfaz, el MISMO POC
// mide el jugador completo o solo su `Composite` -- ver participante.go y participante-real.go.txt, y el
// porque medido en la evidencia.
type participante interface {
	Render(surface d2interface.Surface) error
	Advance(elapsed float64)
}

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_heap_libre
func d2cHeapLibre() uint

//extern d2c_us
func d2cUs() uint64

//extern d2c_gc_n
func d2cGcN() uint

func init() { d2cSerialInit() }

const (
	rutaMPQ      = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChars = "/cd/JUEGO/D2CHAR.MPQ"

	// OJO: LoadDS1/LoadDT1 del AssetManager agregan ELLOS el prefijo "/data/global/tiles/", asi que aca
	// va la ruta RELATIVA a esa carpeta (medido: con la ruta completa el loader busca
	// /data/global/tiles/data/global/tiles/... y da file not found).
	nivelEnMPQ = "ACT1/TOWN/townN1.ds1"
	palEnMPQ   = "data/global/palette/ACT1/pal.dat"

	// prefijoTiles: lo que RutaDelDT1 antepone y lo que LoadDT1 vuelve a anteponer.
	prefijoTiles = "data/global/tiles/"

	ladoVentana      = 512
	centroX, centroY = 28, 20

	ancho, alto = 640, 480
	mapaX       = (ancho - ladoVentana) / 2
	mapaY       = (alto - ladoVentana) / 2

	// cuadrosPorEtapa: los cuadros que se miden por cada N (a 25 Hz de juego son 2,4 s de animacion)
	cuadrosPorEtapa = 60

	// cuadrosDeEpilogo: despues de la etapa de 4 se sigue dibujando, para que la captura de pantalla
	// tenga el cuadro de los cuatro y para ver si el estado se sostiene (o se cae) con el tiempo.
	cuadrosDeEpilogo = 600

	// subtilesPorCelda: 5 subtiles por celda del mapa (d2mapentity.subtilesPerTile)
	subtilesPorCelda = 5
)

// dibujarJugadores: si el cuadro pinta a los jugadores. Medido en esta tarjeta: con los seis .dcc del
// personaje cargados, el PRIMER cuadro que los pinta muere con "Unhandled exception ... Data address error"
// (PC 3fd17b4f, PR identico, R3 apuntando a VRAM) y la consola se reinicia. Se apaga para que la corrida
// llegue a las tres etapas y mida lo que la tarjeta pregunta (RAM y tiempo de creacion por jugador); el
// fallo del pintado queda como bloqueo declarado en la evidencia.
const dibujarJugadores = false

// zonaActiva: si el POC compone la zona del acto 1 (el .ds1 y sus ocho .dt1) antes de los jugadores. Con el
// jugador completo enlazado esto NO entra (ver el comentario en main y la evidencia); con el Composite
// (MODO=composite) el BIN vuelve a 2,76 MB y la zona compone igual que en DC-002.70/DC-002.75.
const zonaActiva = false

// niveles son las etapas de la corrida, en orden y acumulativas.
var niveles = []int{1, 2, 4}

// celdas y direcciones de los cuatro jugadores: cada uno en su celda y mirando a otro lado, que es lo que
// hace que no compartan el frame compuesto (el frame del personaje depende de la direccion).
var (
	celdas      = [4][2]int{{28, 20}, {31, 20}, {28, 23}, {31, 23}}
	direcciones = [4]int{0, 16, 32, 48}
)

var (
	am       *d2asset.AssetManager
	caches   *d2asset.CachesDeAssets
	renderer *pvr.Renderer

	// los MPQ abiertos a mano para las LECTURAS GRANDES (ver el comentario en main)
	mpqDatos, mpqCharsArch *d2mpq.MPQ

	jugadores []participante

	mapaIndices   []byte
	paletaDelMapa [][3]uint8

	// estado de la etapa en curso
	etapa      int
	nEtapa     int
	cuadros    int
	etapaLista bool
	usDeEtapa  uint64
	gcDeEtapa  uint
	avanceUs   int64
	dibujoUs   int64
	creacionUs int64
)

func main() {
	println("COOP276: arranca -- cuanto cuesta cada jugador (1, 2 y 4) en la consola")

	// ============================== 0. LOS PRESUPUESTOS DE DC-002.75 ==============================
	antes := d2asset.PresupuestosVigentes()
	println("COOP276: budgets DE PC animaciones=", antes.Animaciones, "cofs=", antes.CoFs, "dccs=", antes.DCCs,
		"dt1s=", antes.DT1s, "ds1s=", antes.DS1s)

	d2asset.AjustarPresupuestosDeConsola()

	ahora := d2asset.PresupuestosVigentes()
	println("COOP276: budgets DE CONSOLA animaciones=", ahora.Animaciones, "cofs=", ahora.CoFs, "dccs=", ahora.DCCs,
		"dt1s=", ahora.DT1s, "ds1s=", ahora.DS1s, "sumaDeDatos=", ahora.SumaDeLosCachesDeDatos())

	// ============================== 1. EL VIDEO PRIMERO ==============================
	// El arranque del PVR necesita el monton vacio (medido en DC-002.50): por eso va antes de cargar nada.
	renderer = pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("COOP276: el video no arranco:", err.Error())

		return
	}

	println("COOP276: video andando montonLibre=", int(d2cHeapLibre()), "vramDeLaPantalla=", pvr.VramDeLaPantalla())

	// ============================== 2. EL ASSETMANAGER DEL MOTOR ==============================
	// El mismo AssetManager del juego: loader real, caches reales, fuentes MPQ del disco.
	var err error

	am, caches, err = d2asset.NuevoAssetManagerDeConsola(d2util.LogLevelDefault, rutaMPQ, rutaMPQChars)
	if err != nil {
		println("COOP276: el AssetManager no se pudo armar:", err.Error())

		return
	}

	println("COOP276: assetmanager listo montonLibre=", int(d2cHeapLibre()))

	// los dos MPQ abiertos a mano: las LECTURAS GRANDES no pueden pasar por `AssetManager.LoadFile`
	// (que es `ioutil.ReadAll` sobre el stream del MPQ) -- medido en esta tarjeta: el `io.ReadAll` del
	// primer .dt1 de 505 KB mata el SH-4 con "Data address error" (ver la evidencia). `ReadFile` del MPQ
	// reserva el buffer de una sola vez y lee de un tiron, que es lo que ya usaba DC-002.70.
	mpqDatos, err = d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("COOP276: MPQ-ABRIR FALLO:", err.Error())

		return
	}

	mpqCharsArch, err = d2mpq.FromFile(rutaMPQChars)
	if err != nil {
		println("COOP276: MPQ-CHARS-ABRIR FALLO:", err.Error())

		return
	}

	println("COOP276: mpq a mano listos montonLibre=", int(d2cHeapLibre()))

	// ============================== 2. LA ZONA (OPCIONAL: NO ENTRA) ==============================
	// MEDIDO EN ESTA TARJETA, y es uno de sus resultados: la zona del acto 1 (8 .dt1, 4,35 MB de crudos)
	// NO entra junto con el jugador del motor. El jugador real arrastra 73 paquetes (d2mapentity ->
	// d2hero/d2records/d2ui/d2item...) y el BIN pasa de 2,74 MB (DC-002.75) a 4,87 MB: el monton baja de
	// 13,5 MB a 11,38 MB y la rotacion de los ocho .dt1 (crudo + parseado, ~1,2 MB por archivo) lo llena.
	// Con `zonaActiva = true` la corrida muere en el .dt1 6 de 8 con
	// "fatal error: gc_external_alloc: OOM ... Requested sbrk_base ... diff 397312"
	// (console/dc/evidencia/dc-002.76-zona-no-entra.log). Con false, este POC mide lo que la tarjeta pide
	// -- los jugadores -- sobre el lienzo negro, y lo declara.
	if zonaActiva {
		componerLaZona()
	} else {
		println("COOP276: ZONA no compuesta: el jugador del motor y la zona del acto 1 no entran juntos (ver la evidencia)")
	}

	palCrudo, err := am.LoadFile(palEnMPQ)
	if err != nil || len(palCrudo) < 768 {
		println("COOP276: PALETA FALLO:", palEnMPQ)

		return
	}

	paleta := PaletaRGB(palCrudo)
	paletaDelMapa = paleta[:]

	palCrudo = nil

	// ============================== 3. LA TABLA DE ANIMACIONES ==============================
	// Sin ella `Composite.createMode` responde "could not find Animation data" y no carga ni una capa: el
	// jugador se quedaria sin animacion. Es la carga que hace `d2app.initAnimationData` en la PC.
	animData := cargarAnimData()

	if animData == nil {
		return
	}

	println("COOP276: BASELINE (N=0) montonLibre=", int(d2cHeapLibre()), "gcColecciones=", int(d2cGcN()),
		"vramDelLienzo=", pvr.VramDelLienzo())

	// ============================== 4. EL CICLO DE CUADROS DEL MOTOR ==============================
	// `Run` es el mismo ciclo de DC-002.70: espera el PVR, actualiza, dibuja, sube el cuadro y lo pinta.
	// Adentro, `actualizar` cambia de etapa (crea los jugadores que faltan) cada `cuadrosPorEtapa` cuadros.
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast)"); err != nil {
		println("COOP276: el ciclo de cuadros salio con error:", err.Error())
	}
}

// componerLaZona es el camino de DC-002.70/DC-002.75 (el .ds1 y sus ocho .dt1 por los caches del motor) y
// deja el LIENZO en `mapaIndices`. Un .dt1 a la vez y con su crudo suelto: el monton del SH-4 no tiene
// lugar para los ocho vivos (ver el comentario en main).
func componerLaZona() {
	d, err := am.LoadDS1(nivelEnMPQ)
	if err != nil {
		println("COOP276: DS1-LEER FALLO:", err.Error())

		return
	}

	println("COOP276: DS1 version=", d.Version(), "act=", int(d.Act), "w=", d.Width(), "h=", d.Height(),
		"files=", len(d.Files), "ds1sPeso=", caches.Ds1s.GetWeight(), "ds1sEvicto=", caches.Ds1s.Evictions(),
		"montonLibre=", int(d2cHeapLibre()))

	nivel := NuevoNivel(d, ladoVentana, ladoVentana, centroX, centroY)

	for _, f := range d.Files {
		ruta := relativoATiles(RutaDelDT1(f))

		dt1 := cargarDT1EnElCache(ruta)
		if dt1 == nil {
			continue
		}

		nivel.Alimentar(dt1.Tiles)

		dt1 = nil

		caches.Dt1s.Clear()
		runtime.GC()

		println("COOP276: DT1 soltado montonLibre=", int(d2cHeapLibre()), "gcColecciones=", int(d2cGcN()))
	}

	nivel.Cerrar()

	println("COOP276: COMPUESTO pisos=", nivel.PisosCompuestos, "/", nivel.PisosPedidos,
		"paredes=", nivel.ParedesCompuestas, "/", nivel.ParedesPedidas,
		"sintilePiso=", nivel.PisosSinTile, "sintilePared=", nivel.ParedesSinTile,
		"dt1sPeso=", caches.Dt1s.GetWeight(), "dt1sEvicto=", caches.Dt1s.Evictions())
	println("COOP276: LIENZO pixelesNoVacios=", nivel.PixelesNoVacios(), "de", nivel.Ancho*nivel.Alto,
		"crc32=", nivel.CRC32(), "montonLibre=", int(d2cHeapLibre()))

	mapaIndices = nivel.Canvas
}

// cargarDT1EnElCache hace los MISMOS pasos que `AssetManager.LoadDT1` (leer, parsear, insertar en el
// cache de los dt1 con el peso en bytes) con UNA diferencia declarada: no pasa por `LoadFileHandoff`.
//
// POR QUE. `LoadFileHandoff` guarda los bytes crudos de cada archivo para el segundo lector que los pida,
// con un tope de 24 MB (`handoffBudget`, d2core/d2asset/file_handoff.go): es un tope de PC. En la consola
// los ocho .dt1 de una zona son 4,35 MB de crudos que quedarian vivos a la vez, encima del cache. Con
// `LoadFile` el crudo se suelta apenas se parsea (el `dt1` parseado es lo que queda, y es lo que el cache
// pesa). El cache y sus numeros son los del motor: `Pesar` y `TamanoDT1` son los mismos que usa LoadDT1.
func cargarDT1EnElCache(ruta string) *d2dt1.DT1 {
	if valor, encontrado := caches.Dt1s.Retrieve(ruta); encontrado {
		return valor.(*d2dt1.DT1)
	}

	crudo, err := LeerDelMPQ(mpqDatos, "data/global/tiles/"+ruta)
	if err != nil {
		println("COOP276: DT1 LEER FALLO", ruta, err.Error())

		return nil
	}

	dt1, err := d2dt1.LoadDT1(crudo)
	if err != nil {
		println("COOP276: DT1 PARSE FALLO", ruta, err.Error())

		return nil
	}

	crudo = nil

	if err := caches.Dt1s.Insert(ruta, dt1, d2asset.Pesar(d2asset.TamanoDT1(dt1))); err != nil {
		println("COOP276: DT1 INSERT FALLO", ruta, err.Error())

		return nil
	}

	println("COOP276: DT1", ruta, "tiles=", len(dt1.Tiles),
		"pesoEnCache=", caches.Dt1s.GetWeight(), "evicto=", caches.Dt1s.Evictions(),
		"montonLibre=", int(d2cHeapLibre()))

	return dt1
}

// cargarAnimData es la carga de `d2app.initAnimationData`: el archivo y el parser del motor. Se lee con
// `ReadFile` del MPQ y no con `LoadFile` por lo mismo que los .dt1 (411.104 B por `io.ReadAll` matan el
// SH-4). El crudo se suelta enseguida; lo que queda vivo son los registros.
func cargarAnimData() *d2animdata.AnimationData {
	crudo, err := LeerDelMPQ(mpqDatos, "data/global/animdata.d2")
	if err != nil {
		println("COOP276: ANIMDATA-LEER FALLO:", err.Error())

		return nil
	}

	animData, err := d2animdata.Load(crudo)
	if err != nil {
		println("COOP276: ANIMDATA-PARSE FALLO:", err.Error())

		return nil
	}

	am.Records.Animation.Data = animData

	println("COOP276: ANIMDATA registros=", animData.GetRecordsCount(), "crudo=", len(crudo),
		"montonLibre=", int(d2cHeapLibre()))

	crudo = nil

	runtime.GC()

	println("COOP276: ANIMDATA despues del GC montonLibre=", int(d2cHeapLibre()))

	return animData
}

// prepararEtapa deja la etapa en condiciones: crea los jugadores que falten y toma la foto de partida.
func prepararEtapa() {
	nObjetivo := niveles[etapa]

	usAntes := d2cUs()
	gcAntes := d2cGcN()

	for len(jugadores) < nObjetivo {
		crearJugador(len(jugadores))
	}

	creacionUs = int64(d2cUs() - usAntes)
	gcDeEtapa = d2cGcN() - gcAntes
	usDeEtapa = d2cUs()
	avanceUs = 0
	dibujoUs = 0
	cuadros = 0
	nEtapa = nObjetivo
	etapaLista = true

	println("COOP276: ETAPA n=", nEtapa, "montonLibre=", int(d2cHeapLibre()),
		"creacionUs=", creacionUs, "gcEnLaCreacion=", int(gcDeEtapa),
		"gcTotal=", int(d2cGcN()))

	informeDeCaches(nEtapa, "al crear")

	// una recoleccion FORZADA, cronometrada: el numero que el GC agrega al cuadro cuando corre.
	t0 := d2cUs()
	runtime.GC()
	t1 := d2cUs()

	println("COOP276: ETAPA n=", nEtapa, "gcForzadoUs=", int64(t1-t0), "montonLibreDespuesDelGC=", int(d2cHeapLibre()))
}

// crearJugador instancia UN jugador y cuenta lo que costo. El COMO lo decide participante.go: el Composite
// del motor (lo que entra en la consola) o el `d2mapentity.Player` completo (participante-real.go.txt).
func crearJugador(indice int) {
	celda := celdas[indice]
	subtiles := [2]int{celda[0] * subtilesPorCelda, celda[1] * subtilesPorCelda}

	nombre := nombres[indice]

	runtime.GC() // medido: sin esta pausa el jugador 1 ya no carga (ver la evidencia)

	// la foto del monton y de la tabla de animaciones ANTES de crear: si la tabla se vacia, el GC de
	// libgodc se llevo el map de records (medido: ver la evidencia)
	if am.Records != nil && am.Records.Animation.Data != nil {
		println("COOP276: ANIMDATA vivo registros=", am.Records.Animation.Data.GetRecordsCount(),
			"gcTotal=", int(d2cGcN()))
	}

	t0 := d2cUs()
	h0 := d2cHeapLibre()
	g0 := d2cGcN()

	jugador, err := nuevoParticipante(indice, nombre, subtiles, direcciones[indice])

	t1 := d2cUs()
	h1 := d2cHeapLibre()

	if err != nil {
		println("COOP276: JUGADOR", nombre, "FALLO:", err.Error())

		return
	}

	jugadores = append(jugadores, jugador)

	println("COOP276: JUGADOR", nombre, "indice=", indice, "direccion=", direcciones[indice],
		"celda=", celda[0], celda[1], "subtiles=", subtiles[0], subtiles[1],
		"creacionUs=", int64(t1-t0), "montonAntes=", int(h0), "montonDespues=", int(h1),
		"gcPorLaCreacion=", int(d2cGcN()-g0), "gcTotal=", int(d2cGcN()))
}

var nombres = [4]string{"J1", "J2", "J3", "J4"}

// informeDeCaches imprime peso y desalojos de cada cache de datos, que es donde se ve el costo de un
// jugador extra (sus capas .dcc y su COF).
func informeDeCaches(n int, cuando string) {
	println("COOP276: CACHES n=", n, cuando,
		"| dt1s peso=", caches.Dt1s.GetWeight(), "evicto=", caches.Dt1s.Evictions(), "de", caches.Dt1s.GetBudget(),
		"| ds1s peso=", caches.Ds1s.GetWeight(), "evicto=", caches.Ds1s.Evictions(), "de", caches.Ds1s.GetBudget(),
		"| cofs peso=", caches.Cofs.GetWeight(), "evicto=", caches.Cofs.Evictions(), "de", caches.Cofs.GetBudget(),
		"| dccs peso=", caches.Dccs.GetWeight(), "evicto=", caches.Dccs.Evictions(), "de", caches.Dccs.GetBudget(),
		"| anim peso=", caches.Animations.GetWeight(), "evicto=", caches.Animations.Evictions(), "de", caches.Animations.GetBudget())
}

// cerrarEtapa imprime la fila de la etapa: es el numero de la tarjeta.
func cerrarEtapa() {
	transcurrido := d2cUs() - usDeEtapa

	fps := int64(0)
	if transcurrido > 0 {
		fps = int64(cuadros) * 1000000 / int64(transcurrido)
	}

	avancePorCuadro := int64(0)
	dibujoPorCuadro := int64(0)

	if cuadros > 0 {
		avancePorCuadro = avanceUs / int64(cuadros)
		dibujoPorCuadro = dibujoUs / int64(cuadros)
	}

	println("COOP276: FILA n=", nEtapa, "montonLibre=", int(d2cHeapLibre()),
		"gcColeccionesTotal=", int(d2cGcN()), "gcEnLaEtapa=", int(d2cGcN()-gcDeEtapa),
		"cuadros=", cuadros, "avanceUsPorCuadro=", avancePorCuadro, "dibujoUsPorCuadro=", dibujoPorCuadro,
		"trabajoUsPorCuadro=", avancePorCuadro+dibujoPorCuadro, "fps=", fps)

	informeDeCaches(nEtapa, "al cerrar")
}

// actualizar: un cuadro de juego. Avanza los N jugadores (es el costo de CPU del cooperativo) y cuenta los
// cuadros de la etapa; al llegar a cuadrosPorEtapa imprime la fila y pasa a la siguiente N.
func actualizar() error {
	if !etapaLista {
		prepararEtapa()
	}

	t0 := d2cUs()

	for i := 0; i < nEtapa; i++ {
		jugadores[i].Advance(pasoDeCuadro)
	}

	avanceUs += int64(d2cUs() - t0)
	cuadros++

	if cuadros < cuadrosPorEtapa {
		return nil
	}

	// la etapa de 4 NO se cierra con el mismo corte: sigue dibujandose (ver cuadrosDeEpilogo)
	if etapa == len(niveles)-1 {
		return nil
	}

	cerrarEtapa()

	etapa++
	etapaLista = false

	return nil
}

// dibujar: compone el cuadro en el lienzo de la Surface (el mapa y los N jugadores) y, en el primer cuadro
// de cada etapa, deja el crc32 del cuadro en la evidencia.
func dibujar(superficie d2interface.Surface) error {
	malla, ok := superficie.(*pvr.Surface)
	if !ok {
		return errorDelPoc("el Renderer no entrego una Surface del backend PVR")
	}

	if !etapaLista {
		return nil
	}

	t0 := d2cUs()

	malla.Clear(color.RGBA{R: 0, G: 0, B: 0, A: 255})

	if mapaIndices != nil {
		malla.PushTranslation(mapaX, mapaY)
		malla.RenderIndices(mapaIndices, ladoVentana, ladoVentana, paletaDelMapa)
		malla.Pop()
	}

	for i := 0; i < nEtapa && dibujarJugadores; i++ {
		jugadores[i].Render(malla)
	}

	dibujoUs += int64(d2cUs() - t0)

	if cuadros == 1 || cuadros == cuadrosPorEtapa || cuadros == cuadrosPorEtapa+cuadrosDeEpilogo {
		pixeles := malla.Imagen().Pixeles()
		llenos := 0

		for j := 0; j+3 < len(pixeles); j += 4 {
			if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
				llenos++
			}
		}

		println("COOP276: CUADRO n=", nEtapa, "cuadro=", cuadros, "pixelesNoVacios=", llenos,
			"crc32=", int(crc32.ChecksumIEEE(pixeles)), "montonLibre=", int(d2cHeapLibre()))
	}

	// la etapa de 4 no se cierra: se sostiene y se informa cada tanto (la captura de pantalla cae aca)
	if etapa == len(niveles)-1 && cuadros%cuadrosPorEtapa == 0 {
		println("COOP276: SIGUE n=", nEtapa, "cuadros=", cuadros, "montonLibre=", int(d2cHeapLibre()),
			"gcColeccionesTotal=", int(d2cGcN()), "fps=", renderer.CurrentFPS())
	}

	return nil
}

// pasoDeCuadro: el paso que recibe el motor por cuadro, igual que el juego (tickTime de d2gamescreen).
const pasoDeCuadro = 1.0 / 25.0

// relativoATiles: LoadDT1 del AssetManager antepone "/data/global/tiles/", y RutaDelDT1 ya lo trae.
func relativoATiles(ruta string) string {
	return strings.TrimPrefix(ruta, prefijoTiles)
}

// errorDelPoc: un error con nombre, para que el que llama pueda distinguirlo de uno del motor.
type errorDelPoc string

func (e errorDelPoc) Error() string { return string(e) }
