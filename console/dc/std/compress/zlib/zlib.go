// Package zlib: la descompresion de zlib para la consola, PUENTEADA a la zlib que ya trae KOS.
//
// Por que un puente y no una implementacion: DEFLATE es un algoritmo grande (cientos de lineas) y la
// Dreamcast ya tiene zlib compilada y probada (kos-ports: ~/dreamcast/kos-ports/zlib). Escribir otra
// seria trabajo y riesgo por nada. Aca se envuelve la de KOS detras de la MISMA interfaz que la de Go,
// asi el codigo del motor (d2mpq, d2util/assets) compila sin enterarse de donde viene.
//
// Lo que NO esta: el lado de compresion (NewWriter) y las variantes de diccionario. El juego lee
// assets comprimidos y nunca escribe zlib en la consola, asi que no hacen falta; si algun dia hacen
// falta, se agregan con su prueba.
//
// DC-002.40: EL DEFECTO QUE ESTE PAQUETE TENIA, MEDIDO. Hasta aca el Reader descomprimia TODO de una en la
// primera lectura, con `d2c_zlib_inflate` y el destino que calcula la REGLA `4 x comprimido + 64 KB`
// (`d2c_zlib_inflate_bound`). Medido con los bytes BUENOS de un sector del `(listfile)` de patch_d2.mpq
// (662 bytes guardados, 4.096 de salida, destino de 68.180: la regla sobraba 16 veces) en el POC
// `console/dc/poc/compr-40`:
//
//	COMPR-40 M2 puente de una sola vez FALLO (devolvio -1 ) cota= 68180 ms= 0     <- el defecto
//	COMPR-40 M3 puente en streaming bytes= 4096 crc32= 2274402335                 <- los numeros del host
//	COMPR-40 M4 compress/zlib FALLO al leer: zlib: invalid header                 <- el sintoma que se veia
//
// O sea: el `zlib: invalid header` que salia al leer un archivo COMPRIMIDO de un MPQ no era ni el dato
// (los bytes del sector coinciden con el host byte a byte: 662 / crc32 3898719478) ni el tamano (la cota
// sobraba): era el puente de UNA SOLA VEZ. El camino que SI funciona en esta consola es el inflate EN
// STREAMING de `csrc/zlib_dc.c` -- el que ya usaba `d2std/compress/gzip` desde DC-002.20, con los buffers
// como uintptr y el resultado del paso leido por funciones aparte (la convencion que el proyecto pago cara
// en DC-002.19/2.20). Este paquete pasa a usar ESE puente, igual que el gzip.
//
// Lo que cambia para el que llama: nada en la interfaz (NewReader devuelve io.ReadCloser y el Read
// devuelve los bytes descomprimidos). Lo que cambia adentro es que ya no materializa el resultado entero
// en la primera lectura: pide los bytes en el `p` del que lee, asi que el destino ES el buffer del motor.
package zlib

import (
	"d2std/io"
	"unsafe"
)

// El puente en C (csrc/zlib_dc.c) que a su vez llama a la zlib de KOS: el inflate EN STREAMING.
//
// El estado de la zlib (z_stream) vive en C y aca solo se lleva su INDICE: en la consola no se guarda un
// puntero a estado de C en el monton de Go. Los buffers van como uintptr, no como slices: un slice pasado
// a una funcion de C es justo lo que NO andaba en el puente de una sola vez.
//
//extern d2c_zlib_stream_init
func d2cZlibStreamInit() int32

//extern d2c_zlib_stream_step
func d2cZlibStreamStep(slot int32, src uintptr, srcLen int32, dst uintptr, dstCap int32) int32

//extern d2c_zlib_stream_consumido
func d2cZlibStreamConsumido(slot int32) int32

//extern d2c_zlib_stream_fin
func d2cZlibStreamFin(slot int32) int32

//extern d2c_zlib_stream_end
func d2cZlibStreamEnd(slot int32) int32

//extern d2c_zlib_stream_reset
func d2cZlibStreamReset(slot int32) int32

// Reader: lo que devuelve NewReader. Descomprime EN STREAMING y pide los bytes en el buffer del que lee.
type Reader struct {
	comprimido []byte
	pos        int // cuantos bytes del comprimido ya consumio la zlib
	slot       int32
	abierto    bool
	fin        bool
	cerrado    bool
	err        error
}

// ErrChecksum y ErrHeader: los mismos errores de Go, para que el codigo que los compara siga andando.
type errorString string

func (e errorString) Error() string { return string(e) }

const (
	ErrChecksum = errorString("zlib: invalid checksum")
	ErrHeader   = errorString("zlib: invalid header")
	ErrTrailer  = errorString("zlib: invalid trailer")
)

// NewReader devuelve un lector que descomprime r.
//
// El comprimido SI se guarda entero (el motor le pasa un `bytes.Reader` con el bloque del MPQ, que ya
// esta en memoria): lo que NO se materializa es el RESULTADO.
func NewReader(r io.Reader) (io.ReadCloser, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &Reader{comprimido: data, slot: -1}, nil
}

// NewReaderDict: la variante con diccionario. Se acepta el diccionario y se ignora (el juego no la usa
// con diccionario real); queda declarada para que el import compile.
func NewReaderDict(r io.Reader, dict []byte) (io.ReadCloser, error) {
	return NewReader(r)
}

