//go:build dreamcast

// DC-002.79 -- LEER LOS ARCHIVOS GRANDES POR EL CAMINO DEL MOTOR (`LoadFile`), EN LA CONSOLA (Fase C.1c).
//
// QUE PRUEBA. DC-002.76 midio que `AssetManager.LoadFile` (un `ioutil.ReadAll` sobre el stream del MPQ)
// muere en la Dreamcast con los archivos grandes: el .ds1 de 57.652 B se lee, el primer .dt1 de 505.000 B
// y el animdata.d2 de 411.104 B matan el SH-4 con "Data address error" y el PC adentro de `io.ReadAll`.
// Con `mpq.ReadFile` (buffer del tamano exacto) los mismos archivos se leen sin problema. Este POC es la
// prueba del ANTES y del DESPUES con el camino del MOTOR, sin esquivarlo:
//
//	LECTURA=vieja   el camino de siempre (`io.ReadAll`): el POC no llama a `ActivarLecturaDeConsola`.
//	                Se espera la excepcion en el primer .dt1 grande -- es la prueba del antes.
//	LECTURA=nueva   la lectura con el tamano reservado (`d2asset/lectura_consola.go`).
//	                Se esperan los 8 .dt1 leidos, con el crc32 igual al del host.
//
// QUE MIDE. Por cada archivo: bytes leidos, crc32 del contenido y el monton de C (arena/usado/libre) ANTES
// y DESPUES de la lectura, que es el instrumento de DC-002.28/51/75/77 (mallinfo, no `ReadMemStats`, que en
// esta consola queda en cero: DC-002.9).
//
//	MODO=archivo    cada .dt1 se lee con `am.LoadFile`: el motor devuelve un buffer NUEVO por archivo (es lo
//	                que la tarjeta pide probar). El monton sube lo que mida cada archivo, para siempre.
//	MODO=buffer     cada .dt1 se lee con `am.LoadFileEn` en UN buffer que se reusa: el monton NO sube por
//	                archivo. Es la prueba de que la lectura ya no pide memoria por archivo.
//
// LA COMPARACION CON EL HOST es el crc32: los mismos ocho archivos medidos en la PC con
// `go run ./console/dc/medidas/dt1-crc` tienen que dar exactamente los mismos numeros.
//
// QUE NO ENTRA: parsear los .dt1 (el recolector saltando en medio del parseo y el crash al pintar son
// tarjetas aparte, DC-002.78/80), el mapa, el video. Aca se mide LA LECTURA y nada mas.
package main

