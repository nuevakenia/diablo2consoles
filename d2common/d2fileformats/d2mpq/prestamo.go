package d2mpq

// DC-002.80 -- LEER UN ARCHIVO DEL MPQ CON LA MEMORIA PRESTADA POR EL QUE LLAMA (Fase B.1e).
//
// EL DEFECTO QUE ARREGLA (medido en DC-002.78, no supuesto). Con el recolector FRENADO (`INHIBE=1`), el
// semiespacio de 2 MB del recolector de la consola se llena DURANTE LA CARGA DEL PRIMER .dt1 y el runtime
// muere con `gc_alloc: out of memory` pidiendo 3.328 B. O sea: leer UN .dt1 de 505 KB por el stream del MPQ
// pide ~2 MB de semiespacio. El destino de la lectura ya no pedia memoria (el arena de DC-002.78); la
// LECTURA si. Los que piden, uno por archivo y leidos en `mpq_stream.go`:
//
//	loadBlockOffsets   make([]uint32, sectores+1) + make([]byte, sectores*4)   la tabla de posiciones
//	sectorBytes        make([]byte, toRead)                                    el bloque comprimido
//	decompressMulti    el bloque descompreso (bytes.Buffer de zlib, o el `make` de explode)
//
// Con 4 KB de sector y un .dt1 de 505 KB son 124 sectores: 124 bloques comprimidos + 124 descompresos por
// archivo, mas lo que pide zlib adentro. Todo eso cae en el semiespacio (`< 64 KB`), que son 2 MB.
//
// LA CURA: UN PRESTAMO. `Scratch` es la memoria que el que llama pide UNA vez (o hereda de la version
// anterior) y le presta a la lectura: la tabla de posiciones, el bloque comprimido, el bloque descompreso y
// -- la parte que se olvida facil -- el DESCOMPRESOR (el lector de zlib se reusa con `Reset`, que es lo que
// evita pedir sus tablas de Huffman por sector). `MPQ.ReadFileInto(nombre, dst, scratch)` escribe el archivo
// en `dst` sin pedir un byte: ni por sector, ni por archivo.
//
// EL CONTRATO:
//   - `dst` tiene que alcanzar para el tamano descomprimido del archivo (`TamanoDeArchivo`); si no, se
//     devuelve `ErrNoEntraEnElBuffer` en vez de pedir memoria.
//   - el Scratch NO es seguro para varias goroutines: cada uno con el suyo (el de la consola es
//     `d2asset/lectura_de_tiles.go`, que lo usa bajo candado).
//   - lo descomprimido vive en el Scratch: vale hasta la lectura siguiente (igual que un `BufferGrande`).
//
// QUE NO CAMBIA. `ReadFile`, `ReadFileStream` y `LeerEn` quedan como estan; `LeerEn` con `scratch == nil` es
// el camino de siempre. En la PC nada de esto se usa salvo que alguien pida leer con prestamo.
//
// POR QUE ACA Y NO EN OTRO LADO. El que sabe cuantas reservas hace la lectura es el que la hace: por eso el
// Scratch es de este paquete y se pasa como puntero concreto (no una interfaz), que es lo que permite
// reusarlo sin que el recolector vea un objeto nuevo por archivo.

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"strings"
)

var (
	// ErrNoEntraEnElBuffer: el archivo mide mas que el buffer que se presto (o que el tramo que queda del
	// arena). Se expone porque el que llama tiene que distinguir "no entra" -- y tirar el arena -- de un
	// fallo de verdad.
	ErrNoEntraEnElBuffer = errNoEntraEnElBuffer

	// errSectorNoEntra: el bloque comprimido o descompreso no entra en el prestamo. No pasa con los
	// tamanos de este MPQ (el sector es de 4 KB); si pasara, se dice en vez de pedir memoria.
	errSectorNoEntra = errors.New("d2mpq: el sector no entra en la memoria prestada")
)

