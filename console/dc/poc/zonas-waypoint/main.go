//go:build dreamcast

// DC-002.77 -- TILES POR ZONA Y WAYPOINT: que el pico de memoria NO crezca al cambiar de escenario
// (Fase B.1d).
//
// QUE PRUEBA. DC-002.75 acoto los caches del AssetManager a 2,81 MB y midio que UNA zona entra. Lo que
// faltaba medir es lo que de verdad hace el juego: ENTRAR a una zona, IRSE a otra y VOLVER. Si el cache
// no desaloja, cada zona que se visita deja su pedazo de RAM tomado y el monton baja un escalon por
// zona hasta que el juego se cae. Esta corrida mide, con numeros, las tres cosas que pide la tarjeta:
//
//  1. el PESO de cada zona (el .ds1 y cada .dt1 que el mismo lista, crudos y ya parseados),
//  2. el PICO del monton en (a) entrar al pueblo, (b) ir a una segunda zona, (c) volver al pueblo,
//  3. cuanto TARDA cada carga (milisegundos del reloj del hardware). Ese costo esta aceptado de antemano:
//     es el precio de cargar menos (decision de Daniel, 2026-10-30).
//
// LA PRUEBA DE LA EVCICION ES LA FASE (c), no la (b): la (c) es LA MISMA ZONA que la (a). Si el pico de
// (a) y el de (c) son el mismo (dentro del margen), el cache devolvio lo que ya no se usa; si (c) pide
// mas que (a), cada visita deja basura tomada. La (b) se informa igual, pero una zona distinta tiene
// otro tamano y no se puede comparar contra (a) sin decirlo.
//
// EL PICO SE MIDE CON mallinfo (arena / uordblks / fordblks), el MISMO instrumento de DC-002.51/75,
// porque el otro candidato -- runtime.ReadMemStats -- esta MEDIDO como inservible en esta consola: el
// runtime de libgodc lo deja en cero (DC-002.9, `console/dc/poc/goroutines`: `ms.Alloc` = 0). Se muestrea
// en CADA paso de la carga (crudo leido / parseado / insertado), no solo al final, para que un pico
// transitorio -- el crudo y lo parseado viven juntos un instante -- no quede tapado.
//
// Y SE SEPARA LA BASURA DEL CACHE: al principio de cada fase se corre `runtime.GC()` y se informa el
// monton antes y despues. Si el monton crece pero el GC lo devuelve, es basura del recolector; si el GC
// no lo devuelve, es el cache. Sin esa separacion el numero no dice nada.
//
// QUE NO ENTRA: el personaje (DC-002.76 lo midio: 349.519 B de .dcc + 1.132 B de COF y 2,01 s por
// jugador), la composicion del mapa mas alla de lo que ya hace `nivel.go`, y optimizar el tiempo de carga.
package main

import (
	"hash/crc32"
	"image/color"
	"runtime"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset/types"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2render/pvr"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_ms
func d2cMs() uint

//extern d2c_libre
func d2cLibre(p uintptr)

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
	rutaMPQ = "/cd/JUEGO/D2DATA.MPQ"

	// zonaA: el pueblo del acto 1, la MISMA zona de DC-002.70/75 (asi los numeros se comparan).
	zonaA              = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	centroAx, centroAy = 28, 20

	// zonaB: el preset de EXTERIORES del acto 1 -- el tipo de zona al que lleva un waypoint del pueblo
	// (Cold Plains y compania se arman con estos presets). El POC no trae el generador de DRLG del
	// motor, asi que se carga el preset tal cual, igual que la zona A se carga como townN1.ds1.
	zonaB              = `data\global\tiles\ACT1\OUTDOORS\wild1.ds1`
	centroBx, centroBy = 8, 8

	palEnMPQ = "data/global/palette/ACT1/pal.dat"

	ladoVentana = 512
	ancho, alto = 640, 480
	mapaX       = (ancho - ladoVentana) / 2
	mapaY       = (alto - ladoVentana) / 2

	// cuadrosCuadro: a los cuantos cuadros se informa el monton "con el cuadro andando".
	cuadrosCuadro = 60
)

// cuentas: aciertos y faltas de un cache durante TODO el recorrido (el Cache solo lleva los desalojos).
type cuentas struct{ aciertos, faltas int }

