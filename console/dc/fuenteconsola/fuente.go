package fuenteconsola

// DC-002.96 -- LA SOURCE DE CONSOLA: las TRES interfaces de `asset` en UNA implementacion, con el dueno
// del buffer EXPLICITO (`Pedir` / `Liberar`).
//
// POR QUE EXISTE. La decision de Daniel (31/10) es que los assets grandes viven FUERA del monton de Go, con
// liberacion explicita, y que el motor NO se toca: todo lo de la consola vive detras de la interfaz
// `asset.Source`. Habia tres verdes parciales -- `MPQ.ReadFile` de una vez (DC-002.70), la lectura con
// memoria prestada (DC-002.80, 14 KB de trabajo para un archivo de 1 MB) y la zona sin pedir memoria por
// archivo (DC-002.81) -- pero el camino de consola NO cubria todas las cargas: `d2asset.LoadFile` (el que
// usan el .ds1, el .cof, el .dcc, el .dc6 y el animdata) pide con `make([]byte, tamano)` un buffer NUEVO
// por archivo, y en libgodc todo pedido de mas de 64 KB se va a `gc_external_alloc` (malloc pelado, fuera
// de los dos semiespacios) que el recolector NO devuelve jamas. Leer una zona entera por ahi deja tomado,
// por archivo, lo que mide el archivo (medido en DC-002.77/78). Solo el .dt1 se salvaba, por el arena de
// DC-002.78/.88.
//
// LA CURA, y es una sola pieza: una Source que (1) implementa las tres interfaces en un solo sitio, asi
// que el motor ve EXACTAMENTE la misma superficie que con `d2loader/mpq.Source`, y (2) sabe PRESTAR el
// buffer de un archivo entero (`Pedir`), de modo que el que pide el tramo es el que lo libera
// (`Tramo.Liberar`) y el motor nunca recibe un buffer que tenga que liberar el.
//
// EL CONTRATO DEL DUENO (esto es lo que hay que respetar al usarla):
//   - `Pedir(nombre)` devuelve un `*Tramo` cuyo `Datos` es del QUE LLAMA. Nadie mas lo libera.
//   - `Tramo.Liberar()` lo devuelve al monton: si el tramo paso del umbral externo (64 KB) se le hace
//     `free()` al bloque que `gc_external_alloc` pidio; si no paso el umbral vive en el semiespacio del
//     recolector y lo recoge el GC (liberarlo a mano ahi seria corromper el monton). Es idempotente: dos
//     `Liberar` no liberan dos veces (se cuentan las dobles y se ignoran).
//   - `Datos` queda en `nil` al liberar, asi un uso despues de liberar se ve como una rebanada vacia y no
//     como memoria de otro.
//   - Los bytes del tramo valen hasta `Liberar`: los bloques que el parseo deje apuntando adentro (los
//     `EncodedData` de un .dt1, los pixeles de un .dcc) se tienen que COPIAR o consumir ANTES.
//   - `LeerEnteroEn` y `LeerEnteroEnConPrestamo` NO piden memoria: escriben en el `dst` del que llama. Eso
//     es lo que garantiza que el motor nunca reciba un buffer que tenga que liberar el.
//
// POR QUE UN PAQUETE APARTE Y NO UN METODO EN `d2loader/mpq`. El motor no se toca: `d2loader/mpq.Source` y
// `d2core/d2asset` quedan como estan y la PC no cambia de comportamiento. Esta Source es una
// implementacion NUEVA y opcional, que se puede poner en la lista de fuentes del loader sin cambiar una
// linea del motor (`RegistrarEn`, que es lo que hace que las cargas del camino de consola pasen por aca).
//
// POR QUE EL `Liberador` SE INYECTA. `free()` no esta en la libreria estandar de Go: en la consola es el
// `d2c_libre` del shim en C (el mismo `free` que usa `runtime.FreeExternal`, ver `gc_heap.c`). El paquete
// no puede llamarlo solo, asi que quien arranca la consola dice con que se libera (`UsarLiberador`). En la
// PC no se inyecta nada: `Liberar` solo suelta la referencia y el recolector hace su trabajo (que es
// exactamente lo que corresponde cuando no hay monton externo que devolver).

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2mpq"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2loader/asset"
)

