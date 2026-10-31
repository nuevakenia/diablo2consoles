//go:build dreamcast

// DC-002.96 -- LA SOURCE DE CONSOLA EN LA DREAMCAST: los seis formatos con dueno, y el monton que vuelve.
//
// QUE PRUEBA (el criterio de la tarjeta, con numeros):
//
//  1. TODAS las cargas grandes del camino de consola entran por la Source nueva (`console/dc/fuenteconsola`),
//     con su `Tamano` y su dueno explicito: el .dt1, el .ds1, el .dcc, el .cof, el .dc6 y el animdata.
//  2. El MONTON VUELVE AL NIVEL DE PARTIDA despues de cada archivo. Se mide con `mallinfo` (`uordblks`), el
//     mismo instrumento de DC-002.28/51/75/77: en libgodc todo pedido de mas de 64 KB se va a
//     `gc_external_alloc` (malloc pelado) y el recolector NO lo devuelve, asi que el numero dice exactamente
//     si el buffer se devolvio o se quedo tomado (medido en DC-002.77/78: leer un .dt1 de 505 KB por el
//     camino del motor dejaba ~1,8 MB tomados para siempre).
//  3. La ZONA compone igual que en las tarjetas anteriores: el pueblo del acto 1 con crc32 3507647456
//     (DC-002.38/.81/.88), ahora con sus nueve cargas (1 .ds1 + 8 .dt1) por la fuente nueva.
//  4. El dueño devuelve todo: `Pedidos == Liberados` y `BytesVivos == 0` al cerrar.
//
// LOS CUATRO BLOQUES, en este orden y por un motivo de RAM (no de gusto):
//
//	A) LA ZONA      1 .ds1 + 8 .dt1 -> el compositor del motor (`nivel.go`) -> crc32 del lienzo. Va PRIMERO
//	                porque es la unica carga que necesita los buffers del compositor (lienzo de 262 KB +
//	                scratch de 80 KB) y conviene medirla con el monton ordenado.
//	B) LOS SEIS     un archivo de cada formato, cada uno: `Pedir` -> crc32 (+ parseo con el DECODIFICADOR
//	                DEL MOTOR) -> `Liberar`, con el monton antes y despues.
//	C) 1 MB         el .dt1 mas grande del pueblo (1.008.003 B): la prueba en contra del monton, sola.
//	D) SIN DUENO    los mismos seis, leidos como los lee `d2asset.LoadFile` hoy: un buffer NUEVO por archivo
//	                (`make`) que no libera nadie. Es la prueba EN CONTRA: si el instrumento sirve, aca el
//	                monton NO vuelve y queda tomado lo que mide la suma de los archivos grandes. Va al final
//	                a proposito: es el unico bloque que deja memoria tomada.
//
// MUTANTE (`NO_LIBERA=1`, lo pone el script recompilando): el dueño NO libera. Con el mutante el bloque A y
// el C tienen que dejar el monton arriba, igual que el D: si no, el instrumento no mide lo que dice medir.
//
// QUE NO ENTRA: el video (este POC no enciende el PVR: mide memoria, no dibujo), el motor (no se toca ni
// `d2asset` ni `d2loader`), ni el camino de la PC.
package main

import (
	"hash/crc32"
	"runtime"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dc6"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2ds1"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dt1"
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

	// el pueblo del acto 1: la MISMA zona de DC-002.38/70/75/77/81, para que el crc32 del lienzo se compare
	// contra el numero ya establecido (3507647456).
	ds1Pueblo          = `data\global\tiles\ACT1\TOWN\townN1.ds1`
	centroPuebloX      = 28
	centroPuebloY      = 20
	ladoDeLaVentana    = 512
	crcDelPuebloEspera = 3507647456 // DC-002.38, repetido en DC-002.81/.88

	// los otros cinco formatos. El .dt1 de ~1 MB es el mas grande del pueblo; el .dcc y el .dc6 pasan de
	// 64 KB (monton externo, hay que devolverlos); el .cof de un objeto mide 253 B (semiespacio: NO se libera
	// a mano, y el instrumento tiene que decir que no lo hizo).
	dt1Grande  = `data\global\tiles\ACT1\TOWN\floor.dt1`
	dccGrande  = `data\global\objects\AA\S2\aas2litnuhth.dcc`
	cofObjeto  = `data\global\objects\E2\COF\e2S1HTH.cof`
	dc6Grande  = `data\global\ui\FrontEnd\ts11b.DC6`
	animdataD2 = `data\global\AnimData.D2`

	// el margen del monton: 4 KB. El pico de una lectura puede dejar el asignador con algunos bytes de
	// encabezado, pero un buffer que NO se devolvio se ve seguro (los archivos grandes pesan 8.192 B o mas).
	margenDelMonton = 4 * 1024
)