var (
	// DC-002.78: cuando la palanca `porBuffer` esta prendida, los .dt1 se piden al AssetManager del MOTOR
	// (`am.LoadDT1`): asi lo que se mide en Flycast es la ruta real del juego -- el buffer unico que se reusa
	// y la copia de los bloques al parsear (d2core/d2asset/lectura_de_tiles.go, d2dt1.LoadDT1CopiaBloques).
	am *d2asset.AssetManager

	caches *d2asset.CachesDeAssets

	cuentaDT1s cuentas
	cuentaDS1s cuentas

	// los arreglos del compositor que se reusan entre zonas (lienzo, scratch y pedidos). En la Dreamcast
	// el lienzo (262.144 B) y el scratch (81.920 B) pasan de 64 KB: si se piden por fase, se quedan
	// tomados para siempre y el pico de (c) trae de mas el de (b) (ver nivel.go, BuffersDelNivel).
	reusoDelNivel BuffersDelNivel

	// el lienzo y la paleta de la ULTIMA zona cargada: es lo que se dibuja al final para probar que el
	// PVR sigue sano despues de entrar y salir de tres zonas.
	mapaIndices   []byte
	paletaDelMapa [][3]uint8

	mapaPintado bool
	cuadros     int
)

// medida: lo que se midio en UNA carga de zona.
type medida struct {
	rotulo    string
	ruta      string
	ds1Crudo  int
	ds1Vivo   int
	files     int
	pedidos   int
	compuesto int
	pixeles   int
	crc       uint32
	ms        int

	dt1Crudos int
	dt1Vivos  int
	dt1Fallos int

	// el monton al principio (tras GC) y el pico durante la carga
	usadoAlEmpezar int
	usadoPico      int
	arenaPico      int
	libreMin       int
	dondePico      string
	usadoAlFinal   int
	libreAlFinal   int

	eviccionesDT1 int
	eviccionesDS1 int
	pesoDT1       int
	pesoDS1       int
	faltasDT1     int
	aciertosDT1   int
	faltasDS1     int
	aciertosDS1   int
}

