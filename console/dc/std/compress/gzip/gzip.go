// Package gzip: la descompresion de gzip para la consola, sobre el mismo puente a la zlib de KOS que
// usa compress/zlib.
//
// gzip es zlib con otra cabecera: la zlib de KOS ya descomprime las dos (se le pide inflateInit2 con
// ventana 15+32, que es "decidi vos si es zlib o gzip"). Asi que aca no hay algoritmo nuevo: se envuelve
// el mismo puente y se expone la interfaz de Go.
//
// Lo que NO esta: el lado de escritura (NewWriter) ni las variantes con diccionario. El juego lee
// assets comprimidos y no escribe gzip en la consola.
package gzip

import (
	"d2std/io"
)

//extern d2c_zlib_inflate
func d2cZlibInflate(src []byte, srcLen int32, dst []byte, dstCap int32) int32

//extern d2c_zlib_inflate_bound
func d2cZlibInflateBound(srcLen int32) int32

// Header: la cabecera de un miembro gzip. Se completa al leer (el motor la mira para saber si es texto
// o binario, que es lo que hace gzip.Header.Text).
type Header struct {
	Name    string
	Comment string
	ModTime interface{} // time.Time en Go; aca se deja como esta porque el reloj de la consola no esta
	OS      byte
	Text    bool
}

// ErrChecksum y ErrHeader: los mismos errores de Go.
type errorString string

func (e errorString) Error() string { return string(e) }

const (
	ErrChecksum = errorString("gzip: invalid checksum")
	ErrHeader   = errorString("gzip: invalid header")
)

// Reader: el lector de gzip.
type Reader struct {
	comprimido []byte
	crudo      []byte
	pos        int
	cerrado    bool
	err        error
	Header     Header
}

// NewReader devuelve un lector que descomprime r.
func NewReader(r io.Reader) (*Reader, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	g := &Reader{comprimido: data}
	// la cabecera de gzip son 10 bytes fijos: 1f 8b, el metodo, las banderas, la fecha, y los dos
	// ultimos. Se miran solo para llenar Header (si no es gzip, la zlib va a fallar igual y con mejor
	// mensaje que inventado aca).
	if len(data) >= 10 && data[0] == 0x1f && data[1] == 0x8b {
		g.Header.OS = data[9]
		g.Header.Text = data[3]&1 != 0
	}
	return g, nil
}

func (g *Reader) descomprimir() {
	if g.crudo != nil || g.err != nil {
		return
	}
	destino := make([]byte, d2cZlibInflateBound(int32(len(g.comprimido))))
	n := d2cZlibInflate(g.comprimido, int32(len(g.comprimido)), destino, int32(len(destino)))
	if n < 0 {
		g.err = ErrHeader
		return
	}
	g.crudo = destino[:n]
}

func (g *Reader) Read(p []byte) (int, error) {
	if g.cerrado {
		return 0, io.EOF
	}
	g.descomprimir()
	if g.err != nil {
		return 0, g.err
	}
	if g.pos >= len(g.crudo) {
		return 0, io.EOF
	}
	n := copy(p, g.crudo[g.pos:])
	g.pos += n
	return n, nil
}

// Close: no hay recurso que soltar.
func (g *Reader) Close() error {
	g.cerrado = true
	g.comprimido = nil
	return nil
}