// forma: un formato de la lista de los seis, con el banco de pruebas del decodificador del motor (nil = solo
// se lee y se hashea; el animdata NO se parsea: DC-002.50 midio que sus 411 KB + 2.556 registros no entran
// en el monton de la consola -- lo que se prueba aca es la CARGA, no el parser).
type forma struct {
	formato string
	nombre  string
	probar  func(datos []byte) (string, error)
}

// losSeis: las seis cargas del camino de consola, con su decodificador.
//
// OJO CON EL PARSEO EN LA CONSOLA: `probar` nil significa que el archivo se LEE y se hashea, pero no se
// parsea, y no es un olvido. Medido en esta tarjeta (corrida 1, /tmp/dcframes296):
//
//   - el .dcc (aas2litnuhth, 144.996 B) con `d2dcc.LoadDirection`: el decodificador de UNA direccion lleva el
//     monton de Go al limite -- 45 recolecciones seguidas con lo vivo subiendo 48 KB en cada una (1,78 MB ->
//     2,07 MB, el tamano del semiespacio) y la consola muere con `FATAL: recover without checkpoint`. No es la
//     fuente: es que el monton de la consola no aguanta ese decodificador, que es el defecto de DC-002.50.
//   - el animdata (411.104 B) ya venia sin parseo por lo mismo (DC-002.50: 411 KB + 2.556 registros no entran).
//
// En los dos casos lo que la tarjeta tiene que probar es la CARGA: el tamano y el crc32 dicen si lo leido es
// el archivo. El parseo del .dt1, el .ds1, el .cof y el .dc6 si entra, y se hace.
var losSeis = []forma{
	{".dt1", dt1Grande, func(datos []byte) (string, error) {
		dt1, err := d2dt1.LoadDT1(datos)
		if err != nil {
			return "", err
		}

		return "tiles=" + itoa(len(dt1.Tiles)), nil
	}},
	{".ds1", ds1Pueblo, func(datos []byte) (string, error) {
		d, err := d2ds1.Unmarshal(datos)
		if err != nil {
			return "", err
		}

		return "w=" + itoa(d.Width()) + " h=" + itoa(d.Height()) + " objetos=" + itoa(len(d.Objects)) +
			" archivos=" + itoa(len(d.Files)), nil
	}},
	// el .dcc: SOLO la carga (ver arriba). El decodificador no entra en el monton de la consola.
	{".dcc", dccGrande, nil},
	{".cof", cofObjeto, func(datos []byte) (string, error) {
		c, err := d2cof.Unmarshal(datos)
		if err != nil {
			return "", err
		}

		return "capas=" + itoa(c.NumberOfLayers) + " framesPorDir=" + itoa(c.FramesPerDirection), nil
	}},
	{".dc6", dc6Grande, func(datos []byte) (string, error) {
		dc, err := d2dc6.Load(datos)
		if err != nil {
			return "", err
		}

		fr := dc.Frames[0]
		pix := dc.DecodeFrame(0)

		return "cuadros=" + itoa(len(dc.Frames)) + " caja=" + itoa(int(fr.Width)) + "x" +
			itoa(int(fr.Height)) + " pixeles=" + itoa(noCero(pix)), nil
	}},
	// el animdata: SOLO la carga (DC-002.50).
	{"animdata", animdataD2, nil},
}