// InfladorDePrestamo descomprime un bloque de zlib escribiendo en un destino que le prestan, sin pedir
// memoria: el que lo implementa reusa su estado entre bloques.
//
// POR QUE UN ENGANCHE Y NO UN `zlib.Reader` MAS. `compress/zlib` de la PC tiene `Reset` sobre el lector, y con
// eso el prestamo no pide nada. La zlib de la CONSOLA es otro paquete (un puente a la zlib de KOS, ver
// `console/dc/std/compress/zlib`) que no tiene `Reset` y cuyo `NewReader` COPIA el comprimido con `io.ReadAll`:
// usarlo por sector pide memoria por sector, que es justo lo que este archivo evita. Como gccgo compila el
// mismo codigo para la consola, la diferencia se resuelve con este gancho: en la consola lo conecta el
// `init` de `prestamo_de_zlib_consola.go` (archivo con tag `dreamcast`) y en la PC queda nil.
type InfladorDePrestamo interface {
	// DescomprimirEn descomprime `comprimido` (un stream de zlib completo) dentro de `dst` y devuelve
	// cuantos bytes escribio. Si no entra, dice que no entra.
	DescomprimirEn(comprimido []byte, dst []byte) (int, error)
	// Cerrar suelta el estado del descompresor (la ventana de la zlib, que en la consola vive en C).
	Cerrar()
}

// reiniciador: lo que el lector de zlib de Go sabe hacer y el de la consola no. Se declara aca con esa forma
// para no depender del nombre que le ponga el paquete de turno (`zlib.Resetter` en la PC).
type reiniciador interface {
	Reset(r io.Reader, dict []byte) error
}

var (
	// nuevoInfladorDePrestamo: el que sabe descomprimir con destino prestado en ESTA plataforma. nil en la PC.
	nuevoInfladorDePrestamo func() InfladorDePrestamo
)

// ConectarInfladorDePrestamo conecta el descompresor de zlib con destino prestado (en la consola lo hace el
// `init` de `prestamo_de_zlib_consola.go`). Con `nil` se vuelve al camino de `compress/zlib` con un lector
// por bloque.
func ConectarInfladorDePrestamo(f func() InfladorDePrestamo) { nuevoInfladorDePrestamo = f }

// Scratch es la memoria que se le PRESTA a una lectura: una tabla de posiciones, el bloque comprimido, el
// bloque descompreso y el descompresor de zlib ya armado. Se pide UNA vez y se reusa archivo tras archivo.
type Scratch struct {
	// el stream tambien vive aca: sin esto, `ReadFileInto` pediria un objeto por archivo (chico, pero
	// por archivo) y su `Block` lo obligaria a escapar al monton.
	stream Stream
	bloque Block

	posiciones    []uint32 // una por sector (+1)
	crudo         []byte   // la tabla de posiciones como bytes, antes de descifrarla
	comprimido    []byte   // el bloque comprimido de UN sector
	descomprimido []byte   // el bloque descompreso de UN sector

	// el descompresor de zlib, reusado: `zlib.NewReader` arma las tablas de Huffman de cada stream, y
	// hacerlo por sector (124 por archivo) es parte de lo que llenaba el semiespacio. `Reset` las reusa.
	// (En la consola este camino no se usa: ahi el descompresor es el de `InfladorDePrestamo`.)
	fuente             bytes.Reader
	lector             io.ReadCloser
	reinicio           reiniciador
	infladorDePrestamo InfladorDePrestamo

	// el instrumento: cuantas lecturas, cuantos sectores, y cuantas veces hubo que PEDIR memoria (que
	// tiene que quedar en cero despues de la primera lectura).
	lecturas  int
	sectores  int
	crecidas  int
	reservado int // bytes del prestamo de trabajo (posiciones + comprimido + descomprimido)
}

// EstadoDelPrestamo es el instrumento del prestamo, para informar en la consola.
type EstadoDelPrestamo struct {
	Lecturas  int // cuantos archivos se leyeron con este prestamo
	Sectores  int // cuantos bloques se leyeron/descomprimieron
	Crecidas  int // cuantas veces hubo que PEDIR memoria (tiene que ser 0 despues de la primera lectura)
	Reservado int // bytes del prestamo de trabajo
}

