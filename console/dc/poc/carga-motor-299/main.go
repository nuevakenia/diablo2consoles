//go:build dreamcast

// DC-002.99 -- EL ULTIMO `make` POR ARCHIVO: `LoadFile` del motor con dueno, en la Dreamcast.
//
// QUE PRUEBA (el criterio de la tarjeta, con numeros):
//
//  1. LOS OCHO .dt1 DE UNA ZONA POR EL CAMINO DEL MOTOR (`d2asset.LoadFile`, la puerta de entrada de TODAS
//     las cargas grandes) pidiendo UN SOLO tramo de memoria para los ocho, y el MONTON DE C volviendo al
//     nivel de partida al soltarlo (`SoltarLecturasDeConsola`).
//  2. LA PRUEBA EN CONTRA, con el mismo codigo y el mismo binario: los mismos ocho archivos por `LoadFile`
//     SIN la palanca del tramo (el camino de DC-002.79: un `make` por archivo). Ahi el monton de C NO vuelve,
//     y lo que se queda tomado es lo que mide cada archivo. Si los dos bloques dieran igual, el instrumento
//     no estaria midiendo el dueno.
//  3. LA ZONA NO CAMBIA: el pueblo del acto 1 compone igual que en DC-002.38/.81/.88/.96, crc32 3507647456.
//  4. EL DUEÑO DEVUELVE TODO: en la fuente, `BytesVivos == 0` al cerrar (los tramos que se pidieron se
//     devolvieron).
//
// EL ORDEN DE LOS BLOQUES ES POR RAM, NO POR GUSTO: la zona va PRIMERO (necesita el monton ordenado y sus
// buffers de compositor), despues el tramo (la tarjeta), y ULTIMO el bloque sin dueno, que es el unico que
// deja memoria tomada a proposito (los ocho `.dt1` suman 4.350.665 B que no vuelven).
//
// MUTANTE (`NO_LIBERA=1`, lo pone el script al compilar): la fuente no tiene liberador, asi que `Liberar` no
// devuelve el bloque. Con el mutante el bloque del tramo tiene que decir lo mismo que el bloque sin dueno.
//
// LOS DOS GERENTES. `LoadFile` no necesita un AssetManager completo: necesita un `d2loader.Loader` con sus
// fuentes. Se arman dos, livianos (sin caches ni tablas de records, que en la consola son la pared de
// DC-002.82), sobre el MISMO MPQ abierto:
//
//	sinTramos  la fuente de consola ENVUELTA sin `Prestar` -> `LoadFile` no puede pedir tramo y reserva un
//	           buffer por archivo. Es el camino de hoy, medido con el codigo de hoy.
//	conTramos  la fuente de consola tal cual, con `Prestar` -> `LoadFile` lee en UN tramo y lo reusa.
package main

