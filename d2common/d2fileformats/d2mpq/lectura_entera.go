package d2mpq

// DC-002.79 -- LEER UN ARCHIVO ENTERO SIN EL STREAM: TAMANO PRIMERO, BUFFER EXACTO, SIN `ReadAll`.
//
// EL DEFECTO QUE ARREGLA (medido en DC-002.76 y DC-002.78, no supuesto). El camino del motor para llevarse
// un archivo entero a memoria es `io.ReadAll` sobre el stream del MPQ (`d2asset.LoadFile`). En la consola
// eso muere con los archivos grandes:
//
//	.ds1 de 57.652 B          ReadAll OK
//	primer .dt1 de 505.000 B  ReadAll -> "Unhandled exception ... Data address error (read)", PC en io.ReadAll
//	animdata.d2 de 411.104 B  idem
//
// y con `mpq.ReadFile` (buffer del tamano exacto, UNA lectura) esos mismos archivos se leen sin problema.
// La diferencia entre los dos caminos no es el MPQ: es CUANTA MEMORIA PIDE CADA UNO.
//
//  1. `io.ReadAll` no sabe cuanto mide el archivo, asi que va duplicando el buffer: 512 B, 1 KB, 2 KB...
//     hasta pasarse del tamano. En libgodc todo pedido de mas de 64 KB (`GC_LARGE_OBJECT_THRESHOLD`) va a
//     `gc_external_alloc` (`gc_heap.c:116`), que es `malloc` pelado y NO lo recolecta nadie: de esa cadena
//     de duplicaciones quedan tomados ~2 MB por archivo de 505 KB, para siempre (medido en DC-002.77:
//     `poc/sonda-gc-277`).
//  2. Ademas cada `Read` del stream prefetchea el archivo COMPRIMIDO ENTERO (`sectorBytes`,
//     `prefetchLimit = 2 MB`) en un buffer propio de ese stream que tampoco se libera. Otro medio megabyte
//     tomado por archivo.
//
// LA CURA, de a una: aca se agrega lo que el que llama necesita para no tener que adivinar el tamano
// (`TamanoDeArchivo`) ni duplicar el buffer (`LeerEn`: lee ADENTRO del arreglo que le prestan), y una
// palanca para apagar el prefetch cuando la memoria es mas cara que los sectores (`InhibirPrefetch`).
//
// QUE NO CAMBIA. `ReadFile` y `ReadFileStream` quedan como estan: `ReadFile` ya reservaba el tamano exacto
// (por eso funciona) y el stream sigue sirviendo para los consumidores que no quieren el archivo entero.
// Todo esto es ADITIVO: la PC no cambia de comportamiento salvo que llame a `InhibirPrefetch(true)`.

import (
	"errors"
	"io"
	"strings"
)

const (
	// trozoDeLectura: cuanto se le pide al stream de una vez cuando se lee con `LeerEn`. Es el mismo
	// tamano que usa `d2datautils.BufferGrande` y no pide memoria propia: escribe directo en el buffer
	// del que llama. Los sectores del MPQ son de 4 KB, asi que un pedido mas grande no ahorra trabajo.
	trozoDeLectura = 32 * 1024
)

var (
	// errNoEntraEnElBuffer: el archivo mide mas que el buffer que se presto. Se dice en vez de pedir
	// memoria: pedirla es justo lo que este camino evita.
	errNoEntraEnElBuffer = errors.New("d2mpq: el archivo no entra en el buffer prestado")

	// sinPrefetch: apaga el prefetch del archivo comprimido entero (ver `sectorBytes`). Lo prende la
	// consola; en la PC queda apagado (o sea: el prefetch sigue andando como siempre).
	sinPrefetch bool
)