import (
	"hash/crc32"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
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

// init: la consola serie es la UNICA salida de texto en la Dreamcast.
func init() { d2cSerialInit() }

const (
	rutaMPQ = "/cd/JUEGO/D2DATA.MPQ"

	// zonaA: el pueblo del acto 1, la MISMA zona de DC-002.70/75/76/77 (asi los numeros se comparan).
	zonaA = `data\global\tiles\ACT1\TOWN\townN1.ds1`
)

// barraInvertida: el separador de path del MPQ, armado por su codigo (92) para no depender de como se
// escapa una barra invertida en una cadena.
const barraInvertida = string(rune(92))

// reusoDelBuffer: el buffer UNICO de MODO=buffer. Se pide cuando no alcanza (y se informa: con el tamano
// del archivo mas grande de la zona, reservado antes de la primera lectura, no tiene que crecer).
var reusoDelBuffer []byte

func main() {
	println("")
	println("LECTURA: arranca DC-002.79 -- LoadFile con archivos grandes en la consola")
	println("LECTURA: palancas: lecturaDeConsola=", lecturaDeConsola, "modoBuffer=", modoBuffer,
		"liberaCrudo=", liberaCrudo)
	println("LECTURA: monton al arrancar: " + estado())

	// el arranque de la consola (DC-002.75): caches acotados y pesando en BYTES
	d2asset.AjustarPresupuestosDeConsola()

	// DC-002.79: LA PALANCA DE ESTA TARJETA. Sin esto, `LoadFile` es el de siempre (`io.ReadAll`).
	if lecturaDeConsola {
		d2asset.ActivarLecturaDeConsola()
	}

	e := d2asset.EstadoDeLosArchivos()
	println("LECTURA: camino de lectura: consola=", e.CaminoDeConsola, "prefetchApagado=", e.PrefetchApagado)

	am, _, err := d2asset.NuevoAssetManagerDeConsola(d2util.LogLevelDefault, rutaMPQ)
	if err != nil {
		println("LECTURA: el AssetManager no se pudo armar:", err.Error())

		return
	}

	println("LECTURA: assetmanager de consola listo: " + estado())

	// ------------------------------ el .ds1: el archivo chico, la referencia ------------------------------
	// Es el que se lee bien en los dos caminos (57.652 B). Si este falla, el problema es otro.
	rutaDS1 := rutaDelDS1(zonaA)

	crudoDS1, err := am.LoadFile(rutaDS1)
	if err != nil {
		println("LECTURA: DS1 FALLO:", rutaDS1, err.Error())

		return
	}

	println("LECTURA: DS1", rutaDS1, "bytes=", len(crudoDS1), "crc32=", int(crc32.ChecksumIEEE(crudoDS1)),
		"monton=", estado())

	d, err := d2ds1.Unmarshal(crudoDS1)
	if err != nil {
		println("LECTURA: DS1 PARSE FALLO:", err.Error())

		return
	}

	crudoDS1 = nil

	println("LECTURA: DS1 w=", d.Width(), "h=", d.Height(), "archivos=", len(d.Files))

	// ------------------------------ los ocho .dt1, por la puerta del motor ------------------------------
	var (
		leidos     int
		fallidos   int
		totalBytes int
		crecidas   int
		t0         = d2cMs()
	)

	for _, f := range d.Files {
		nombre := rutaDelDT1(f)

		bytesArchivo, errArchivo := leerUno(am, nombre, &crecidas)
		if errArchivo != nil {
			fallidos++

			println("LECTURA: DT1", nombre, "FALLO:", errArchivo.Error(), "monton=", estado())

			continue
		}

		leidos++
		totalBytes += bytesArchivo
	}

	println("")
	println("LECTURA: ================ EL VEREDICTO ================")
	println("LECTURA: dt1 leidos=", leidos, "de", len(d.Files), "fallidos=", fallidos)
	println("LECTURA: bytes leidos=", totalBytes, "en", int(d2cMs()-t0), "ms")
	println("LECTURA: monton al final: " + estado())

	if modoBuffer {
		println("LECTURA: buffer reusado: capacidad=", cap(reusoDelBuffer), "crecidas=", crecidas)
	}

	estadoLectura := d2asset.EstadoDeLosArchivos()
	println("LECTURA: instrumento: lecturas=", estadoLectura.Lecturas, "bytes=", int(estadoLectura.Bytes),
		"sinTamano=", estadoLectura.SinTamano, "prefetchApagado=", estadoLectura.PrefetchApagado)

	if leidos == len(d.Files) {
		println("LECTURA: OK -- los", len(d.Files), ".dt1 de la zona se leyeron por el camino del motor")
	} else {
		println("LECTURA: MAL -- no se leyeron todos los .dt1 de la zona")
	}

	// el POC se queda quieto al final: si `main` vuelve, el runtime de la consola reinicia la maquina y la
	// corrida se repite (medido: la segunda vuelta ya no puede abrir el MPQ y ensucia el log con FALLOS que
	// no son de la medicion).
	for {
		d2cMs()
	}
}

// leerUno lee UN .dt1 por el camino que diga la palanca y lo informa: bytes, crc32 y el monton antes y
// despues de la lectura (que es lo que la tarjeta pide medir).
func leerUno(am *d2asset.AssetManager, nombre string, crecidas *int) (int, error) {
	antes := d2cMontonUsado()
	libreAntes := d2cMontonLibre()

	var (
		datos []byte
		err   error
	)

	if modoBuffer {
		// se reserva UNA vez el tamano del archivo (lo dice el MPQ) y se lee adentro del mismo buffer
		tamano, sabido := am.Loader.TamanoDeArchivo(nombre)
		if !sabido {
			println("LECTURA: DT1", nombre, "SIN-TAMANO")
		}

		if tamano > cap(reusoDelBuffer) {
			*crecidas++

			println("LECTURA: buffer crece:", cap(reusoDelBuffer), "->", tamano, "por", nombre)

			reusoDelBuffer = make([]byte, tamano)
		}

		datos, err = am.LoadFileEn(nombre, reusoDelBuffer)
	} else {
		datos, err = am.LoadFile(nombre)
	}

	usado := d2cMontonUsado()

	if err != nil {
		return 0, err
	}

	println("LECTURA: DT1", nombre, "bytes=", len(datos), "crc32=", int(crc32.ChecksumIEEE(datos)),
		"monton=", estado())
	println("LECTURA:   monton de la lectura: antes=", antes, "despues=", usado, "delta=", usado-antes,
		"libreAntes=", libreAntes)

	if liberaCrudo && len(datos) > 0 {
		// el monton externo de libgodc no lo devuelve nadie: aca se libera a mano (el mismo `free` que uso
		// `gc_external_alloc`) para ver si la corrida aguanta los ocho archivos sin crecer.
		d2cLibre(uintptr(unsafe.Pointer(&datos[0])))

		datos = nil

		println("LECTURA:   crudo liberado: monton=", d2cMontonUsado())
	}

	return len(datos), nil
}

// rutaDelDS1: el camino que pide el motor -- con barras normales y la barra inicial.
func rutaDelDS1(ruta string) string {
	return "/" + strings.ReplaceAll(ruta, barraInvertida, "/")
}

// rutaDelDT1: la MISMA traduccion que el motor (`MapEngine.AddDS1` -> lo que despues pide `LoadDT1`):
// minusculas, sin el prefijo de la instalacion, `.tg1` -> `.dt1`, y con el prefijo `/data/global/tiles/`.
func rutaDelDT1(f string) string {
	f = strings.ToLower(f)
	f = strings.ReplaceAll(f, "c:", "")
	f = strings.ReplaceAll(f, ".tg1", ".dt1")

	if i := strings.LastIndex(f, "tiles"); i >= 0 {
		f = f[i+len("tiles"):]
	}

	f = strings.TrimPrefix(f, barraInvertida)
	f = strings.TrimPrefix(f, "/")

	return "/data/global/tiles/" + f
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