// Las tres interfaces, en una sola implementacion. Si alguna se rompe, esto no compila: es el criterio 1 de
// la tarjeta ("las tres interfaces en una sola implementacion de consola") escrito como asercion.
var (
	_ asset.Source                     = (*Fuente)(nil)
	_ asset.SourceDeLecturaEntera      = (*Fuente)(nil)
	_ asset.SourceDeLecturaConPrestamo = (*Fuente)(nil)
)

// umbralExterno: el mismo corte que usa libgodc para mandar una asignacion al monton de C
// (`GC_LARGE_OBJECT_THRESHOLD` = 64 KB, `gc_heap.c:116`). Debajo de esto el bloque vive en el semiespacio
// del recolector y liberarlo a mano corrompe el monton; arriba, es `malloc` y hay que devolverlo.
const umbralExterno = 64 * 1024

// Liberador devuelve al sistema un bloque que el runtime mando al monton externo. En la consola es
// `d2c_libre` (free); en la PC no se conecta (nil) porque ahi no hay monton externo que devolver.
type Liberador func(direccion uintptr)

var (
	// errSinTamano: la fuente no sabe cuanto mide el archivo (no esta en la tabla de bloques del MPQ). Se
	// dice en vez de adivinar: pedir el buffer del tamano equivocado es lo que este camino evita.
	errSinTamano = errors.New("fuenteconsola: la fuente no sabe el tamano del archivo")

	// errSinMPQ: se uso la fuente sin MPQ (cerrada o mal armada).
	errSinMPQ = errors.New("fuenteconsola: la fuente no tiene MPQ abierto")
)

// Fuente es la Source de consola: un MPQ envuelto en las tres interfaces mas el prestamo explicito de
// buffers. No es segura para varias goroutines (el MPQ de la consola tampoco lo es en este uso: una sola
// hebra carga los assets).
type Fuente struct {
	mpq  *d2mpq.MPQ
	ruta string

	// liberador: con que se devuelve un tramo grande al monton. nil = no se libera a mano (la PC).
	liberador Liberador

	// prestamo: la memoria de trabajo de la lectura (tabla de posiciones, bloque comprimido, descompreso y
	// el descompresor de zlib), pedida UNA vez y reusada archivo tras archivo (DC-002.80). Es la misma para
	// los tres caminos de la fuente.
	prestamo *d2mpq.Scratch

	// el instrumento: que camino atendio cada carga y cuantos buffers se prestaron y se devolvieron.
	abiertos            int // Open: el camino del STREAM (no pide el archivo entero, y no lo libera nadie)
	tamanos             int // cuantas veces se pregunto el tamano
	lecturasEnBuffer    int // LeerEnteroEn (dst del que llama)
	lecturasConPrestamo int // LeerEnteroEnConPrestamo
	sinTamano           int // archivos de los que no se supo el tamano

	pedidos         int // Pedir: cuantos tramos se prestaron
	liberados       int // Liberar: cuantos se devolvieron
	libresExternos  int // de esos, cuantos fueron un free() DE VERDAD (pasaron el umbral)
	liberadosDobles int // Liberar dos veces (no libera dos veces: se cuenta)
	bytesPedidos    int // bytes que se prestaron en total
	bytesLeidos     int // bytes que se leyeron en total
	bytesVivos      int // bytes prestados que todavia no se devolvieron (tiene que volver a 0)
	pedidosFallidos int // Pedir que no pudo leer: el buffer NO se entrega
}