var (
	// fuente: la Source de consola de la corrida.
	fuente *fuenteconsola.Fuente

	// los contadores del informe: cuantas cargas pasaron por la fuente nueva y cuantas por el camino viejo.
	cargasPorLaFuente      int
	cargasPorElCaminoViejo int

	// baseDelMonton: el `uordblks` al arrancar, DESPUES del recolector. "Vuelve a la base" se mide contra
	// este numero y contra el de antes de cada archivo (el que importa: la base de la fase).
	baseDelMonton int
)

func main() {
	d2cSerialInit()

	println("")
	println("FUENTE296: arranca DC-002.96 -- la Source de consola, los seis formatos con dueno")
	println("FUENTE296: palancas: noLibera=", noLibera)
	println("FUENTE296: monton al arrancar: " + estado())

	runtime.GC()

	baseDelMonton = d2cMontonUsado()
	println("FUENTE296: base del monton (tras GC) usado=", baseDelMonton, "libre=", d2cMontonLibre())

	f, err := fuenteconsola.Abrir(rutaMPQ)
	if err != nil {
		println("FUENTE296: MPQ-ABRIR FALLO:", rutaMPQ, err.Error())

		return
	}

	fuente = f

	// EL DUENO: con que se devuelve un tramo grande al monton. Sin esto (o con el mutante) el tramo se queda
	// tomado, que es exactamente el defecto que la tarjeta viene a cerrar.
	if !noLibera {
		fuente.UsarLiberador(fuenteconsola.Liberador(d2cLibre))
	}

	println("FUENTE296: MPQ abierto con la Source de consola; liberador conectado=", !noLibera,
		"monton="+estado())

	// EL PRESTAMO SE CALIENTA ANTES DE MEDIR. La primera lectura pide la memoria de trabajo (tabla de
	// posiciones, bloque comprimido, descompreso y descompresor de zlib: 14 KB medidos en el host). Si eso
	// cayera dentro de un bloque medido, el delta del monton traeria el prestamo y no el archivo. Se calienta
	// con el archivo mas grande, que es el que fija el tamano de todos.
	if tramo, err := fuente.Pedir(dt1Grande); err == nil {
		tramo.Liberar()
	}

	e0 := fuente.Estado()
	println("FUENTE296: prestamo calentado: reservado=", e0.PrestamoReservado, "crecidas=", e0.PrestamoCrecidas,
		"monton="+estado())

	// ============================== A) LA ZONA ==============================
	crcZona, okZona := cargarLaZona()

	// ============================== B) LOS SEIS FORMATOS ==============================
	aciertos := 0
	fallos := 0

	for _, caso := range losSeis {
		if probarFormato(caso) {
			aciertos++
		} else {
			fallos++
		}
	}

	// ============================== C) LA PRUEBA EN CONTRA DEL 1 MB ==============================
	okGrande := probarElGrande()

	// ============================== D) EL MISMO ARCHIVO SIN DUENO (el camino de hoy) ==============================
	sinDueno := probarSinDueno()

	// ============================== EL VEREDICTO ==============================
	runtime.GC()

	despuesDeTodo := d2cMontonUsado()
	e := fuente.Estado()

	println("")
	println("FUENTE296: ================ EL VEREDICTO ================")
	println("FUENTE296: cargas por la Source nueva=", cargasPorLaFuente,
		"por el camino viejo=", cargasPorElCaminoViejo)
	println("FUENTE296: seis formatos: aciertos=", aciertos, "de", len(losSeis), "fallos=", fallos)
	println("FUENTE296: instrumento de la fuente: pedidos=", e.Pedidos, "liberados=", e.Liberados,
		"bytesVivos=", int(e.BytesVivos), "externos=", e.LibresExternos, "dobles=", e.LiberadosDobles,
		"enBuffer=", e.LecturasEnBuffer, "conPrestamo=", e.LecturasConPrestamo,
		"prestamo=", e.PrestamoReservado, "crecidas=", e.PrestamoCrecidas)

	if e.Pedidos != e.Liberados || e.BytesVivos != 0 {
		println("FUENTE296: DUENO MAL -- se pidieron", e.Pedidos, "tramos y se devolvieron", e.Liberados,
			"; quedan", int(e.BytesVivos), "bytes vivos")
	} else {
		println("FUENTE296: DUENO OK -- todos los tramos que se pidieron se devolvieron (0 bytes vivos)")
	}

	if okZona {
		println("FUENTE296: ZONA OK -- el pueblo compone igual: crc32", int(crcZona), "==", crcDelPuebloEspera)
	} else {
		println("FUENTE296: ZONA MAL -- el pueblo NO compone igual (crc32", int(crcZona), "contra", crcDelPuebloEspera, ")")
	}

	println("FUENTE296: monton al final: usado=", despuesDeTodo, "libre=", d2cMontonLibre(),
		" (el bloque D deja tomado lo que no se libera:", sinDueno, "bytes)")

	if okGrande {
		println("FUENTE296: 1MB OK en el veredicto -- el monton volvio a la base con el archivo de 1 MB")
	} else {
		println("FUENTE296: 1MB MAL en el veredicto -- el archivo de 1 MB dejo bytes tomados")
	}

	if noLibera {
		println("FUENTE296: MUTANTE noLibera=true -- la corrida de ARRIBA no vale como prueba del dueno")
	}

	// el POC se queda quieto: si `main` vuelve, el runtime de la consola reinicia la maquina y la corrida se
	// repite (medido en DC-002.79: la segunda vuelta ya no puede abrir el MPQ y ensucia el log)
	for {
		d2cMs()
	}
}

