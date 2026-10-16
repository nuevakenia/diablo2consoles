// Package io: lo que el motor usa de verdad (Reader, Writer y EOF) mas lo que necesita nuestro fmt.
//
// Lo que el motor pide (medido al compilar d2common/d2datautils y d2dc6 para SH-4, DC-002.3):
// io.EOF, io.Reader y io.Writer. En la consola el destino final lo enchufa el arranque (consola
// serie o pantalla); aca solo vive el contrato.
package io

import "d2std/errors"

// EOF es el error que devuelve un Reader cuando no queda nada por leer.
var EOF = errors.New("EOF")

// Reader es el contrato de lectura, igual que en Go.
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer es el contrato de escritura, igual que en Go.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// StringWriter lo implementan los que saben escribir un string sin convertirlo antes.
type StringWriter interface {
	WriteString(s string) (n int, err error)
}

// Discard se traga todo (util para medir sin ensuciar la salida).
var Discard Writer = discard{}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// WriteString escribe s en w usando WriteString si esta disponible.
func WriteString(w Writer, s string) (n int, err error) {
	if sw, ok := w.(StringWriter); ok {
		return sw.WriteString(s)
	}
	return w.Write([]byte(s))
}

// ByteWriter: requerido por encoding/binary.
type ByteWriter interface {
	WriteByte(c byte) error
}

// ErrUnexpectedEOF es lo que devuelve ReadFull cuando el archivo se corta a mitad de dato: el motor
// lo usa para detectar archivos truncados (io.ErrUnexpectedEOF, en la lista de simbolos del muro).
var ErrUnexpectedEOF = errors.New("unexpected EOF")

// ReadFull lee len(buf) bytes de r. Devuelve EOF solo si no leyo NADA: si leyo una parte y se
// corto, devuelve ErrUnexpectedEOF (es la semantica de Go, y es la que el motor espera para decir
// "archivo truncado" en vez de "archivo vacio").
func ReadFull(r Reader, buf []byte) (n int, err error) {
	for n < len(buf) {
		m, e := r.Read(buf[n:])
		n += m
		if e == EOF {
			if n == 0 {
				return 0, EOF
			}
			return n, ErrUnexpectedEOF
		}
		if e != nil {
			return n, e
		}
	}
	return n, nil
}

// ReadAll lee hasta EOF.
func ReadAll(r Reader) ([]byte, error) {
	var buf []byte
	var tmp [4096]byte
	for {
		n, err := r.Read(tmp[:])
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err == EOF {
			return buf, nil
		}
		if err != nil {
			return buf, err
		}
	}
}

// ---------------------------------------------------------------- Seek

// Las constantes de Seek, con los MISMOS valores que en Go (un Seeker del motor las usa para
// posicionarse en un archivo del MPQ).
const (
	SeekStart   = 0 // busca desde el principio del archivo
	SeekCurrent = 1 // busca desde el offset actual
	SeekEnd     = 2 // busca desde el final
)

// Seeker es la interfaz de Go, igual.
type Seeker interface {
	Seek(offset int64, whence int) (int64, error)
}

// ReadSeeker, ReadWriteSeeker y ReadCloser: las combinaciones que usa el motor.
type ReadSeeker interface {
	Reader
	Seeker
}

type ReadWriteSeeker interface {
	Reader
	Writer
	Seeker
}

type ReadCloser interface {
	Reader
	Closer
}

type Closer interface {
	Close() error
}

type ReadWriteCloser interface {
	Reader
	Writer
	Closer
}

// NopCloser: devuelve un Closer que no hace nada (el equivalente al de Go, para envolver un Reader).
func NopCloser(r Reader) ReadCloser { return nopCloser{r} }

type nopCloser struct{ Reader }

func (nopCloser) Close() error { return nil }

// ---------------------------------------------------------------- SectionReader

// SectionReader: la version de io.SectionReader de Go: una ventana sobre un ReaderAt. La usa d2mpq
// para leer una parte del MPQ.
type SectionReader struct {
	r     ReaderAt
	base  int64
	off   int64
	limit int64
}

// ReaderAt es la interfaz de Go, igual.
type ReaderAt interface {
	ReadAt(p []byte, off int64) (n int, err error)
}

// NewSectionReader devuelve un Reader que lee n bytes del ReaderAt r empezando en off.
func NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader {
	return &SectionReader{r: r, base: off, off: off, limit: off + n}
}

func (s *SectionReader) Read(p []byte) (n int, err error) {
	if s.off >= s.limit {
		return 0, ioEOF
	}
	if max := s.limit - s.off; int64(len(p)) > max {
		p = p[0:max]
	}
	n, err = s.r.ReadAt(p, s.off)
	s.off += int64(n)
	return
}

// Size: el tamano de la ventana.
func (s *SectionReader) Size() int64 { return s.limit - s.base }

// Seek: se mueve dentro de la ventana (con las mismas constantes que io).
func (s *SectionReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case SeekStart:
		s.off = s.base + offset
	case SeekCurrent:
		s.off += offset
	case SeekEnd:
		s.off = s.limit + offset
	default:
		return 0, errWhence
	}
	if s.off < s.base {
		return 0, errOffset
	}
	if s.off > s.limit {
		return 0, errOffset
	}
	return s.off - s.base, nil
}

// ReadAt: lee dentro de la ventana sin mover la posicion.
func (s *SectionReader) ReadAt(p []byte, off int64) (n int, err error) {
	off += s.base
	if max := s.limit - off; int64(len(p)) > max {
		p = p[0:max]
		n, err = s.r.ReadAt(p, off)
		if err == nil {
			err = ioEOF
		}
		return n, err
	}
	return s.r.ReadAt(p, off)
}

var ioEOF error = stringError("EOF")

type stringError string

func (e stringError) Error() string { return string(e) }

var (
	errWhence = stringError("Seek: invalid whence")
	errOffset = stringError("Seek: invalid offset")
)

// ioEOF ya esta declarado arriba? si no, se declara aca.