// Abrir abre el MPQ con la Source de consola. La ruta es la del disco (en la Dreamcast, `/cd/JUEGO/...`).
func Abrir(ruta string) (*Fuente, error) {
	m, err := d2mpq.FromFile(ruta)
	if err != nil {
		return nil, err
	}

	return &Fuente{mpq: m, ruta: ruta}, nil
}

// UsarLiberador dice con que se devuelve al monton un tramo grande. En la consola se pasa el `free` del
// shim (ver la cabecera); sin el, `Liberar` solo suelta la referencia y el tramo queda tomado hasta que
// termine el programa -- que es justo el defecto que esta tarjeta viene a cerrar.
func (f *Fuente) UsarLiberador(l Liberador) { f.liberador = l }

// Cerrar cierra el MPQ. Los tramos ya prestados NO se liberan aca: el dueno de cada uno es el que llamo a
// `Pedir` (por eso el dueno es explicito).
func (f *Fuente) Cerrar() error {
	if f.mpq == nil {
		return nil
	}

	return f.mpq.Close()
}

// Path devuelve la ruta del MPQ en el disco. asset.Source.
func (f *Fuente) Path() string { return f.ruta }

// String devuelve la ruta del MPQ en el disco. asset.Source (fmt.Stringer).
func (f *Fuente) String() string { return f.ruta }

// Exists dice si el archivo esta en el MPQ. asset.Source.
func (f *Fuente) Exists(subPath string) bool {
	if f.mpq == nil {
		return false
	}

	return f.mpq.Contains(limpiar(subPath))
}

// Open devuelve un stream del archivo. asset.Source.
//
// OJO: es el camino que NO sabe el tamano y NO libera nada (el stream prefetchea el archivo comprimido
// entero, hasta 2 MB). En la consola esta para que la interfaz este completa -- el loader del motor lo usa
// para los archivos que no se quieren enteros -- pero las cargas GRANDES van por `Pedir`, `LeerEnteroEn` o
// `LeerEnteroEnConPrestamo`. Se cuenta (`Abiertos`) para poder decir cuantas cargas quedaron en este camino.
func (f *Fuente) Open(name string) (io.ReadSeeker, error) {
	if f.mpq == nil {
		return nil, errSinMPQ
	}

	f.abiertos++

	return f.mpq.ReadFileStream(limpiar(name))
}

// Tamano devuelve el tamano descomprimido del archivo (los bytes que hay que reservar). Sale de la tabla de
// bloques del MPQ: no lee ni un byte del archivo. asset.SourceDeLecturaEntera.
func (f *Fuente) Tamano(nombre string) (int, bool) {
	if f.mpq == nil {
		return 0, false
	}

	f.tamanos++

	return f.mpq.TamanoDeArchivo(limpiar(nombre))
}

// LeerEnteroEn lee el archivo entero DENTRO del buffer que presta el que llama. No pide memoria: el destino
// es del que llama y la liberacion tambien (esta funcion no devuelve nada que liberar). Es el camino del
// motor (`d2loader.LeerEnteroEn` -> `d2asset.LoadFile`). asset.SourceDeLecturaEntera.
func (f *Fuente) LeerEnteroEn(nombre string, dst []byte) (int, error) {
	if f.mpq == nil {
		return 0, errSinMPQ
	}

	leidos, err := f.mpq.LeerEn(limpiar(nombre), dst)
	if err != nil {
		return leidos, err
	}

	f.lecturasEnBuffer++
	f.bytesLeidos += leidos

	return leidos, nil
}