// cargarLaZona: el pueblo del acto 1 cargado ENTERO por la fuente nueva -- el .ds1 y los ocho .dt1 que el
// mismo lista -- compuesto por el compositor del motor (`nivel.go`, el mismo archivo que usan el POC y el
// verificador del host). Devuelve el crc32 del lienzo.
//
// EL DUENO SE VE ACA: los bloques del .dt1 apuntan ADENTRO del tramo, asi que se libera recien despues de
// `Alimentar` (que los compone al lienzo). Ese es el contrato del tramo, escrito en la cabecera de la fuente.
func cargarLaZona() (uint32, bool) {
	println("")
	println("FUENTE296: ===== A) LA ZONA (el pueblo) =====")

	t0 := d2cMs()

	tramoDS1, err := fuente.Pedir(ds1Pueblo)
	if err != nil {
		println("FUENTE296: DS1 PEDIR FALLO:", err.Error())

		return 0, false
	}

	cargasPorLaFuente++

	bytesDS1 := len(tramoDS1.Datos)
	crcDS1 := crc32.ChecksumIEEE(tramoDS1.Datos)

	d, err := d2ds1.Unmarshal(tramoDS1.Datos)
	if err != nil {
		println("FUENTE296: DS1 PARSE FALLO:", err.Error())

		tramoDS1.Liberar()

		return 0, false
	}

	// el tramo ya no es del que llama despues de esto: los numeros se guardaron ANTES
	tramoDS1.Liberar()

	println("FUENTE296: DS1 bytes=", bytesDS1, "crc32=", int(crcDS1))

	n := NuevoNivelEn(d, ladoDeLaVentana, ladoDeLaVentana, centroPuebloX, centroPuebloY, &reusoDelNivel)

	println("FUENTE296: DS1 w=", d.Width(), "h=", d.Height(), "archivos=", len(d.Files),
		"ventana=", n.Ancho, "origen=", n.OrigenX, n.OrigenY)
	println("FUENTE296: PEDIDOS pisos=", n.PisosPedidos, "paredes=", n.ParedesPedidas, "total=", len(n.Pedidos))

	leidos := 0

	for _, nombreDelDS1 := range d.Files {
		nombre := RutaDelDT1(nombreDelDS1)

		antes := d2cMontonUsado()

		tramo, err := fuente.Pedir(nombre)
		if err != nil {
			println("FUENTE296: DT1 PEDIR FALLO:", nombre, err.Error())

			continue
		}

		cargasPorLaFuente++

		bytesDT1 := len(tramo.Datos)
		crcDT1 := crc32.ChecksumIEEE(tramo.Datos)

		dt1, err := d2dt1.LoadDT1(tramo.Datos)
		if err != nil {
			println("FUENTE296: DT1 PARSE FALLO:", nombre, err.Error())

			tramo.Liberar()

			continue
		}

		n.Alimentar(dt1.Tiles)

		// el tramo se devuelve DESPUES de componer: los bloques del .dt1 apuntaban adentro
		dt1 = nil
		tramo.Liberar()

		leidos++

		despues := d2cMontonUsado()

		println("FUENTE296: DT1", nombre, "bytes=", bytesDT1, "crc32=", int(crcDT1),
			"monton antes=", antes, "despues=", despues, "delta=", despues-antes)
	}

	n.Cerrar()

	crc := n.CRC32()

	println("FUENTE296: COMPUESTO pisos=", n.PisosCompuestos, "/", n.PisosPedidos,
		"paredes=", n.ParedesCompuestas, "/", n.ParedesPedidas,
		"sintilePiso=", n.PisosSinTile, "sintilePared=", n.ParedesSinTile, "fuera=", n.FueraDeVentana)
	println("FUENTE296: LIENZO pixelesNoVacios=", n.PixelesNoVacios(), "de", n.Ancho*n.Alto,
		"crc32=", int(crc), "dt1 leidos=", leidos, "de", len(d.Files),
		"ms=", int(d2cMs()-t0))

	// los buffers del compositor se reusan entre zonas (DC-002.78): aca se guardan para que no se pidan dos
	// veces si esta corrida compusiera otra
	reusoDelNivel.Canvas = n.Canvas
	reusoDelNivel.Scratch = n.Scratch
	reusoDelNivel.Pedidos = n.Pedidos

	runtime.GC()

	println("FUENTE296: monton tras la zona: usado=", d2cMontonUsado(), "libre=", d2cMontonLibre())

	return crc, crc == crcDelPuebloEspera && leidos == len(d.Files)
}

