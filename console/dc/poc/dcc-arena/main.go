//go:build dreamcast

// DC-002.98 -- EL .dcc EN LA CONSOLA: decodificado sobre una arena del monton de C, con las recolecciones
// contadas.
//
// QUE PRUEBA (el criterio de la tarjeta, con numeros):
//
//  1. EL FALLO DE HOY, reproducido solo con un .dcc (sin mapa y sin personaje): el decodificador pide la
//     memoria al monton de Go y la consola muere. Se corre con `SIN_ARENA=1` y lo que hay que ver es el
//     contador de recolecciones subiendo y la corrida cortandose en el bloque A.
//  2. LA CURA: el mismo archivo, la misma direccion, con `d2dcc.PedirDeArena` puesto. Los pixeles y las
//     celdas salen del MONTON DE C (tramos pedidos a `malloc` por ser de mas de 64 KB, fuera de los dos
//     semiespacios), y el dueno los devuelve con `d2c_libre` cuando termino de usarlos. Lo que se mide es
//     `gc_stats` (semiespacio usado y colecciones) y `mallinfo` (monton de C), antes y despues.
//  3. EL JUGADOR: con el .dcc resuelto, las capas del COF de la amazona se decodifican por el mismo camino y
//     se COMPONE el cuadro con `personaje.go` (el mismo archivo del POC del personaje y del verificador del
//     host), con el crc32 de cada cuadro como prueba de contenido.
//
// EL INSTRUMENTO ES EL DE SIEMPRE EN ESTA RAMA (y por eso sirve): `d2c_gc_n` (gc_stats.collections),
// `d2c_gc_heap_usado`/`d2c_gc_heap_total` (semiespacio activo) y `mallinfo` (`d2c_monton_usado`/libre/arena).
// En la consola el semiespacio son 2.097.152 B: con 1,4 MB de celdas y 737 KB de pixeles el decodificador no
// entra, y eso es lo que este POC mide.
package main