// LeerEnteroEnConPrestamo lee el archivo entero con la memoria de trabajo prestada: ni el destino ni la
// lectura piden nada. `trabajo` puede ser nil (se usa, y se arma, el de la fuente). Es el camino de los
// .dt1 de una zona (`d2loader.LeerEnteroEnConPrestamo`). asset.SourceDeLecturaConPrestamo.
func (f *Fuente) LeerEnteroEnConPrestamo(nombre string, dst []byte, trabajo *d2mpq.Scratch) (int, error) {
	if f.mpq == nil {
		return 0, errSinMPQ
	}

	if trabajo == nil {
		trabajo = f.Prestamo()
	}

	leidos, err := f.mpq.ReadFileInto(limpiar(nombre), dst, trabajo)
	if err != nil {
		return leidos, err
	}

	f.lecturasConPrestamo++
	f.bytesLeidos += leidos

	return leidos, nil
}

// Prestamo devuelve la memoria de trabajo de la lectura, armandola la primera vez. Es la MISMA para todos
// los archivos y para los tres caminos: pedirla por archivo es lo que llenaba el semiespacio del
// recolector (DC-002.80).
func (f *Fuente) Prestamo() *d2mpq.Scratch {
	if f.prestamo == nil {
		f.prestamo = &d2mpq.Scratch{}
	}

	return f.prestamo
}

// Tramo es un archivo entero prestado por la fuente. `Datos` es del QUE LLAMA: el que pidio el tramo es el
// que lo libera, y nadie mas lo toca.
type Tramo struct {
	// Nombre: el nombre con el que se pidio (tal cual, no el del MPQ).
	Nombre string

	// Datos: el contenido del archivo. Vale hasta `Liberar`.
	Datos []byte

	fuente   *Fuente
	liberado bool
}

// Liberado dice si el tramo ya se devolvio (para no leer un `Datos` que ya no es del que llama).
func (t *Tramo) Liberado() bool { return t == nil || t.liberado }

// Liberar devuelve el tramo al monton. Si paso del umbral externo se le hace `free()` con el liberador que
// se inyecto; si no, el bloque vive en el semiespacio del recolector y lo recoge el GC (liberarlo a mano
// ahi seria corromper el monton). Es idempotente a proposito: el que llama no tiene que acordarse de si ya
// lo libero, y una doble liberacion se cuenta en vez de romper.
func (t *Tramo) Liberar() {
	if t == nil || t.liberado {
		if t != nil && t.fuente != nil {
			t.fuente.liberadosDobles++
		}

		return
	}

	t.liberado = true

	f := t.fuente
	n := len(t.Datos)

	f.liberados++
	f.bytesVivos -= n

	if n > umbralExterno && f.liberador != nil {
		f.liberador(uintptr(unsafe.Pointer(&t.Datos[0])))
		f.libresExternos++
	}

	// el contenido deja de ser del que llama: que un uso despues de liberar se vea, no que lea memoria de
	// otro archivo
	t.Datos = nil
}

// Pedir le presta al que llama el archivo ENTERO: reserva el tamano exacto (lo dice la tabla de bloques del
// MPQ, no se adivina) y lo lee adentro con la memoria de trabajo prestada. El tramo es del que llama y se
// devuelve con `Tramo.Liberar`.
//
// Es la pieza que el motor no tiene: `LoadFile` devuelve un `[]byte` sin dueno, y en la consola ese buffer
// se queda tomado para siempre.
func (f *Fuente) Pedir(nombre string) (*Tramo, error) {
	if f.mpq == nil {
		return nil, errSinMPQ
	}

	tamano, hay := f.Tamano(nombre)
	if !hay {
		f.sinTamano++
		f.pedidosFallidos++

		return nil, fmt.Errorf("%w: %s", errSinTamano, nombre)
	}

	// un archivo vacio no puede terminar en `&dst[0]`: se guarda un byte para que la direccion exista (lo
	// leido sigue siendo 0)
	reserva := tamano
	if reserva == 0 {
		reserva = 1
	}

	datos := make([]byte, reserva)

	leidos, err := f.LeerEnteroEnConPrestamo(nombre, datos, f.Prestamo())
	if err != nil {
		f.pedidosFallidos++

		return nil, err
	}

	f.pedidos++
	f.bytesPedidos += leidos
	f.bytesVivos += leidos

	return &Tramo{Nombre: nombre, Datos: datos[:leidos], fuente: f}, nil
}