// Read: los bytes descomprimidos, de a lo que entre en p.
//
// El bucle es por el caso normal del inflate: un paso puede CONSUMIR todo el comprimido y escribir CERO
// bytes (la zlib esta llenando su ventana), y otro puede devolver menos de lo que entra en p. Se corta
// cuando hubo salida, cuando la zlib dijo que el flujo termino, o cuando un paso no avanzo por NINGUN lado
// (ese ultimo caso es un defecto del puente y se declara como error en vez de colgar el motor en un bucle).
func (z *Reader) Read(p []byte) (int, error) {
	if z.cerrado {
		return 0, io.EOF
	}
	if z.fin {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	if !z.abierto {
		z.slot = d2cZlibStreamInit()
		if z.slot < 0 {
			z.err = ErrHeader

			return 0, z.err
		}
		z.abierto = true
	}

	for {
		resto := z.comprimido[z.pos:]
		if len(resto) == 0 {
			// se acabo el comprimido y la zlib no dijo que termino: el flujo esta cortado
			z.err = ErrHeader

			return 0, z.err
		}

		n := d2cZlibStreamStep(z.slot, uintptr(unsafe.Pointer(&resto[0])), int32(len(resto)),
			uintptr(unsafe.Pointer(&p[0])), int32(len(p)))
		consumido := int(d2cZlibStreamConsumido(z.slot))
		termino := d2cZlibStreamFin(z.slot) != 0

		z.pos += consumido

		if n < 0 {
			z.err = ErrHeader

			return 0, z.err
		}
		if termino {
			// El flujo termino: se suelta el slot ACA y no solo en Close. El motor descomprime un bloque
			// por archivo leido (`d2mpq/mpq_stream.go:deflate`) y los slots del puente son 4: si un solo
			// camino se olvidara del Close, el quinto archivo fallaria por falta de slot en vez de por sus
			// datos. Soltandolo cuando termina, eso no puede pasar.
			z.soltarSlot()
			z.fin = true
		}
		if n > 0 {
			return int(n), nil
		}
		if z.fin {
			return 0, io.EOF
		}
		if consumido == 0 {
			// ni salida ni consumo: el puente no avanza. Mejor el error que un bucle infinito.
			z.err = ErrHeader

			return 0, z.err
		}
	}
}

func (z *Reader) soltarSlot() {
	if z.abierto {
		d2cZlibStreamEnd(z.slot)
		z.abierto = false
	}
}

// Close: suelta el slot del inflate (el estado de la zlib vive en C, no en el monton de Go).
func (z *Reader) Close() error {
	z.soltarSlot()
	z.cerrado = true
	z.comprimido = nil

	return nil
}

// ==================================================================================================
// DC-002.80 -- Inflador: descomprimir EN UN DESTINO PRESTADO, reusando UN slot (el de la lectura del MPQ).
//
// POR QUE EXISTE. La lectura del MPQ descomprime un bloque por sector (124 en un .dt1 de 505 KB) y en la
// consola `NewReader` no sirve para eso: COPIA el comprimido con `io.ReadAll` y arma un `Reader` nuevo por
// bloque, o sea memoria pedida POR SECTOR. Medido en DC-002.78: leer un .dt1 por el stream pedia ~2 MB del
// semiespacio del recolector y el runtime moria con `gc_alloc: out of memory`.
//
// QUE HACE. El `z_stream` de la zlib de KOS se queda VIVO en el slot entre bloques y se reinicia con
// `inflateReset` (que conserva la ventana de 32 KB, no la vuelve a pedir). El comprimido y el destino son
// los dos arreglos que presta el que llama: aca no se pide un byte.
//
// COMO SE USA. Uno por lectura (`NuevoInflador`), `DescomprimirEn` por bloque y `Cerrar` al final -- el slot
// se devuelve ahi: el puente tiene 4 y quedarse con uno para siempre dejaria sin zlib al resto del motor.
// ==================================================================================================

// Inflador: un descompresor de bloques de zlib con destino prestado, atado a UN slot del puente.
type Inflador struct {
	slot    int32
	abierto bool
}

// NuevoInflador devuelve un inflador sin slot: el slot se toma en el primer bloque.
func NuevoInflador() *Inflador { return &Inflador{slot: -1} }

// DescomprimirEn descomprime `src` (un stream de zlib entero) dentro de `dst` y devuelve cuantos bytes
// escribio. Escribe hasta llenar `dst`: si el flujo pide mas lugar del que hay, se corta ahi (el que llama
// sabe cuanto tiene que salir y lo compara).
func (i *Inflador) DescomprimirEn(src []byte, dst []byte) (int, error) {
	if len(src) == 0 || len(dst) == 0 {
		return 0, ErrHeader
	}

	if !i.abierto {
		i.slot = d2cZlibStreamInit()
		if i.slot < 0 {
			return 0, ErrHeader
		}

		i.abierto = true
	} else if d2cZlibStreamReset(i.slot) != 0 {
		return 0, ErrHeader
	}

	total, pos := 0, 0

	for {
		if total >= len(dst) {
			return total, nil
		}

		resto := src[pos:]
		if len(resto) == 0 {
			return total, ErrHeader
		}

		n := d2cZlibStreamStep(i.slot, uintptr(unsafe.Pointer(&resto[0])), int32(len(resto)),
			uintptr(unsafe.Pointer(&dst[total])), int32(len(dst)-total))
		consumido := int(d2cZlibStreamConsumido(i.slot))
		termino := d2cZlibStreamFin(i.slot) != 0

		pos += consumido

		if n < 0 {
			return total, ErrHeader
		}

		total += int(n)

		if termino {
			return total, nil
		}

		if n == 0 && consumido == 0 {
			return total, ErrHeader
		}
	}
}

// Cerrar devuelve el slot al puente (y con el, la ventana de la zlib).
func (i *Inflador) Cerrar() {
	if i.abierto {
		d2cZlibStreamEnd(i.slot)
		i.abierto = false
	}
}