import (
	"hash/crc32"
	"io"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
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

const (
	rutaMPQ = "/cd/JUEGO/D2DATA.MPQ"

	// el pueblo del acto 1: la MISMA zona de DC-002.38/70/75/77/81/88/96, para que el crc32 del lienzo se
	// compare contra el numero ya establecido
	ds1Pueblo          = "data/global/tiles/ACT1/TOWN/townN1.ds1"
	centroPuebloX      = 28
	centroPuebloY      = 20
	ladoDeLaVentana    = 512
	crcDelPuebloEspera = 3507647456

	// el margen del monton: 4 KB. Un buffer que NO se devolvio se ve seguro (los .dt1 del pueblo pesan cientos
	// de miles de bytes); el margen es para el encabezado del asignador.
	margenDelMonton = 4 * 1024
)

var (
	// fuente: la Source de consola de la corrida (un solo MPQ abierto para todo).
	fuente *fuenteconsola.Fuente

	// baseDelMonton: el `uordblks` de referencia de cada bloque, medido DESPUES del recolector.
	baseDelMonton int
)

func main() {
	d2cSerialInit()

	println("")
	println("CARGA299: arranca DC-002.99 -- LoadFile del motor por el tramo prestado")
	println("CARGA299: palancas: noLibera=", noLibera)
	println("CARGA299: monton al arrancar: " + estado())

	runtime.GC()

	baseDelMonton = d2cMontonUsado()
	println("CARGA299: base del monton (tras GC) usado=", baseDelMonton, "libre=", d2cMontonLibre())

	f, err := fuenteconsola.Abrir(rutaMPQ)
	if err != nil {
		println("CARGA299: MPQ-ABRIR FALLO:", rutaMPQ, err.Error())

		return
	}

	fuente = f

	// EL DUENO: con que se devuelve al monton un tramo grande (los .dt1 pasan los 64 KB, o sea monton externo)
	if !noLibera {
		fuente.UsarLiberador(fuenteconsola.Liberador(d2cLibre))
	}

	// EL PRESTAMO SE CALIENTA ANTES DE MEDIR: la primera lectura pide la memoria de trabajo (14 KB). Si eso
	// cayera adentro de un bloque medido, el delta traeria el prestamo y no el tramo.
	if tramo, err := fuente.Pedir(`data/global/tiles/ACT1/TOWN/floor.dt1`); err == nil {
		tramo.Liberar()
	}

	println("CARGA299: MPQ abierto, liberador=", !noLibera, "prestamo=", fuente.Estado().PrestamoReservado,
		"monton="+estado())

	// LAS PALANCAS DEL MOTOR: la lectura de consola (DC-002.79) y el tramo (DC-002.99). En el motor de verdad
	// las prende `d2asset.ArrancarComoConsola`, que es una sola linea.
	d2asset.ActivarLecturaDeConsola()
	d2asset.ActivarLecturasPorTramos()

	e := d2asset.EstadoDeLosTramos()
	println("CARGA299: palancas del motor: lecturaDeConsola=true tramos=", e.Camino)

	// los dos gerentes livianos, sobre el mismo MPQ
	amSinTramo := gerenteDe(&fuenteDeHoy{f: fuente})
	amConTramo := gerenteDe(fuente)

	// ============================== C) LA ZONA ==============================
	crcZona, nombres, okZona := cargarLaZona()

	// ============================== B) EL TRAMO (la tarjeta) ==============================
	okTramo := bloqueDelTramo(amConTramo, nombres)

	// ============================== A) SIN DUENO (el camino de hoy) ==============================
	tomadoSinDueno := bloqueSinDueno(amSinTramo, nombres)

	// ============================== EL VEREDICTO ==============================
	runtime.GC()

	es := fuente.Estado()
	et := d2asset.EstadoDeLosTramos()

	println("")
	println("CARGA299: ================ EL VEREDICTO ================")
	println("CARGA299: instrumento del tramo: pedidos=", et.Pedidos, "liberados=", et.Liberados,
		"reusos=", et.Reusos, "lecturas=", et.Lecturas, "bytes=", int(et.Bytes),
		"bytesDelTramo=", et.BytesDelTramo)
	println("CARGA299: instrumento de la fuente: pedidos=", es.Pedidos, "liberados=", es.Liberados,
		"prestamos=", es.Prestamos, "bytesVivos=", int(es.BytesVivos), "externos=", es.LibresExternos,
		"dobles=", es.LiberadosDobles, "prestamo=", es.PrestamoReservado, "crecidas=", es.PrestamoCrecidas)
	println("CARGA299: monton al final: usado=", d2cMontonUsado(), "libre=", d2cMontonLibre(),
		" (el bloque A deja tomado lo que no se libera:", tomadoSinDueno, "bytes)")

	if es.Pedidos != es.Liberados || es.Prestamos != es.PrestamosLiberados || es.BytesVivos != 0 {
		println("CARGA299: DUENO MAL -- archivos pedidos=", es.Pedidos, "devueltos=", es.Liberados,
			"tramos de espacio pedidos=", es.Prestamos, "devueltos=", es.PrestamosLiberados,
			"; quedan", int(es.BytesVivos), "bytes vivos")
	} else {
		println("CARGA299: DUENO OK -- todos los tramos que se pidieron se devolvieron (0 bytes vivos)")
	}

	if okZona {
		println("CARGA299: ZONA OK -- el pueblo compone igual: crc32", int(crcZona), "==", crcDelPuebloEspera)
	} else {
		println("CARGA299: ZONA MAL -- el pueblo NO compone igual (crc32", int(crcZona), "contra",
			crcDelPuebloEspera, ")")
	}

	if okTramo {
		println("CARGA299: TRAMO OK -- los ocho .dt1 del pueblo por LoadFile con el monton de C de vuelta a la base")
	} else {
		println("CARGA299: TRAMO MAL -- los ocho .dt1 por LoadFile NO dejaron el monton en la base")
	}

	if noLibera {
		println("CARGA299: MUTANTE noLibera=true -- la corrida de ARRIBA no vale como prueba del dueno")
	}

	// el POC se queda quieto: si `main` vuelve, el runtime de la consola reinicia la maquina y la corrida se
	// repite (medido en DC-002.79)
	for {
		d2cMs()
	}
}

// fuenteDeHoy: la fuente de consola ENVUELTA sin `Prestar`. O sea: las tres interfaces de lectura de siempre
// (Source, SourceDeLecturaEntera y SourceDeLecturaConPrestamo) y NADA MAS.
//
// PARA QUE. El bloque A tiene que medir el camino de HOY con el codigo de hoy: `d2asset.loadFileDeConsola` con
// su `make([]byte, tamano)` por archivo. No se puede "apagar la palanca" para eso (es una variable de paquete
// del motor), pero si se le puede dar al motor una fuente que no sepa prestar memoria: entonces
// `Loader.TieneTramos` dice que no, y `LoadFile` reserva por archivo, que es exactamente lo que hacia ayer.
type fuenteDeHoy struct {
	f *fuenteconsola.Fuente
}

var (
	_ asset.Source                     = (*fuenteDeHoy)(nil)
	_ asset.SourceDeLecturaEntera      = (*fuenteDeHoy)(nil)
	_ asset.SourceDeLecturaConPrestamo = (*fuenteDeHoy)(nil)
)

func (x *fuenteDeHoy) Path() string                              { return x.f.Path() }
func (x *fuenteDeHoy) String() string                            { return x.f.String() }
func (x *fuenteDeHoy) Exists(nombre string) bool                 { return x.f.Exists(nombre) }
func (x *fuenteDeHoy) Open(nombre string) (io.ReadSeeker, error) { return x.f.Open(nombre) }
func (x *fuenteDeHoy) Tamano(nombre string) (int, bool)          { return x.f.Tamano(nombre) }

func (x *fuenteDeHoy) LeerEnteroEn(nombre string, dst []byte) (int, error) {
	return x.f.LeerEnteroEn(nombre, dst)
}

func (x *fuenteDeHoy) LeerEnteroEnConPrestamo(nombre string, dst []byte, p *d2mpq.Scratch) (int, error) {
	return x.f.LeerEnteroEnConPrestamo(nombre, dst, p)
}

// gerenteDe: el AssetManager minimo que necesita `LoadFile` -- un `d2loader.Loader` con sus fuentes. Sin
// caches ni tablas de records: lo que se mide es la LECTURA, no el motor entero (que es DC-002.88/.93).
func gerenteDe(fuentes ...asset.Source) *d2asset.AssetManager {
	l, err := d2loader.NewLoader(0)
	if err != nil {
		println("CARGA299: el loader no se pudo armar:", err.Error())

		return nil
	}

	l.Sources = fuentes

	return &d2asset.AssetManager{Loader: l}
}

// bloqueDelTramo: LOS OCHO .dt1 POR `d2asset.LoadFile` CON LA PALANCA DEL TRAMO (DC-002.99).
//
// Lo que se mide: el monton de C antes y despues de cada archivo (tiene que haber UN solo pedido grande, el
// del primer archivo, y crecimientos cuando aparece uno mas grande), y el monton al soltar el tramo, que tiene
// que volver EXACTO al de la base.
func bloqueDelTramo(am *d2asset.AssetManager, nombres []string) bool {
	println("")
	println("CARGA299: ===== B) LOS OCHO .dt1 POR LoadFile CON TRAMO (DC-002.99) =====")

	if am == nil {
		return false
	}

	runtime.GC()

	antes := d2cMontonUsado()
	etAntes := d2asset.EstadoDeLosTramos()

	crcs := make([]uint32, 0, len(nombres))

	for _, nombre := range nombres {
		medidoAntes := d2cMontonUsado()

		datos, err := am.LoadFile(nombre)
		if err != nil {
			println("CARGA299: TRAMO FALLO:", nombre, err.Error())

			return false
		}

		crcs = append(crcs, crc32.ChecksumIEEE(datos))

		medidoDespues := d2cMontonUsado()

		et := d2asset.EstadoDeLosTramos()

		println("CARGA299: TRAMO", archivo(nombre), "bytes=", len(datos), "crc32=", int(crc32.ChecksumIEEE(datos)),
			"monton antes=", medidoAntes, "despues=", medidoDespues, "delta=", medidoDespues-medidoAntes,
			"tramo=", et.BytesDelTramo, "reusos=", et.Reusos-etAntes.Reusos, "pedidos=", et.Pedidos-etAntes.Pedidos)
	}

	conElTramo := d2cMontonUsado()

	// SOLTAR: el tramo vuelve al monton
	d2asset.SoltarLecturasDeConsola()

	runtime.GC()

	despues := d2cMontonUsado()

	et := d2asset.EstadoDeLosTramos()

	volvio := despues-antes <= margenDelMonton
	unTramo := et.Pedidos-etAntes.Pedidos >= 1 && et.Pedidos-etAntes.Pedidos <= 3
	reusos := et.Reusos - etAntes.Reusos

	println("CARGA299: TRAMO monton antes=", antes, "con los ocho=", conElTramo, "tras soltar=", despues,
		"delta=", despues-antes, "margen=", margenDelMonton, "volvioALaBase=", volvio)
	println("CARGA299: TRAMO instrumento: pedidos=", et.Pedidos-etAntes.Pedidos, "reusos=", reusos,
		"bytesDelTramo=", et.BytesDelTramo, "unTramo=", unTramo)

	if volvio && unTramo && reusos == len(nombres)-(et.Pedidos-etAntes.Pedidos) {
		println("CARGA299: TRAMO OK -- ", len(nombres), "archivos en", et.Pedidos-etAntes.Pedidos,
			"tramos y el monton volvio al nivel de partida")

		return true
	}

	println("CARGA299: TRAMO MAL -- delta=", despues-antes, "pedidos=", et.Pedidos-etAntes.Pedidos,
		"reusos=", reusos, "de", len(nombres), "archivos")

	return false
}

// bloqueSinDueno: LOS MISMOS OCHO .dt1 POR `d2asset.LoadFile` SIN LA PALANCA -- el camino de ayer, con un
// `make([]byte, tamano)` por archivo que no libera nadie. Es la prueba en contra del instrumento, y va al final
// porque es el unico bloque que deja el monton tomado.
func bloqueSinDueno(am *d2asset.AssetManager, nombres []string) int {
	println("")
	println("CARGA299: ===== A) LOS MISMOS OCHO SIN DUENO (el camino de ayer) =====")

	if am == nil {
		return 0
	}

	runtime.GC()

	antes := d2cMontonUsado()

	var crudo [][]byte

	for _, nombre := range nombres {
		datos, err := am.LoadFile(nombre)
		if err != nil {
			println("CARGA299: SINDUENO FALLO:", nombre, err.Error())

			continue
		}

		crudo = append(crudo, datos)

		println("CARGA299: SINDUENO", archivo(nombre), "bytes=", len(datos),
			"crc32=", int(crc32.ChecksumIEEE(datos)), "monton=", d2cMontonUsado())
	}

	runtime.GC()

	despues := d2cMontonUsado()
	tomado := despues - antes

	println("CARGA299: SINDUENO monton antes=", antes, "despues=", despues, "tomado=", tomado,
		"(un buffer por archivo que no vuelve)")

	// los buffers NO se liberan a proposito: es la prueba
	crudo = nil

	return tomado
}

// cargarLaZona: el pueblo del acto 1 compuesto como en DC-002.96 (la fuente con `Pedir` y el compositor
// compartido `nivel.go`), y sus ocho `.dt1` devueltos como lista para los otros dos bloques.
func cargarLaZona() (uint32, []string, bool) {
	println("")
	println("CARGA299: ===== C) LA ZONA (el pueblo) =====")

	t0 := d2cMs()

	tramoDS1, err := fuente.Pedir(ds1Pueblo)
	if err != nil {
		println("CARGA299: DS1 PEDIR FALLO:", err.Error())

		return 0, nil, false
	}

	bytesDS1 := len(tramoDS1.Datos)
	crcDS1 := crc32.ChecksumIEEE(tramoDS1.Datos)

	d, err := d2ds1.Unmarshal(tramoDS1.Datos)
	if err != nil {
		println("CARGA299: DS1 PARSE FALLO:", err.Error())

		tramoDS1.Liberar()

		return 0, nil, false
	}

	tramoDS1.Liberar()

	println("CARGA299: DS1 bytes=", bytesDS1, "crc32=", int(crcDS1), "w=", d.Width(), "h=", d.Height(),
		"archivos=", len(d.Files))

	n := NuevoNivelEn(d, ladoDeLaVentana, ladoDeLaVentana, centroPuebloX, centroPuebloY, &reusoDelNivel)

	nombres := make([]string, 0, len(d.Files))
	leidos := 0

	for _, nombreDelDS1 := range d.Files {
		nombre := RutaDelDT1(nombreDelDS1)
		nombres = append(nombres, nombre)

		antes := d2cMontonUsado()

		tramo, err := fuente.Pedir(nombre)
		if err != nil {
			println("CARGA299: DT1 PEDIR FALLO:", nombre, err.Error())

			continue
		}

		dt1, err := d2dt1.LoadDT1(tramo.Datos)
		if err != nil {
			println("CARGA299: DT1 PARSE FALLO:", nombre, err.Error())

			tramo.Liberar()

			continue
		}

		n.Alimentar(dt1.Tiles)

		// el tramo se devuelve DESPUES de componer: los bloques del .dt1 apuntaban adentro
		dt1 = nil
		tramo.Liberar()

		leidos++

		despues := d2cMontonUsado()

		println("CARGA299: DT1", archivo(nombre), "monton antes=", antes, "despues=", despues,
			"delta=", despues-antes)
	}

	n.Cerrar()

	crc := n.CRC32()

	println("CARGA299: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("CARGA299: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto,
		"crc32=", int(crc), "dt1 leidos=", leidos, "de", len(d.Files), "ms=", int(d2cMs()-t0))

	// los buffers del compositor se reusan entre zonas (DC-002.78)
	reusoDelNivel.Canvas = n.Canvas
	reusoDelNivel.Scratch = n.Scratch
	reusoDelNivel.Pedidos = n.Pedidos

	runtime.GC()

	println("CARGA299: monton tras la zona: usado=", d2cMontonUsado(), "libre=", d2cMontonLibre())

	return crc, nombres, crc == crcDelPuebloEspera && leidos == len(d.Files)
}

// reusoDelNivel: los buffers del compositor, que se reusan (ver nivel.go, BuffersDelNivel).
var reusoDelNivel BuffersDelNivel

// archivo: el nombre corto, para que las lineas entren en el registro serie.
func archivo(ruta string) string {
	for i := len(ruta) - 1; i >= 0; i-- {
		if ruta[i] == '/' {
			return ruta[i+1:]
		}
	}

	return ruta
}

// estado: los cuatro numeros del monton de C, juntos.
func estado() string {
	return "arena=" + itoa(d2cMontonArena()) + " usado=" + itoa(d2cMontonUsado()) +
		" libre=" + itoa(d2cMontonLibre()) + " sbrk=0x" + hex32(d2cSbrkActual()) +
		" techo=0x" + hex32(d2cMontonTecho())
}

// itoa: sin el paquete strconv, que en la consola esta de mas.
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