// InhibirPrefetch prende/apaga el prefetch del archivo comprimido entero que hace `sectorBytes`.
//
// Con el prefetch PRENDIDO (el comportamiento de siempre) el primer sector de un archivo comprimido lee
// de un tiron todo el tramo comprimido (`last-first` bytes, hasta 2 MB) y lo guarda en el stream: los
// sectores siguientes se sirven de ahi. Es rapido y en la PC no cuesta nada. En la consola ese buffer pasa
// de 64 KB, se va al monton externo y no lo devuelve nadie, asi que APAGARLO cambia velocidad por memoria:
// cada sector se lee con `ReadAt` del tamano que necesita (unos pocos KB, en el semiespacio del recolector).
//
// Es una variable y no un build tag por la misma razon que `d2asset/presupuestos.go`: gccgo compila TODOS
// los .go del paquete (no filtra por tag, medido en DC-002.2).
func InhibirPrefetch(v bool) { sinPrefetch = v }

// PrefetchInhibido informa si el prefetch esta apagado (para el instrumento del POC).
func PrefetchInhibido() bool { return sinPrefetch }

// TamanoDeArchivo devuelve el tamano DESCOMPRIMIDO del archivo, que es lo que hay que reservar para
// leerlo entero. Sale de la tabla de bloques del MPQ (no hay que abrir el archivo ni leer un byte).
func (mpq *MPQ) TamanoDeArchivo(fileName string) (int, bool) {
	block, err := mpq.getFileBlockData(fileName)
	if err != nil || block == nil {
		return 0, false
	}

	// el mismo techo que `checkBlock`: un tamano hostil no puede terminar en una reserva de gigabytes
	if block.UncompressedFileSize > maxFileSize {
		return 0, false
	}

	return int(block.UncompressedFileSize), true
}

// TamanoComprimido devuelve lo que ocupa el archivo DENTRO del MPQ (lo que el stream prefetchea y lo que
// mide el `.gdi`). Se informa para medir: es la memoria que se ahorra al inhibir el prefetch.
func (mpq *MPQ) TamanoComprimido(fileName string) (int, bool) {
	block, err := mpq.getFileBlockData(fileName)
	if err != nil || block == nil {
		return 0, false
	}

	return int(block.CompressedFileSize), true
}

// LeerEn lee el archivo entero DENTRO del buffer que le prestan y devuelve cuantos bytes escribio.
//
// Es `ReadFile` pero sin pedir el buffer: el que llama decide donde vive el destino (un arena, un buffer
// que reusa entre archivos, lo que tenga). El buffer tiene que alcanzar para `TamanoDeArchivo`.
//
// Igual que `ReadFile`, cada stream se lleva su COPIA del bloque (los streams de un mismo MPQ se usan
// desde varias goroutines) y el nombre va en minusculas, que es lo que usa la semilla de cifrado.
func (mpq *MPQ) LeerEn(fileName string, dst []byte) (int, error) {
	fileBlockData, err := mpq.getFileBlockData(fileName)
	if err != nil {
		return 0, err
	}

	block := *fileBlockData
	block.FileName = strings.ToLower(fileName)

	tamano := int(block.UncompressedFileSize)
	if tamano > len(dst) {
		return 0, errNoEntraEnElBuffer
	}

	stream, err := CreateStream(mpq, &block, fileName)
	if err != nil {
		return 0, err
	}

	return leerTodoEn(stream, dst[:tamano], tamano)
}

// leerTodoEn escribe el stream entero en `dst`, de a trozos, sin pedir memoria.
func leerTodoEn(stream *Stream, dst []byte, tamano int) (int, error) {
	leido := 0

	for leido < tamano {
		pedido := tamano - leido
		if pedido > trozoDeLectura {
			pedido = trozoDeLectura
		}

		n, err := stream.Read(dst, uint32(leido), uint32(pedido))
		leido += int(n)

		if err == io.EOF {
			// el archivo se acabo: si ya estan todos los bytes, el EOF sobra
			if leido >= tamano {
				return leido, nil
			}

			return leido, io.ErrUnexpectedEOF
		}

		if err != nil {
			return leido, err
		}

		if n == 0 {
			return leido, io.ErrUnexpectedEOF
		}
	}

	return leido, nil
}