func main() {
	println("ZONAS: arranca DC-002.81 -- el MOTOR carga las zonas con la ruta de tiles cerrada en el POC")
	println("ZONAS: palancas: porBuffer=", porBuffer, "leerConPrestamo=", leerConPrestamo,
		"sinCacheDt1=", sinCacheDt1, "gcEntreArchivos=", gcEntreArchivos,
		"porMotor=", porMotor, "arranqueDeConsola=", arranqueDeConsola)
	println("ZONAS: monton de C al arrancar: " + estado())

	// DC-002.81: el ARRANQUE de la consola. En el motor de verdad esto son las tres palancas juntas
	// (`d2asset.ArrancarComoConsola`), y el binario de consola del motor lo llama ANTES de d2app.Create.
	// La palanca `MOTOR=1 ARRANQUE=0` es el ANTES: el motor tal como arranca hoy, sin la linea, con el
	// camino de la PC -- para que "el main no lo usaba" sea una medicion y no una sospecha.
	switch {
	case porMotor && !arranqueDeConsola:
		println("ZONAS: ARRANQUE=0 -- el motor como arranca HOY: sin la linea, camino de la PC")
	case porMotor:
		d2asset.ArrancarComoConsola()
	default:
		// los caminos de DC-002.78/.80, tal cual quedaron medidos: el POC prende lo suyo a mano
		d2asset.AjustarPresupuestosDeConsola()
	}

	p := d2asset.PresupuestosVigentes()
	println("ZONAS: presupuestos vigentes: animaciones=", p.Animaciones, "cofs=", p.CoFs, "dccs=", p.DCCs,
		"dt1s=", p.DT1s, "ds1s=", p.DS1s, "sumaDatos=", p.SumaDeLosCachesDeDatos())

	if porMotor {
		// ================= EL CAMINO DEL MOTOR (DC-002.81) =================
		// El AssetManager lo arma el MOTOR: `NewAssetManager` (d2app/app.go:134) + la fuente MPQ que
		// agrega su arranque (d2app/initialization.go:92). Antes de esta tarjeta ese camino dejaba los
		// presupuestos de PC y las tres palancas apagadas; con la linea del arranque queda el mismo
		// objeto que arma `NuevoAssetManagerDeConsola`, sin cambiar ni una linea de la PC.
		d2asset.UsarLecturaPrestadaDeTiles(leerConPrestamo)

		var err error

		am, err = d2asset.NewAssetManager(d2util.LogLevelDefault)
		if err != nil {
			println("MOTOR: NewAssetManager FALLO:", err.Error())

			return
		}

		if err := am.AddSource(rutaMPQ, types.AssetSourceMPQ); err != nil {
			println("MOTOR: AddSource FALLO:", err.Error())

			return
		}

		caches = am.CachesDeAssets()

		arena := d2asset.EstadoDeLosTiles()
		archivos := d2asset.EstadoDeLosArchivos()

		println("MOTOR: assetmanager armado por el camino del motor (NewAssetManager + AddSource, como d2app)")
		println("MOTOR: caminoDeTilesDeConsola=", arena.CaminoDeConsola,
			"lecturaPrestada=", d2asset.LecturaPrestadaDeTiles(),
			"lecturaDeArchivosDeConsola=", archivos.CaminoDeConsola,
			"prefetchApagado=", archivos.PrefetchApagado)
		println("MOTOR: monton con el assetmanager del motor: " + estado())
	} else if porBuffer {
		// la palanca de medicion de DC-002.78: prende la ruta del arena. Desde DC-002.80 el camino lo prende
		// `NuevoAssetManagerDeConsola` solo; aca se adelanta para que el informe de abajo salga con la ruta
		// ya prendida (y para poder medir con `BUFFER=0`, sin AssetManager de consola, igual que antes).
		d2asset.ActivarCaminoDeTilesDeConsola()

		// DC-002.80: y como se lee cada .dt1 adentro del arena -- con la memoria de trabajo prestada
		// (`mpq.ReadFileInto`, lo de esta tarjeta) o por el stream del MPQ (lo de DC-002.78). El
		// descompresor de la consola lo conecta el propio `d2mpq` al arrancar (init del build `dreamcast`).
		d2asset.UsarLecturaPrestadaDeTiles(leerConPrestamo)

		println("ZONAS: camino de tiles de consola prendido:", d2asset.EstadoDeLosTiles().CaminoDeConsola,
			"lecturaPrestada=", leerConPrestamo, "monton="+estado())

		// el AssetManager del MOTOR (el mismo de DC-002.76): es el que sabe cargar un .dt1 por la ruta de
		// la consola. Se arma ANTES del video por el mismo motivo que todo lo demas: el PVR necesita el
		// monton ordenado al arrancar.
		var err error

		am, caches, err = d2asset.NuevoAssetManagerDeConsola(d2util.LogLevelDefault, rutaMPQ)
		if err != nil {
			println("ZONAS: el AssetManager no se pudo armar:", err.Error())

			return
		}

		println("ZONAS: assetmanager de consola listo, monton: " + estado())
	} else {
		caches = d2asset.NuevosCachesDeAssets()
	}

	// el video primero: el arranque del PVR necesita el monton vacio (medido en DC-002.50)
	renderer := pvr.NuevoRenderer()

	if err := pvr.PantallaIniciar(ancho, alto); err != nil {
		println("ZONAS: el video no arranco:", err.Error())

		return
	}

	println("ZONAS: video andando, monton: " + estado())

	mpq, err := d2mpq.FromFile(rutaMPQ)
	if err != nil {
		println("ZONAS: MPQ-ABRIR FALLO:", err.Error())

		return
	}

	// la paleta del acto: es la misma para las tres fases (el acto no cambia)
	rawPal, err := LeerDelMPQ(mpq, palEnMPQ)
	if err != nil || len(rawPal) < 768 {
		println("ZONAS: PALETA FALLO:", palEnMPQ)

		return
	}

	palActo := PaletaRGB(rawPal)

	// ============================== LAS TRES FASES ==============================
	a1 := cargarZona(mpq, "a) ENTRA AL PUEBLO", zonaA, centroAx, centroAy)
	if a1 == nil {
		return
	}

	paletaDelMapa = palActo[:]

	// DC-002.78: el lienzo y el scratch se REUSAN entre las tres fases (ver nivel.go, BuffersDelNivel).
	// Antes se pedia uno nuevo por fase: en la Dreamcast esos dos arreglos pasan de 64 KB, se van al
	// monton externo y no los devuelve nadie, asi que el pico de (c) traia de mas los de (b) y la
	// comparacion contra (a) media otra cosa.
	b := cargarZona(mpq, "b) WAYPOINT A LA SEGUNDA ZONA", zonaB, centroBx, centroBy)
	if b == nil {
		return
	}

	a2 := cargarZona(mpq, "c) VUELVE AL PUEBLO", zonaA, centroAx, centroAy)
	if a2 == nil {
		return
	}

	mapaIndices = a2.canvas

	// ============================== EL VEREDICTO ==============================
	println("")
	println("ZONAS: ================ EL VEREDICTO ================")
	println("ZONAS: (a) pueblo      picoUsado=", a1.usadoPico, "arena=", a1.arenaPico,
		"libreMin=", a1.libreMin, "ms=", a1.ms, "crc32=", int(a1.crc))
	println("ZONAS: (b) segunda     picoUsado=", b.usadoPico, "arena=", b.arenaPico,
		"libreMin=", b.libreMin, "ms=", b.ms, "crc32=", int(b.crc))
	println("ZONAS: (c) vuelta      picoUsado=", a2.usadoPico, "arena=", a2.arenaPico,
		"libreMin=", a2.libreMin, "ms=", a2.ms, "crc32=", int(a2.crc))

	// la prueba de la eviccion: (c) es la MISMA zona que (a). El margen es 64 KB (el 1,5 % del monton
	// medido de ~6 MB): mas que eso NO es ruido del asignador.
	margen := 64 * 1024
	difPico := a2.usadoPico - a1.usadoPico
	difArena := a2.arenaPico - a1.arenaPico

	println("ZONAS: pico(c) - pico(a) =", difPico, "bytes (margen admitido +-", margen, ")")
	println("ZONAS: arena(c) - arena(a) =", difArena, "bytes")

	if difPico <= margen && difArena <= margen {
		println("ZONAS: EVCICION OK -- volver al pueblo no dejo RAM tomada de mas: el cache devuelve lo que no se usa")
	} else {
		println("ZONAS: EVCICION MAL -- cada visita al pueblo dejo RAM tomada (el cache NO esta desalojando)")
	}

	if a1.crc == a2.crc && a1.pixeles == a2.pixeles {
		println("ZONAS: LIENZO OK -- el pueblo de la vuelta es identico al de la entrada: crc32", int(a1.crc),
			"y", a1.pixeles, "pixeles en las dos")
	} else {
		println("ZONAS: LIENZO CAMBIO -- entrada crc32=", int(a1.crc), a1.pixeles, "pixeles; vuelta crc32=",
			int(a2.crc), a2.pixeles, "pixeles")
	}

	println("ZONAS: dt1s aciertos=", cuentaDT1s.aciertos, "faltas=", cuentaDT1s.faltas,
		"evicto=", caches.Dt1s.Evictions(), "pesoEnCache=", caches.Dt1s.GetWeight(), "de", caches.Dt1s.GetBudget())
	println("ZONAS: ds1s aciertos=", cuentaDS1s.aciertos, "faltas=", cuentaDS1s.faltas,
		"evicto=", caches.Ds1s.Evictions(), "pesoEnCache=", caches.Ds1s.GetWeight(), "de", caches.Ds1s.GetBudget())

	// DC-002.78: el arena de tiles del motor -- capacidad pedida UNA vez, cuanto avanzo el cursor,
	// cuantas veces hubo que tirarlo (cada reinicio vacia el cache de dt1s) y cuantos .dt1 se leyeron.
	estadoArena := d2asset.EstadoDeLosTiles()
	println("ZONAS: ARENA de tiles del motor: caminoDeConsola=", estadoArena.CaminoDeConsola,
		"capacidad=", estadoArena.Capacidad, "usado=", estadoArena.Usado,
		"reinicios=", estadoArena.Reinicios, "archivos=", estadoArena.ArchivosLeidos)

	// DC-002.80: la memoria de trabajo de la LECTURA -- pedida una vez y reusada entre archivos y zonas.
	// `reservado` es lo que ocupa (bytes) y `crecidas` cuantas veces hubo que pedir mas: si crece durante la
	// carga, el prestamo se quedo corto y eso es justo lo que hay que arreglar antes de prender la ruta.
	if estPrestamo, hay := d2asset.EstadoDelPrestamoDeTiles(); hay {
		println("ZONAS: PRESTAMO de la lectura: lecturas=", estPrestamo.Lecturas,
			"sectores=", estPrestamo.Sectores, "reservado=", estPrestamo.Reservado,
			"crecidas=", estPrestamo.Crecidas)
	}

	println("ZONAS: monton al final: " + estado())

	// ============================== EL CUADRO ==============================
	// entrar y salir de tres zonas no puede dejar el PVR roto: se dibuja la ultima y se informa.
	if err := renderer.Run(dibujar, actualizar, ancho, alto, "Diablo II (Dreamcast)"); err != nil {
		println("ZONAS: el ciclo de cuadros salio con error:", err.Error())
	}
}