// Estado: el instrumento. Es lo que hay que mirar para decir, con numeros, cuantas cargas pasaron por la
// Source nueva, por que camino, y si el dueno devolvio todo lo que pidio.
type Estado struct {
	// los tres caminos de la fuente
	Abiertos            int // Open: el stream (camino viejo, no libera)
	LecturasEnBuffer    int // LeerEnteroEn: el destino es del que llama
	LecturasConPrestamo int // LeerEnteroEnConPrestamo: ni el destino ni la lectura piden nada
	SinTamano           int // archivos de los que no se supo el tamano

	// el prestamo explicito
	Pedidos         int   // tramos prestados
	Liberados       int   // tramos devueltos
	LibresExternos  int   // de esos, cuantos fueron un free() de verdad (pasaron el umbral)
	LiberadosDobles int   // Liberar de mas (se ignora, se cuenta)
	PedidosFallidos int   // Pedir que fallo: no se entrego buffer
	BytesPedidos    int64 // bytes prestados en total
	BytesLeidos     int64 // bytes leidos en total (los tres caminos)
	BytesVivos      int64 // bytes prestados sin devolver (0 = el dueno devolvio todo)

	// la memoria de trabajo de la lectura
	PrestamoReservado int // bytes del prestamo (se pide una vez)
	PrestamoCrecidas  int // cuantas veces hubo que pedir mas (0 despues de la primera lectura)
	Tamanos           int // cuantas veces se pregunto el tamano
}

// Estado devuelve el instrumento de la fuente.
func (f *Fuente) Estado() Estado {
	e := Estado{
		Abiertos:            f.abiertos,
		LecturasEnBuffer:    f.lecturasEnBuffer,
		LecturasConPrestamo: f.lecturasConPrestamo,
		SinTamano:           f.sinTamano,
		Pedidos:             f.pedidos,
		Liberados:           f.liberados,
		LibresExternos:      f.libresExternos,
		LiberadosDobles:     f.liberadosDobles,
		PedidosFallidos:     f.pedidosFallidos,
		BytesPedidos:        int64(f.bytesPedidos),
		BytesLeidos:         int64(f.bytesLeidos),
		BytesVivos:          int64(f.bytesVivos),
		Tamanos:             f.tamanos,
	}

	if f.prestamo != nil {
		est := f.prestamo.Estado()
		e.PrestamoReservado = est.Reservado
		e.PrestamoCrecidas = est.Crecidas
	}

	return e
}

// RegistrarEn pone la Source PRIMERO en la lista de fuentes del loader del motor.
//
// El loader prueba las fuentes de izquierda a derecha, asi que ponerla primera es lo que hace que TODAS las
// cargas del camino de consola pasen por aca sin cambiar una linea del motor (`Loader.Sources` es un campo
// exportado): lo que el motor pida por `Load`/`TamanoDeArchivo`/`LeerEnteroEn`/`LeerEnteroEnConPrestamo`
// lo atiende esta fuente.
func (f *Fuente) RegistrarEn(l *d2loader.Loader) {
	if l == nil {
		return
	}

	fuentes := make([]asset.Source, 0, len(l.Sources)+1)
	fuentes = append(fuentes, f)
	fuentes = append(fuentes, l.Sources...)

	l.Sources = fuentes
}

// limpiar: el lector del MPQ es LITERAL con las barras invertidas y no quiere la barra inicial (la misma
// traduccion que `d2loader/mpq.cleanName`, pero sin romperse con el nombre vacio).
func limpiar(nombre string) string {
	nombre = strings.ReplaceAll(nombre, "/", "\\")

	for len(nombre) > 0 && nombre[0] == '\\' {
		nombre = nombre[1:]
	}

	return nombre
}