import (
	"hash/crc32"
	"runtime"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/fuenteconsola"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2cof"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2dcc"
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

//extern d2c_gc_n
func d2cGcN() uint

//extern d2c_gc_heap_usado
func d2cGcHeapUsado() uint

//extern d2c_gc_heap_total
func d2cGcHeapTotal() uint

const (
	rutaMPQData  = "/cd/JUEGO/D2DATA.MPQ"
	rutaMPQChars = "/cd/JUEGO/D2CHAR.MPQ"

	// el mismo .dcc de DC-002.96 (144.996 B, UNA direccion de 30 cuadros, caja 135x182): es el que pide
	// 4,97 MB de monton de Go despues del arreglo de la tabla, y 36,2 MB con la tabla de antes.
	dccGrande = `data\global\objects\AA\S2\aas2litnuhth.dcc`

	// el jugador de SIEMPRE en esta rama: la amazona en neutral con arma a mano (DC-002.38/49)
	cofPersonaje = `data\global\chars\AM\COF\AMNUhth.COF`
	dirJugador   = 0

	// tramoArena: el tamano con que se le piden los tramos al monton de C. Mas de 64 KB a proposito: por
	// encima del umbral del recolector el bloque es `malloc` pelado (gc_external_alloc), o sea que NO sale
	// del semiespacio, que es justo lo que esta tarjeta necesita.
	tramoArena = 256 * 1024

	// umbralExterno: el corte de libgodc (GC_LARGE_OBJECT_THRESHOLD). Debajo de esto el bloque vive en el
	// semiespacio y liberarlo a mano corrompe el monton.
	umbralExterno = 64 * 1024

	// cuadrosDelJugador: cuantos cuadros del ciclo se componen (el ciclo NU de la amazona son 16)
	cuadrosDelJugador = 3
)

// ============================== la arena de la tarjeta ==============================

// arena son los tramos del monton de C de donde salen los buffers del decodificador. Es el dueno: pide los
// tramos, los entrega al decodificador y los devuelve cuando el que llamo ya no necesita lo decodificado.
type arena struct {
	tramos    [][]byte
	cursor    int
	pedidos   int
	entregado int
	liberados int
}

func (a *arena) pedir(bytes int) []byte {
	if len(a.tramos) == 0 || a.cursor+bytes > len(a.tramos[len(a.tramos)-1]) {
		nuevo := tramoArena
		if bytes > nuevo {
			nuevo = bytes
		}

		a.tramos = append(a.tramos, make([]byte, nuevo))
		a.cursor = 0
	}

	t := a.tramos[len(a.tramos)-1]
	b := t[a.cursor : a.cursor+bytes]
	a.cursor += bytes
	a.pedidos++
	a.entregado += bytes

	return b
}

// liberar devuelve TODOS los tramos al monton de C. Despues de esto lo decodificado no se puede tocar: los
// `PixelData` de los cuadros y las celdas apuntaban adentro.
func (a *arena) liberar() {
	for _, t := range a.tramos {
		if len(t) > umbralExterno {
			d2cLibre(uintptr(unsafe.Pointer(&t[0])))
			a.liberados++
		}
	}

	a.tramos = nil
	a.cursor = 0
}

func (a *arena) capacidad() int {
	total := 0
	for _, t := range a.tramos {
		total += len(t)
	}

	return total
}

// ============================== el instrumento ==============================

type medida struct {
	gc      uint
	goUsado uint
	goTotal uint
	monton  int
	libre   int
	arenaC  int
	ms      uint
}

func medir() medida {
	return medida{
		gc:      d2cGcN(),
		goUsado: d2cGcHeapUsado(),
		goTotal: d2cGcHeapTotal(),
		monton:  d2cMontonUsado(),
		libre:   d2cMontonLibre(),
		arenaC:  d2cMontonArena(),
		ms:      d2cMs(),
	}
}

func (m medida) String() string {
	return "gc=" + itoa(int(m.gc)) + " goUsado=" + itoa(int(m.goUsado)) + "/" + itoa(int(m.goTotal)) +
		" montonC usado=" + itoa(m.monton) + " libre=" + itoa(m.libre) + " arena=" + itoa(m.arenaC)
}

// entrega: lo medido entre dos puntos, en el orden en que se lee.
func entrega(antes, despues medida, quien string) {
	println("DCC298: " + quien + " gc=" + itoa(int(despues.gc-antes.gc)) +
		" goUsado=" + itoa(int(despues.goUsado)) + " (antes " + itoa(int(antes.goUsado)) + ")" +
		" montonC=" + itoa(despues.monton) + " (antes " + itoa(antes.monton) + ")" +
		" libreC=" + itoa(despues.libre) + " ms=" + itoa(int(despues.ms-antes.ms)))
}

func main() {
	d2cSerialInit()

	println("DCC298: arranca -- el .dcc con arena del monton de C, sin mapa y sin video")

	base := medir()
	println("DCC298: BASELINE " + base.String())

	// el archivo con dueno (la fuente de DC-002.96): se lee del MPQ y se devuelve
	fuente, err := fuenteconsola.Abrir(rutaMPQData)
	if err != nil {
		println("DCC298: MPQ-ABRIR FALLO:", err.Error())
		return
	}

	fuente.UsarLiberador(func(p uintptr) { d2cLibre(p) })

	tramo, err := fuente.Pedir(dccGrande)
	if err != nil {
		println("DCC298: DCC-LEER FALLO:", err.Error())
		return
	}

	crudo := tramo.Datos
	println("DCC298: DCC leido bytes=", len(crudo), "crc32=", int(crc32.ChecksumIEEE(crudo)))

	bloqueA(crudo)

	tramo.Liberar()

	fuente.Cerrar()

	bloqueB()
}

// bloqueA: el .dcc solo. Con arena, decodifica y devuelve; con SIN_ARENA, pide al monton de Go (y se muere).
func bloqueA(crudo []byte) {
	println("DCC298: ===== A) EL .dcc SOLO (una direccion) =====")

	a := &arena{}

	if sinArena {
		d2dcc.PedirDeArena = nil
		println("DCC298: A) SIN ARENA -- el decodificador pide al monton de Go (el defecto)")
	} else {
		d2dcc.PedirDeArena = a.pedir
		println("DCC298: A) CON ARENA -- los pixeles y las celdas salen del monton de C")
	}

	antes := medir()

	dd, err := d2dcc.LoadDirection(crudo, 0)
	if err != nil {
		println("DCC298: A) PARSE-FALLO:", err.Error())

		return
	}

	despues := medir()

	entrega(antes, despues, "A) decodificado:")

	d := dd.Directions[0]
	pixeles := len(d.Frames[0].PixelData)

	println("DCC298: A) cuadros=", len(d.Frames), "caja=", d.Box.Width, "x", d.Box.Height,
		"pixelesPorCuadro=", pixeles, "pixeles=", pixeles*len(d.Frames))

	celdas := celdasDe(d)
	println("DCC298: A) celdasDeLosCuadros=", celdas, "entradasDeLaTabla=", celdas,
		"crc32DeLosCuadros=", int(crcDeLosCuadros(d)))

	println("DCC298: A) arena entregado=", a.entregado, "en", a.pedidos, "pedidos, capacidad=", a.capacidad())

	// el contenido ya se midio: se devuelve TODO (la arena es del que la pidio)
	dd = nil
	a.liberar()
	runtime.GC()

	vuelta := medir()
	entrega(despues, vuelta, "A) despues de liberar la arena:")

	if sinArena {
		println("DCC298: A) SIN ARENA -- no hay nada que liberar (el monton de Go lo recoge el GC)")

		return
	}

	println("DCC298: A) la arena devolvio", a.liberados, "tramos; montonC libre=", vuelta.libre,
		"usado=", vuelta.monton)
}

// capaJugador: lo que hace falta para componer un cuadro, apuntando a la arena.
type capaJugador struct {
	tipo                     int
	clave                    string
	izq, arriba, ancho, alto int
	cuadros                  [][]byte
}

// bloqueB: el jugador. Las capas del COF de la amazona por el mismo camino, y los cuadros compuestos.
func bloqueB() {
	println("DCC298: ===== B) EL JUGADOR (las capas del COF, compuestas) =====")

	fuente, err := fuenteconsola.Abrir(rutaMPQChars)
	if err != nil {
		println("DCC298: B) MPQ-CHARS-ABRIR FALLO:", err.Error())
		return
	}

	fuente.UsarLiberador(func(p uintptr) { d2cLibre(p) })

	tramoCof, err := fuente.Pedir(cofPersonaje)
	if err != nil {
		println("DCC298: B) COF-LEER FALLO:", err.Error())
		return
	}

	cof, err := d2cof.Unmarshal(tramoCof.Datos)
	tramoCof.Liberar()

	if err != nil {
		println("DCC298: B) COF-PARSE FALLO:", err.Error())
		return
	}

	println("DCC298: B) COF capas=", cof.NumberOfLayers, "framesPorDir=", cof.FramesPerDirection,
		"dirs=", cof.NumberOfDirections)

	per := NuevoPersonaje(cof, dirJugador, 0)

	a := &arena{}
	d2dcc.PedirDeArena = a.pedir

	antes := medir()

	capas := make([]capaJugador, 0, len(cof.CofLayers))

	for i := range cof.CofLayers {
		nombre := per.RutaDeLaCapa(i, "dcc")

		tramo, err := fuente.Pedir(nombre)
		if err != nil {
			println("DCC298: B) CAPA", per.Claves[i], "SIN-GRAFICO:", err.Error())

			continue
		}

		dd, err := d2dcc.LoadDirection(tramo.Datos, dirJugador)
		tramo.Liberar()

		if err != nil {
			println("DCC298: B) CAPA", per.Claves[i], "PARSE-FALLO:", err.Error())

			continue
		}

		d := dd.Directions[dirJugador]

		c := capaJugador{tipo: per.Tipos[i], clave: per.Claves[i],
			izq: d.Box.Left, arriba: d.Box.Top, ancho: d.Box.Width, alto: d.Box.Height}

		for f := 0; f < len(d.Frames) && f < cuadrosDelJugador; f++ {
			c.cuadros = append(c.cuadros, d.Frames[f].PixelData)
		}

		capas = append(capas, c)

		println("DCC298: B) CAPA", c.clave, "cuadros=", len(d.Frames),
			"caja=", d.Box.Width, "x", d.Box.Height, "crc32=", int(crc32.ChecksumIEEE(c.cuadros[0])))
	}

	despues := medir()

	entrega(antes, despues, "B) capas decodificadas:")

	println("DCC298: B) capas=", len(capas), "de", cof.NumberOfLayers,
		"arena entregado=", a.entregado, "en", a.pedidos, "pedidos, capacidad=", a.capacidad())

	// LOS CUADROS COMPUESTOS: los pixeles de las capas siguen en la arena, asi que se componen ANTES de
	// liberar. Es el mismo camino del POC del personaje (`personaje.go`, comprobado por sha256 en el build).
	for f := 0; f < cuadrosDelJugador; f++ {
		p := NuevoPersonaje(cof, dirJugador, f)

		for i := range capas {
			if f >= len(capas[i].cuadros) {
				continue
			}

			p.Agregar(capas[i].tipo, capas[i].clave, capas[i].izq, capas[i].arriba,
				capas[i].ancho, capas[i].alto, capas[i].cuadros[f])
		}

		p.Componer()

		println("DCC298: B) CUADRO", f, "caja=", p.Ancho, "x", p.Alto, "esquina=", p.Izq, p.Arriba,
			"pixelesNoVacios=", p.PixelesNoVacios, "crc32=", int(p.CRC32()))
	}

	vivo := medir()
	println("DCC298: B) con los cuadros compuestos, sin liberar: " + vivo.String())

	a.liberar()
	runtime.GC()

	vuelta := medir()
	entrega(vivo, vuelta, "B) despues de liberar la arena:")
	println("DCC298: B) montonC usado=", vuelta.monton, "libre=", vuelta.libre, "arena=", vuelta.arenaC)
}

func celdasDe(d *d2dcc.DCCDirection) int {
	celdas := 0

	for _, fr := range d.Frames {
		celdas += fr.HorizontalCellCount * fr.VerticalCellCount
	}

	return celdas
}

func crcDeLosCuadros(d *d2dcc.DCCDirection) uint32 {
	var todos []byte

	for _, fr := range d.Frames {
		if fr == nil {
			continue
		}

		todos = append(todos, fr.PixelData...)
	}

	return crc32.ChecksumIEEE(todos)
}

// itoa: el POC no trae strconv (es de las cosas que no entran en el semiespacio con los `.gox` de la
// consola), asi que van las cuatro lineas de siempre.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negativo := n < 0
	if negativo {
		n = -n
	}

	var d [12]byte

	i := len(d)

	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}

	if negativo {
		i--
		d[i] = '-'
	}

	return string(d[i:])
}