// probarFormato: UN archivo de un formato, por la fuente nueva y con el dueno explicito: `Pedir` -> crc32 ->
// el decodificador del motor -> `Liberar`, con el monton medido antes y despues. Devuelve false si el monton
// no volvio a la base o si algo fallo.
func probarFormato(caso forma) bool {
	antes := d2cMontonUsado()

	tramo, err := fuente.Pedir(caso.nombre)
	if err != nil {
		println("FUENTE296: " + caso.formato + " PEDIR FALLO: " + caso.nombre + ": " + err.Error())

		return false
	}

	cargasPorLaFuente++

	bytesArchivo := len(tramo.Datos)
	crcArchivo := crc32.ChecksumIEEE(tramo.Datos)

	// PRIMERO lo que prueba la CARGA (tamano y crc32): si despues el decodificador no entra en el monton, en
	// el log queda igual el numero que dice si lo leido es el archivo.
	println("FUENTE296: " + caso.formato + " LIDO bytes=" + itoa(bytesArchivo) + " crc32=" +
		itoa(int(crcArchivo)) + " monton=" + itoa(d2cMontonUsado()))

	detalle := ""
	parseo := "solo lectura"

	if caso.probar != nil {
		texto, err := caso.probar(tramo.Datos)
		if err != nil {
			println("FUENTE296: " + caso.formato + " PARSE FALLO: " + caso.nombre + ": " + err.Error())

			tramo.Liberar()

			return false
		}

		detalle = texto
		parseo = "parseado"
	}

	tramo.Liberar()

	despues := d2cMontonUsado()

	volvio := despues-antes <= margenDelMonton

	println("FUENTE296: " + caso.formato + " " + caso.nombre + " bytes=" + itoa(bytesArchivo) +
		" crc32=" + itoa(int(crcArchivo)) + " " + parseo + " " + detalle +
		" monton antes=" + itoa(antes) + " despues=" + itoa(despues) + " delta=" + itoa(despues-antes) +
		" volvio=" + siNo(volvio))

	return volvio
}