// zonaCargada: lo que devuelve una fase (el lienzo se queda para dibujar).
type zonaCargada struct {
	canvas []byte

	ms        int
	crc       uint32
	pixeles   int
	usadoPico int
	arenaPico int
	libreMin  int
}

// vigilante: el pico del monton DENTRO de una carga. Se muestrea en cada paso (crudo leido, parseado,
// insertado) en vez de solo al final: el crudo y lo parseado viven juntos un instante y ese es el pico.
type vigilante struct {
	usadoPico int
	arenaPico int
	libreMin  int
	dondePico string
}

func nuevoVigilante() *vigilante {
	return &vigilante{
		usadoPico: 0,
		arenaPico: 0,
		libreMin:  1 << 30,
	}
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

// cargarZona: la secuencia del motor para entrar a una zona -- pedir el .ds1 al cache, armar la ventana
// y alimentar los .dt1 de su lista en el orden en que el DS1 los declara -- con el pico medido paso a
// paso y el reloj del hardware alrededor.
func cargarZona(mpq *d2mpq.MPQ, rotulo, ruta string, centroX, centroY int) *zonaCargada {
	println("")
	println("ZONAS: ===== " + rotulo + " =====")

	// el monton ANTES de la fase, con el recolector ya corrido: asi el pico de la fase no incluye la
	// basura que dejo la fase anterior (esa basura se informa aparte, es lo que el GC devuelve).
	usadoAntesGC := d2cMontonUsado()
	runtime.GC()
	usadoTrasGC := d2cMontonUsado()

	println("ZONAS: monton antes de la fase: usado=", usadoAntesGC, "tras GC=", usadoTrasGC,
		"(el GC devolvio", usadoAntesGC-usadoTrasGC, "bytes)")

	v := nuevoVigilante()
	v.muestra("arranque de la fase")

	t0 := d2cMs()

	d, crudoDS1 := cargarDS1(mpq, ruta, v)
	if d == nil {
		return nil
	}

	f := &medida{
		rotulo:        rotulo,
		ruta:          ruta,
		ds1Crudo:      crudoDS1,
		ds1Vivo:       d2asset.TamanoDS1(d),
		files:         len(d.Files),
		eviccionesDS1: caches.Ds1s.Evictions(),
		pesoDS1:       caches.Ds1s.GetWeight(),
		aciertosDS1:   cuentaDS1s.aciertos,
		faltasDS1:     cuentaDS1s.faltas,
	}

	println("ZONAS: ds1 version=", d.Version(), "acto=", int(d.Act), "w=", d.Width(), "h=", d.Height(),
		"files=", len(d.Files), "crudo=", f.ds1Crudo, "vivo=", f.ds1Vivo, "monton=", estado())

	n := NuevoNivelEn(d, ladoVentana, ladoVentana, centroX, centroY, &reusoDelNivel)

	// el compositor devuelve los MISMOS arreglos: se los guardamos para la zona siguiente (DC-002.78)
	reusoDelNivel.Canvas = n.Canvas
	reusoDelNivel.Scratch = n.Scratch
	reusoDelNivel.Pedidos = n.Pedidos

	for _, nombreDelDS1 := range d.Files {
		nombre := RutaDelDT1(nombreDelDS1)

		dt1, crudo := cargarDT1(mpq, nombre, v)
		if dt1 == nil {
			continue
		}

		n.Alimentar(dt1.Tiles)

		dt1 = nil
		v.muestra("tras alimentar " + nombre)

		// camino sin cache: los bloques del .dt1 son subslices del crudo, asi que se libera recien aca,
		// cuando ya se compuso todo lo que salia de este archivo (ver knobs.go).
		if sinCacheDt1 && crudo != 0 {
			d2cLibre(crudo)
			crudo = 0
			v.muestra("tras liberar " + nombre)
		}

		// el crudo ya se solto y los tiles ya estan compuestos: aca la basura del recolector es maxima.
		// Con la palanca prendida se le pide el GC (ver knobs.go); apagada, se deja al runtime.
		if gcEntreArchivos {
			runtime.GC()
			v.muestra("tras GC de " + nombre)
		}
	}

	n.Cerrar()

	f.ms = int(d2cMs() - t0)
	f.pedidos = n.PisosPedidos + n.ParedesPedidas
	f.compuesto = n.PisosCompuestos + n.ParedesCompuestas
	f.pixeles = n.PixelesNoVacios()
	f.crc = n.CRC32()
	f.eviccionesDT1 = caches.Dt1s.Evictions()
	f.pesoDT1 = caches.Dt1s.GetWeight()
	f.aciertosDT1 = cuentaDT1s.aciertos
	f.faltasDT1 = cuentaDT1s.faltas
	f.usadoAlEmpezar = usadoTrasGC
	f.usadoPico = v.usadoPico
	f.arenaPico = v.arenaPico
	f.libreMin = v.libreMin
	f.dondePico = v.dondePico
	f.usadoAlFinal = d2cMontonUsado()
	f.libreAlFinal = d2cMontonLibre()

	println("ZONAS: VENTANA lado=", ladoVentana, "origen=", n.OrigenX, n.OrigenY,
		"pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas)
	println("ZONAS: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("ZONAS: LIENZO pixelesNoVacios=", f.pixeles, "de", n.Ancho*n.Alto, "crc32=", int(f.crc))

	println("ZONAS: " + rotulo + " PICO usado=" + itoa(f.usadoPico) + " arena=" + itoa(f.arenaPico) +
		" libreMin=" + itoa(f.libreMin) + " (el pico fue en: " + f.dondePico + ")")
	println("ZONAS: " + rotulo + " FIN usado=" + itoa(f.usadoAlFinal) + " libre=" + itoa(f.libreAlFinal) +
		" ms=" + itoa(f.ms))
	println("ZONAS: " + rotulo + " dt1s evicto=" + itoa(f.eviccionesDT1) + " peso=" + itoa(f.pesoDT1) +
		" de " + itoa(caches.Dt1s.GetBudget()) + " | faltas=" + itoa(f.faltasDT1) + " aciertos=" + itoa(f.aciertosDT1))
	println("ZONAS: " + rotulo + " ds1s evicto=" + itoa(f.eviccionesDS1) + " peso=" + itoa(f.pesoDS1) +
		" de " + itoa(caches.Ds1s.GetBudget()) + " | faltas=" + itoa(f.faltasDS1) + " aciertos=" + itoa(f.aciertosDS1))

	return &zonaCargada{
		canvas:    n.Canvas,
		ms:        f.ms,
		crc:       f.crc,
		pixeles:   f.pixeles,
		usadoPico: f.usadoPico,
		arenaPico: f.arenaPico,
		libreMin:  f.libreMin,
	}
}

// cargarDS1: el camino del motor -- Retrieve del cache, y si falta, leer del MPQ, parsear e Insert con
// el peso en bytes. Devuelve nil si no se pudo.
//
// DC-002.81: con la palanca `porMotor` el .ds1 lo carga el MOTOR (`am.LoadDS1`), que es el camino real del
// juego: `LoadFileHandoff` -> `LoadFile`, o sea la lectura con el tamano reservado de DC-002.79 (y sin el
// prefetch del MPQ). El DS1 del pueblo mide 57.652 B, debajo del umbral de 64 KB del monton externo, pero es
// el archivo que dice QUE .dt1 hay que cargar y es el que en el motor se pide por esta funcion.
func cargarDS1(mpq *d2mpq.MPQ, clave string, v *vigilante) (*d2ds1.DS1, int) {
	if porMotor {
		return cargarDS1PorElMotor(clave, v)
	}

	if valor, encontrado := caches.Ds1s.Retrieve(clave); encontrado {
		cuentaDS1s.aciertos++
		v.muestra("ds1 del cache")

		return valor.(*d2ds1.DS1), 0
	}

	cuentaDS1s.faltas++
	v.muestra("ds1 antes de leer")

	raw, err := LeerDelMPQ(mpq, clave)
	if err != nil {
		println("ZONAS: DS1-LEER FALLO:", clave, err.Error())

		return nil, 0
	}

	v.muestra("ds1 crudo leido")

	d, err := d2ds1.Unmarshal(raw)
	if err != nil {
		println("ZONAS: DS1-PARSE FALLO:", err.Error())

		return nil, 0
	}

	v.muestra("ds1 parseado")

	peso := d2asset.TamanoDS1(d)
	crudo := len(raw)

	if err := caches.Ds1s.Insert(clave, d, d2asset.Pesar(peso)); err != nil {
		println("ZONAS: DS1-INSERT FALLO:", err.Error())

		return nil, 0
	}

	v.muestra("ds1 insertado")
	println("ZONAS: DS1-INSERT crudo=", crudo, "peso=", peso, "monton=", estado())

	raw = nil

	return d, crudo
}

// cargarDS1PorElMotor (DC-002.81): el .ds1 por `am.LoadDS1`, el mismo camino que usa el motor. El crudo se
// informa con el tamano que declara la fuente (`TamanoDeArchivo`), que es lo que el motor reserva.
func cargarDS1PorElMotor(clave string, v *vigilante) (*d2ds1.DS1, int) {
	relativa := ClaveRelativaDelDS1(clave)

	if _, encontrado := caches.Ds1s.Retrieve(relativa); encontrado {
		cuentaDS1s.aciertos++
		v.muestra("ds1 del cache")
	} else {
		cuentaDS1s.faltas++
		v.muestra("ds1 antes de leer (motor)")
	}

	d, err := am.LoadDS1(relativa)
	if err != nil {
		println("ZONAS: DS1-LEER FALLO (motor):", relativa, err.Error())

		return nil, 0
	}

	v.muestra("ds1 leido por el motor")

	crudo := 0
	if tamano, sabido := am.TamanoDeArchivo("/data/global/tiles/" + relativa); sabido {
		crudo = tamano
	}

	println("ZONAS: DS1-INSERT (motor) clave=", relativa, "crudo=", crudo, "vivo=", d2asset.TamanoDS1(d),
		"monton=", estado())

	return d, crudo
}

// cargarDT1: lo mismo para un .dt1: es donde el monton se mueve (el archivo mas grande mide 1 MB crudo).
// Devuelve tambien el puntero al crudo (0 si no hay que liberarlo): con el camino sin cache, el que llama
// lo libera despues de componer.
//
// DC-002.78: con la palanca `porBuffer` el archivo lo carga el MOTOR (`am.LoadDT1`), que en la consola lee
// cada .dt1 ADENTRO de un arena pedido una sola vez (d2asset/lectura_de_tiles.go): la carga de una zona deja
// de pedir memoria por archivo. Sin la palanca, el camino de siempre: `mpq.ReadFile` + `LoadDT1`, que deja
// el crudo tomado para siempre (medido en DC-002.77).
func cargarDT1(mpq *d2mpq.MPQ, clave string, v *vigilante) (*d2dt1.DT1, uintptr) {
	if !sinCacheDt1 {
		if valor, encontrado := caches.Dt1s.Retrieve(clave); encontrado {
			cuentaDT1s.aciertos++

			return valor.(*d2dt1.DT1), 0
		}
	}

	cuentaDT1s.faltas++
	v.muestra("antes de leer " + clave)

	var (
		dt1   *d2dt1.DT1
		raw   []byte
		crudo uintptr
	)

	// DC-002.81: el camino del MOTOR tambien cuando se mide el arranque del motor (`porMotor`). Sin esto, la
	// corrida del motor cargaba los .dt1 por el camino viejo del POC (mpq.ReadFile + d2dt1.LoadDT1), que deja
	// el crudo tomado por archivo -- MEDIDO: usado 5.064.336 -> 9.902.840 en la fase (a) y `gc_external_alloc:
	// OOM` al volver al pueblo. Con esta condicion el .dt1 entra por `am.LoadDT1`, que es el camino real.
	if porBuffer || porMotor {
		// la ruta del MOTOR: arena. El tamano del archivo lo informa el instrumento del arena (no hay
		// crudo que medir de este lado).
		//
		// DC-002.78: con la palanca `gcInhibido` se FRENA el recolector durante la carga. Medido: sin el
		// freno, el recolector salta por umbral en medio del parseo y el SH-4 salta a una direccion de
		// monton (PC fuera del segmento de codigo del ELF). Ver el porque en csrc/shim.c.
		if gcInhibido {
			d2cGcFrena()
		}

		dt1Cargado, err := am.LoadDT1(ClaveRelativaDelDT1(clave))

		if gcInhibido {
			d2cGcSuelta()
		}

		if err != nil {
			println("ZONAS: DT1", clave, "FALLO (motor):", err.Error())

			return nil, 0
		}

		dt1 = dt1Cargado

		v.muestra("cargado por el motor " + clave)
	} else {
		var err error

		raw, err = LeerDelMPQ(mpq, clave)
		if err != nil {
			println("ZONAS: DT1", clave, "FALLO:", err.Error())

			return nil, 0
		}

		v.muestra("crudo leido " + clave)

		dt1, err = d2dt1.LoadDT1(raw)
		if err != nil {
			println("ZONAS: DT1", clave, "PARSE-FALLO:", err.Error())

			return nil, 0
		}

		if len(raw) > 0 {
			crudo = uintptr(unsafe.Pointer(&raw[0]))
		}

		v.muestra("parseado " + clave)
	}

	peso := d2asset.TamanoDT1(dt1)

	if sinCacheDt1 {
		// sin cache: el crudo se devuelve para que el que llama lo libere tras componer
		println("ZONAS: DT1", clave, "crudo=", len(raw), "vivo=", peso, "tiles=", len(dt1.Tiles),
			"(SIN CACHE: se libera al componer)", "monton=", estado())

		return dt1, crudo
	}

	if porBuffer || porMotor {
		// el motor YA lo inserto en el cache: aca solo se informa como quedo
		println("ZONAS: DT1", clave, "tiles=", len(dt1.Tiles), "vivo=", peso,
			"evicto=", caches.Dt1s.Evictions(), "pesoCache=", caches.Dt1s.GetWeight(), "monton=", estado())

		return dt1, 0
	}

	if err := caches.Dt1s.Insert(clave, dt1, d2asset.Pesar(peso)); err != nil {
		println("ZONAS: DT1", clave, "INSERT-FALLO:", err.Error())

		return nil, 0
	}

	v.muestra("insertado " + clave)
	println("ZONAS: DT1", clave, "crudo=", len(raw), "vivo=", peso, "tiles=", len(dt1.Tiles),
		"evicto=", caches.Dt1s.Evictions(), "monton=", estado())

	raw = nil

	return dt1, 0
}

// ClaveRelativaDelDT1: lo que pide `am.LoadDT1` -- la ruta del .dt1 SIN el prefijo "/data/global/tiles/",
// que el propio LoadDT1 antepone.
func ClaveRelativaDelDT1(ruta string) string {
	return strings.TrimPrefix(ruta, "data/global/tiles/")
}

// ClaveRelativaDelDS1 (DC-002.81): lo que pide `am.LoadDS1` -- la ruta del .ds1 en barras normales, en
// minusculas y SIN el prefijo "/data/global/tiles/", que el propio LoadDS1 antepone (es la misma forma que
// trae `lvlprest.txt`: `act1/town/townn1.ds1`).
func ClaveRelativaDelDS1(ruta string) string {
	normal := strings.ToLower(strings.ReplaceAll(ruta, barraInvertida, "/"))

	return strings.TrimPrefix(normal, "data/global/tiles/")
}

// estado: los cuatro numeros del monton de C, juntos.
func estado() string {
	return "arena=" + itoa(d2cMontonArena()) + " usado=" + itoa(d2cMontonUsado()) +
		" libre=" + itoa(d2cMontonLibre()) + " sbrk=0x" + hex32(d2cSbrkActual()) +
		" techo=0x" + hex32(d2cMontonTecho())
}

// dibujar: el MISMO callback que DC-002.70/75, con el lienzo de la ultima zona visitada.
func dibujar(superficie d2interface.Surface) error {
	if mapaPintado {
		return nil
	}

	if len(mapaIndices) == 0 {
		return nil
	}

	malla, ok := superficie.(*pvr.Surface)
	if !ok {
		return errorDelPoc("el Renderer no entrego una Surface del backend PVR")
	}

	malla.Clear(color.RGBA{R: 0, G: 0, B: 0, A: 255})

	malla.PushTranslation(mapaX, mapaY)
	malla.RenderIndices(mapaIndices, ladoVentana, ladoVentana, paletaDelMapa)
	malla.Pop()

	mapaPintado = true

	pixeles := malla.Imagen().Pixeles()
	llenos := 0

	for j := 0; j+3 < len(pixeles); j += 4 {
		if pixeles[j] != 0 || pixeles[j+1] != 0 || pixeles[j+2] != 0 {
			llenos++
		}
	}

	anchoReal, altoReal := superficie.GetSize()

	println("ZONAS: CUADRO pixelesNoVacios=", llenos, "de", anchoReal*altoReal,
		"crc32=", int(crc32.ChecksumIEEE(pixeles)), "tamano=", anchoReal, altoReal,
		"montonLibre=", d2cMontonLibre())

	return nil
}

// actualizar: a los cuadrosCuadro cuadros informa como quedo el monton con el cuadro andando.
func actualizar() error {
	cuadros++

	if cuadros == cuadrosCuadro {
		println("ZONAS: DURANTE", cuadros, "CUADROS: monton:", estado(),
			"dt1sPeso=", caches.Dt1s.GetWeight(), "ds1sPeso=", caches.Ds1s.GetWeight())
	}

	return nil
}

// ============================== utilidades (sin el paquete strconv, que en la consola esta de mas) ======

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

// errorDelPoc: un error con nombre, para que el que llama pueda distinguirlo de uno del motor.
type errorDelPoc string

func (e errorDelPoc) Error() string { return string(e) }