// Estado devuelve el instrumento.
func (s *Scratch) Estado() EstadoDelPrestamo {
	return EstadoDelPrestamo{Lecturas: s.lecturas, Sectores: s.sectores, Crecidas: s.crecidas, Reservado: s.reservado}
}

// Crecidas: cuantas veces hubo que PEDIR memoria para el prestamo. Con el prestamo caliente este numero NO
// tiene que crecer: si crece, es que se reservo de menos.
func (s *Scratch) Crecidas() int { return s.crecidas }

// Reiniciar pone los contadores del instrumento en cero (el prestamo NO se libera: se reusa).
func (s *Scratch) Reiniciar() {
	s.lecturas, s.sectores = 0, 0
}

// Reservar deja el prestamo con lugar para un archivo de `sectores` sectores de 4 KB (o los que diga el
// MPQ). Solo PIDE memoria cuando la version anterior no alcanza: la primera lectura de una zona pide todo
// junto y las siguientes no piden nada.
func (s *Scratch) Reservar(sector int, sectores int) {
	if sector <= 0 {
		sector = 4096
	}

	s.posiciones = s.crecerUint32(sectores)
	s.crudo = s.crecer(s.crudo, sectores*4)
	// `loadBlock` rechaza un bloque comprimido de mas de 2 sectores (ver mpq_stream.go), asi que ese es el
	// techo que hay que tener prestado.
	s.comprimido = s.crecer(s.comprimido, 2*sector)
	s.descomprimido = s.crecer(s.descomprimido, sector)

	s.reservado = cap(s.crudo) + cap(s.comprimido) + cap(s.descomprimido) + cap(s.posiciones)*4

	// el descompresor de la plataforma se pide UNA vez, aca, y no en el primer bloque: es parte del
	// prestamo (en la consola es un slot de la zlib de KOS con su ventana de 32 KB, que se reusa con
	// `inflateReset` entre sectores)
	if s.infladorDePrestamo == nil && nuevoInfladorDePrestamo != nil {
		s.infladorDePrestamo = nuevoInfladorDePrestamo()
	}
}

func (s *Scratch) crecer(b []byte, n int) []byte {
	if cap(b) >= n {
		return b[:n]
	}

	s.crecidas++

	return make([]byte, n)
}

func (s *Scratch) crecerUint32(n int) []uint32 {
	if cap(s.posiciones) >= n {
		return s.posiciones[:n]
	}

	s.crecidas++

	return make([]uint32, n)
}

// descomprimir deja un bloque descompreso en el prestamo y devuelve la vista. Es `decompressMulti` pero
// escribiendo en la memoria prestada: para zlib y PKWare no pide nada; los metodos de sonido (ADPCM,
// Huffman, combos) se van por el camino de siempre, que no pasa por este prestamo.
func (s *Scratch) descomprimir(data []byte, largo uint32) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty compressed sector")
	}

	n := int(largo)
	if n < 0 || n > len(s.descomprimido) {
		return nil, errSectorNoEntra
	}

	dst := s.descomprimido[:n]

	switch data[0] {
	case 2: // ZLib/Deflate: es el de los .dt1 y los .ds1 de d2data.mpq
		if err := s.inflar(dst, data[1:]); err != nil {
			return nil, err
		}

		return dst, nil
	case 8: // PKLib/Implode
		if _, err := explodeEn(data[1:], dst); err != nil {
			return nil, err
		}

		return dst, nil
	}

	return decompressMulti(data, largo)
}