// probarElGrande: LA PRUEBA EN CONTRA del monton, sola y con nombre propio: un asset de 1 MB (1.008.003 B,
// el .dt1 mas grande del pueblo) leido por la fuente nueva, con el monton medido antes y despues. Tiene que
// volver al nivel de partida; con el mutante (`NO_LIBERA=1`) NO vuelve y el numero lo dice.
func probarElGrande() bool {
	println("")
	println("FUENTE296: ===== C) LA PRUEBA DEL 1 MB =====")

	runtime.GC()

	antes := d2cMontonUsado()

	tramo, err := fuente.Pedir(dt1Grande)
	if err != nil {
		println("FUENTE296: 1MB PEDIR FALLO:", err.Error())

		return false
	}

	cargasPorLaFuente++

	bytesArchivo := len(tramo.Datos)
	crcArchivo := crc32.ChecksumIEEE(tramo.Datos)

	pico := d2cMontonUsado()

	tramo.Liberar()

	runtime.GC()

	despues := d2cMontonUsado()

	delta := despues - antes
	volvio := delta <= margenDelMonton

	println("FUENTE296: 1MB bytes=", bytesArchivo, "crc32=", int(crcArchivo))
	println("FUENTE296: 1MB monton antes=", antes, "con el tramo vivo=", pico, "despues de liberar=", despues,
		"delta=", delta, "margen=", margenDelMonton, "volvioALaBase=", volvio)

	if volvio {
		println("FUENTE296: 1MB OK -- el monton volvio al nivel de partida despues del archivo de 1 MB")
	} else {
		println("FUENTE296: 1MB MAL -- el tramo NO se devolvio: quedan", delta, "bytes tomados")
	}

	return volvio
}

// probarSinDueno: los mismos seis archivos, leidos COMO LOS LEE HOY el camino del motor (`d2asset.LoadFile`:
// un buffer NUEVO por archivo, con `LeerEnteroEn`, que nadie libera). Es la prueba en contra del instrumento:
// si el monton no sube aca, entonces el bloque B no estaba midiendo lo que dice medir.
//
// Devuelve los bytes que quedaron tomados (la suma de los seis, para los que pasan el umbral externo).
func probarSinDueno() int {
	println("")
	println("FUENTE296: ===== D) LOS MISMOS SEIS SIN DUENO (el camino de hoy) =====")

	runtime.GC()

	antes := d2cMontonUsado()

	var buffers [][]byte

	for _, caso := range losSeis {
		tamano, hay := fuente.Tamano(caso.nombre)
		if !hay {
			continue
		}

		// esto es `d2asset.loadFileDeConsola`: `make` del tamano y `Loader.LeerEnteroEn`. El buffer queda
		// sin dueno (en la consola, del monton externo que el recolector no devuelve).
		dst := make([]byte, tamano)

		leidos, err := fuente.LeerEnteroEn(caso.nombre, dst)
		if err != nil {
			println("FUENTE296: SINDUENO " + caso.formato + " FALLO: " + err.Error())

			continue
		}

		cargasPorElCaminoViejo++

		buffers = append(buffers, dst[:leidos])

		println("FUENTE296: SINDUENO " + caso.formato + " " + caso.nombre + " bytes=" + itoa(leidos) +
			" crc32=" + itoa(int(crc32.ChecksumIEEE(dst[:leidos]))) + " monton=" + itoa(d2cMontonUsado()))
	}

	runtime.GC()

	despues := d2cMontonUsado()

	tomado := despues - antes

	println("FUENTE296: SINDUENO monton antes=", antes, "despues=", despues, "tomado=", tomado,
		"(es lo que el camino de hoy deja por archivo)")

	// los buffers NO se liberan a proposito: es la prueba. Se sueltan de la lista para que el recolector no
	// los tenga como vivos por el slice, pero el bloque del monton externo sigue tomado (eso es lo medido).
	buffers = nil

	return tomado
}

// noCero: cuantos pixeles del cuadro no son el indice 0 (el instrumento del decodificador).
func noCero(pix []byte) int {
	c := 0

	for _, v := range pix {
		if v != 0 {
			c++
		}
	}

	return c
}

// reusoDelNivel: los buffers del compositor, que se reusan (ver nivel.go, BuffersDelNivel).
var reusoDelNivel BuffersDelNivel

func siNo(v bool) string {
	if v {
		return "si"
	}

	return "no"
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
