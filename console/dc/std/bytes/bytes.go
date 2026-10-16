// Package bytes: el minimo que el motor usa de verdad (bytes.Buffer), medido al compilar
// d2common/d2datautils y d2fileformats/d2dc6 para SH-4 en DC-002.3.
//
// Limites declarados (lo que NO esta, y se agrega cuando un paquete del motor lo pida): las funciones
// de busqueda y comparacion (Index, Equal, Contains, Split, ...) y las de conversion de texto. En
// cuanto existan, la prueba diferencial del host las cubre como a las demas.
package bytes

import (
	"d2std/errors"
	"d2std/io"
)

// Buffer es un buffer de bytes, con el subconjunto que el motor usa.
type Buffer struct {
	buf []byte
}

// NewBuffer arma un Buffer con el contenido inicial dado (como en Go, el buffer usa el slice).
func NewBuffer(buf []byte) *Buffer { return &Buffer{buf: buf} }

// NewBufferString arma un Buffer con el texto dado.
func NewBufferString(s string) *Buffer { return &Buffer{buf: []byte(s)} }

// Write agrega los bytes al final.
func (b *Buffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// WriteString agrega el texto al final.
func (b *Buffer) WriteString(s string) (int, error) {
	b.buf = append(b.buf, s...)
	return len(s), nil
}

// WriteByte agrega un byte al final.
func (b *Buffer) WriteByte(c byte) error {
	b.buf = append(b.buf, c)
	return nil
}

// Bytes devuelve el contenido (sin copiar, como en Go).
func (b *Buffer) Bytes() []byte { return b.buf }

// Len devuelve cuantos bytes hay.
func (b *Buffer) Len() int { return len(b.buf) }

// Reset vacia el buffer sin soltar la memoria.
func (b *Buffer) Reset() { b.buf = b.buf[:0] }

// String devuelve el contenido como texto.
func (b *Buffer) String() string { return string(b.buf) }

// ---- Reader: lo que el motor usa para leer estructuras de un bloque ya en memoria ----
// (se adelanto de DC-002.5 porque encoding/binary.Read lo necesita para los casos del corpus)

// Reader lee de un []byte, como el de Go.
type Reader struct {
	s []byte
	i int64
}

// NewReader devuelve un Reader sobre b (sin copiar, como Go).
func NewReader(b []byte) *Reader { return &Reader{s: b} }

// Len devuelve cuantos bytes quedan por leer.
func (r *Reader) Len() int {
	if r.i >= int64(len(r.s)) {
		return 0
	}
	return int(int64(len(r.s)) - r.i)
}

// Size devuelve el tamano total.
func (r *Reader) Size() int64 { return int64(len(r.s)) }

// Read copia hasta len(p) bytes. Devuelve io.EOF cuando no queda nada (y 0, EOF si ya no hay).
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	n = copy(p, r.s[r.i:])
	r.i += int64(n)
	return n, nil
}

// ReadByte devuelve el siguiente byte.
func (r *Reader) ReadByte() (byte, error) {
	if r.i >= int64(len(r.s)) {
		return 0, io.EOF
	}
	b := r.s[r.i]
	r.i++
	return b, nil
}

// UnreadByte devuelve el ultimo byte leido.
func (r *Reader) UnreadByte() error {
	if r.i <= 0 {
		return errors.New("bytes.Reader.UnreadByte: no se leyo nada")
	}
	r.i--
	return nil
}

// ReadAt lee en b desde el offset dado, sin mover la posicion.
func (r *Reader) ReadAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("bytes.Reader.ReadAt: offset negativo")
	}
	if off >= int64(len(r.s)) {
		return 0, io.EOF
	}
	n = copy(b, r.s[off:])
	if n < len(b) {
		err = io.EOF
	}
	return n, err
}

// Seek mueve la posicion (whence: 0 desde el principio, 1 desde la posicion, 2 desde el final).
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = int64(r.i) + offset
	case 2:
		abs = int64(len(r.s)) + offset
	default:
		return 0, errors.New("bytes.Reader.Seek: whence invalido")
	}
	if abs < 0 {
		return 0, errors.New("bytes.Reader.Seek: posicion negativa")
	}
	r.i = abs
	return abs, nil
}

// Reset vuelve a leer desde el principio de b.
func (r *Reader) Reset(b []byte) {
	r.s = b
	r.i = 0
}

// WriteTo vuelca lo que queda en w a traves de su Write (el motor no lo usa, pero cierra el Reader).
func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(r.s[r.i:])
	r.i += int64(n)
	return int64(n), err
}

// ---------------------------------------------------------------- ReadFrom / WriteTo del Buffer
//
// Los pide d2mpq/mpq_stream.go:378 al descomprimir un bloque (buffer.ReadFrom(el zlib.Reader)). El
// Buffer de Go los tiene por la interfaz io.ReaderFrom/io.WriterTo; aca se implementan igual de simple:
// leer todo y pegar, y escribir todo y vaciar.

// ReadFrom lee todo lo que pueda de r y lo agrega al buffer. Devuelve cuantos bytes leyo.
//
// OJO: el Buffer de esta consola es lo mas simple que se puede (un solo slice, sin puntero de lectura
// separado), asi que ReadFrom es "leer todo del origen y pegarlo al final" y WriteTo "escribir todo y
// vaciar". Para el uso del motor (acumular un bloque descomprimido y sacarlo entero) alcanza y sobra.
func (b *Buffer) ReadFrom(r interface{ Read([]byte) (int, error) }) (n int64, err error) {
	bloque := make([]byte, 4096)
	for {
		c, e := r.Read(bloque)
		if c > 0 {
			b.buf = append(b.buf, bloque[:c]...)
			n += int64(c)
		}
		if e != nil {
			if e.Error() == "EOF" {
				return n, nil
			}
			return n, e
		}
	}
}

// WriteTo escribe todo el buffer en w y lo vacia.
func (b *Buffer) WriteTo(w interface{ Write([]byte) (int, error) }) (n int64, err error) {
	if len(b.buf) > 0 {
		c, err := w.Write(b.buf)
		n = int64(c)
		if err != nil {
			return n, err
		}
	}
	b.Reset()
	return n, nil
}