// inflar descomprime un stream de zlib DENTRO de `dst`, reusando el descompresor entre sectores.
func (s *Scratch) inflar(dst []byte, comprimido []byte) error {
	// consola: el descompresor con destino prestado (el puente a la zlib de KOS), que se reusa con un
	// `inflateReset` y no copia el comprimido
	if s.infladorDePrestamo != nil {
		n, err := s.infladorDePrestamo.DescomprimirEn(comprimido, dst)
		if err != nil {
			return err
		}

		if n != len(dst) {
			return io.ErrUnexpectedEOF
		}

		return nil
	}

	// PC: `compress/zlib`, con el lector reiniciado entre sectores (`Reset` reusa sus tablas de Huffman)
	s.fuente.Reset(comprimido)

	switch {
	case s.reinicio != nil:
		if err := s.reinicio.Reset(&s.fuente, nil); err != nil {
			return err
		}
	default:
		lector, err := zlib.NewReader(&s.fuente)
		if err != nil {
			return err
		}

		s.lector = lector
		s.reinicio, _ = lector.(reiniciador)
	}

	if _, err := io.ReadFull(s.lector, dst); err != nil {
		return err
	}

	return nil
}

// ReadFileInto lee el archivo entero DENTRO del buffer que le prestan y no pide memoria: ni por archivo ni
// por sector (la tabla de posiciones, los bloques y el descompresor viven en `scratch`).
//
// Es `LeerEn` con la memoria del trabajo prestada. Con `scratch == nil` se cae al camino de `LeerEn`.
func (mpq *MPQ) ReadFileInto(fileName string, dst []byte, scratch *Scratch) (int, error) {
	if scratch == nil {
		return mpq.LeerEn(fileName, dst)
	}

	fileBlockData, err := mpq.getFileBlockData(fileName)
	if err != nil {
		return 0, err
	}

	// el nombre en minusculas es lo que usa la semilla de cifrado (igual que `ReadFile`/`LeerEn`)
	return mpq.leerBloqueEnPrestamo(fileBlockData, strings.ToLower(fileName), dst, scratch)
}

// leerBloqueEnPrestamo es el cuerpo de `ReadFileInto`, con el bloque ya resuelto: sirve tambien para leer un
// bloque que se eligio por indice (las pruebas del archivo real del juego, que no recorre nombres).
func (mpq *MPQ) leerBloqueEnPrestamo(block *Block, nombre string, dst []byte, scratch *Scratch) (int, error) {
	// igual que `ReadFile`: cada stream se lleva su copia del bloque, y el bloque vive en el prestamo
	scratch.bloque = *block
	scratch.bloque.FileName = nombre

	tamano := int(scratch.bloque.UncompressedFileSize)
	if tamano > len(dst) {
		return 0, ErrNoEntraEnElBuffer
	}

	// el stream se reusa (no se pide uno nuevo por archivo) y apunta a las dos cosas del prestamo
	scratch.stream = Stream{
		MPQ:      mpq,
		Block:    &scratch.bloque,
		Index:    0xFFFFFFFF, //nolint:gomnd // MPQ magic
		prestamo: scratch,
	}

	stream := &scratch.stream

	if stream.Block.HasFlag(FileFixKey) {
		stream.Block.calculateEncryptionSeed(nombre)
	}

	if err := mpq.checkBlock(stream.Block); err != nil {
		return 0, err
	}

	stream.Size = 0x200 << stream.MPQ.header.BlockSize //nolint:gomnd // MPQ magic

	if stream.Block.HasFlag(FilePatchFile) {
		return 0, errors.New("patching is not supported")
	}

	// lo que hay que tener prestado para ESTE archivo (la tabla de posiciones, el bloque comprimido y el
	// descompreso). Solo pide memoria la primera vez, o cuando aparece un archivo con mas sectores que los
	// anteriores. Se reserva para TODOS los archivos, comprimidos o no: un archivo guardado tal cual
	// tambien trae su bloque a traves del prestamo.
	sectores := int((stream.Block.UncompressedFileSize+stream.Size-1)/stream.Size) + 1
	scratch.Reservar(int(stream.Size), sectores)

	if (stream.Block.HasFlag(FileCompress) || stream.Block.HasFlag(FileImplode)) &&
		!stream.Block.HasFlag(FileSingleUnit) {
		if err := stream.loadBlockOffsets(); err != nil {
			return 0, err
		}
	}

	scratch.lecturas++

	return leerTodoEn(stream, dst[:tamano], tamano)
}
