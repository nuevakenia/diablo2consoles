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
package zlib

import (
	"io"
)

// extern del puente en C (csrc/zlib_dc.c), que a su vez llama a la zlib de KOS.
//
//extern d2c_zlib_inflate
func d2cZlibInflate(src []byte, srcLen int32, dst []byte, dstCap int32) int32

//extern d2c_zlib_inflate_bound
func d2cZlibInflateBound(srcLen int32) int32

// Reader: lo que devuelve NewReader. La interfaz que se pide es io.Reader, igual que en Go.
type Reader struct {
	comprimido []byte
	crudo      []byte
	pos        int
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

// NewReader devuelve un lector que descomprime r. A diferencia del de Go (que descomprime a medida que
// se lee), aca se descomprime TODO de una en la primera lectura: en la consola los bloques comprimidos
// del juego son chicos (los MPQ comprimen de a bloques) y tener el resultado entero en memoria es mas
// simple y mas rapido que armar la maquina de estados.
func NewReader(r io.Reader) (io.ReadCloser, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &Reader{comprimido: data, pos: 0}, nil
}

// NewReaderDict: la variante con diccionario. Se acepta el diccionario y se ignora (el juego no la usa
// con diccionario real); queda declarada para que el import compile.
func NewReaderDict(r io.Reader, dict []byte) (io.ReadCloser, error) {
	return NewReader(r)
}

func (z *Reader) descomprimir() {
	if z.crudo != nil || z.err != nil {
		return
	}
	destino := make([]byte, d2cZlibInflateBound(int32(len(z.comprimido))))
	n := d2cZlibInflate(z.comprimido, int32(len(z.comprimido)), destino, int32(len(destino)))
	if n < 0 {
		z.err = ErrHeader
		return
	}
	z.crudo = destino[:n]
}

func (z *Reader) Read(p []byte) (int, error) {
	if z.cerrado {
		return 0, io.EOF
	}
	z.descomprimir()
	if z.err != nil {
		return 0, z.err
	}
	if z.pos >= len(z.crudo) {
		return 0, io.EOF
	}
	n := copy(p, z.crudo[z.pos:])
	z.pos += n
	return n, nil
}

// Close: no hay recurso que soltar (los buffers los libera el recolector), pero la interfaz la pide.
func (z *Reader) Close() error {
	z.cerrado = true
	z.comprimido = nil
	return nil
}
