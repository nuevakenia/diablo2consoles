// Paquete bufio para la consola: Scanner (18 usos en el motor) y Reader (10) sobre cualquier io.Reader
// de la stdlib minima, mas Writer.
//
// Lo que importa del Scanner es el contrato de Go: Scan() devuelve false al final O por un error, y
// despues hay que mirar Err() para saber cual de las dos cosas paso. El motor lee archivos de datos con
// esto (textos de tablas), asi que las lineas tienen que salir iguales que en Go:
//   - la ultima linea SIN salto de linea tambien sale,
//   - un "\r\n" deja el "\\r" (Go no lo saca: eso lo hace el que consume),
//   - una linea vacia al final (el archivo termina en salto) NO genera un token vacio de mas.
//
// En la consola el reloj y el FS dependen de KOS, pero esto NO: se prueba en el host comparando contra
// el bufio de Go (difftest/dc0026c_test.go) con un Reader de prueba propio.
package bufio

import (
	"d2std/errors"
	"d2std/io"
)

// ErrBufferFull / ErrTooLong / ErrNegativeCount: los mismos que nombra el motor.
var (
	ErrBufferFull   = errors.New("bufio: buffer full")
	ErrNegativeRead = errors.New("bufio: reader returned negative count")
	ErrTooLong      = errors.New("bufio: token too long")
)

// tamanoInicial: el mismo tamano de arranque que usa Go (4096).
const tamanoInicial = 4096

// ---------------------------------------------------------------- Reader

// Reader es un lector con buffer.
type Reader struct {
	rd  io.Reader
	buf []byte
	r   int // donde empieza lo que queda por leer
	w   int // donde termina
	err error
}

// NewReader arma un Reader con el buffer del tamano de arranque.
func NewReader(rd io.Reader) *Reader { return NewReaderSize(rd, tamanoInicial) }

// NewReaderSize arma un Reader con el buffer pedido (minimo 16, como Go).
func NewReaderSize(rd io.Reader, size int) *Reader {
	if size < 16 {
		size = 16
	}
	b := &Reader{rd: rd, buf: make([]byte, size)}
	b.r = size
	b.w = size
	return b
}

// Buffered: cuantos bytes hay ya en el buffer.
func (b *Reader) Buffered() int { return b.w - b.r }

// Reset cambia el lector de abajo y tira lo que haya en el buffer.
func (b *Reader) Reset(rd io.Reader) {
	b.rd = rd
	b.r = len(b.buf)
	b.w = len(b.buf)
	b.err = nil
}

// rellenar: mueve lo que quedo al principio del buffer y lee mas. Devuelve el error de la lectura.
func (b *Reader) rellenar() error {
	if b.r > 0 {
		copy(b.buf, b.buf[b.r:b.w])
		b.w -= b.r
		b.r = 0
	}
	if b.w >= len(b.buf) {
		return ErrBufferFull
	}
	n, err := b.rd.Read(b.buf[b.w:])
	if n < 0 {
		return ErrNegativeRead
	}
	b.w += n
	if err != nil {
		b.err = err
	}
	return err
}

// Read lee hasta len(p) bytes, usando lo que ya este en el buffer primero.
func (b *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.Buffered() == 0 && b.err == nil {
		if len(p) >= len(b.buf) {
			// el pedido es mas grande que el buffer: se lee derecho
			n, err := b.rd.Read(p)
			if n < 0 {
				return 0, ErrNegativeRead
			}
			return n, err
		}
		b.rellenar()
	}
	n := copy(p, b.buf[b.r:b.w])
	b.r += n
	if n == 0 && b.err != nil {
		return 0, b.err
	}
	return n, nil
}

// ReadByte lee un byte.
func (b *Reader) ReadByte() (byte, error) {
	var c [1]byte
	p := c[:]
	_, err := b.Read(p)
	if err != nil {
		return 0, err
	}
	return c[0], nil
}

// ReadBytes lee hasta el separador (incluido). El separador lo elige el que llama.
func (b *Reader) ReadBytes(delim byte) ([]byte, error) {
	var salida []byte
	for {
		if b.Buffered() == 0 && b.err != nil {
			return salida, b.err
		}
		trozo := b.buf[b.r:b.w]
		en := -1
		for i := 0; i < len(trozo); i++ {
			if trozo[i] == delim {
				en = i
				break
			}
		}
		if en >= 0 {
			salida = append(salida, trozo[:en+1]...)
			b.r += en + 1
			return salida, nil
		}
		salida = append(salida, trozo...)
		b.r = b.w
		if err := b.rellenar(); err != nil {
			return salida, err
		}
	}
}

// ReadString es ReadBytes devolviendo un string.
func (b *Reader) ReadString(delim byte) (string, error) {
	d, err := b.ReadBytes(delim)
	return string(d), err
}

// ---------------------------------------------------------------- Writer

// Writer es un escritor con buffer.
type Writer struct {
	wr  io.Writer
	buf []byte
	n   int
	err error
}

// NewWriter arma un Writer de 4096 (el mismo que Go).
func NewWriter(wr io.Writer) *Writer { return NewWriterSize(wr, tamanoInicial) }

// NewWriterSize arma un Writer del tamano pedido.
func NewWriterSize(wr io.Writer, size int) *Writer {
	if size <= 0 {
		size = tamanoInicial
	}
	return &Writer{wr: wr, buf: make([]byte, size)}
}

// Buffered: cuantos bytes hay sin escribir todavia.
func (b *Writer) Buffered() int { return b.n }

// Flush escribe lo que quedo en el buffer.
//
// El SHORT WRITE se detecta, como en Go: si la salida de abajo escribe menos bytes de los que le
// pidieron SIN dar error, queda io.ErrShortWrite (y los bytes que no salieron se conservan en el
// buffer). Sin esto el error se pierde: el que escribe cree que escribio todo y Error() devuelve nil.
// Medido en DC-002.5b con el Writer de csv (ver io.ErrShortWrite).
func (b *Writer) Flush() error {
	if b.err != nil {
		return b.err
	}
	if b.n == 0 {
		return nil
	}
	n, err := b.wr.Write(b.buf[:b.n])
	if n < b.n && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < b.n {
			copy(b.buf[0:b.n-n], b.buf[n:b.n])
		}
		b.n -= n
		b.err = err
		return err
	}
	b.n = 0
	return nil
}

// Write escribe (y vacia el buffer si no entra).
func (b *Writer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if b.n+len(p) > len(b.buf) {
		if err := b.Flush(); err != nil {
			return 0, err
		}
		if len(p) >= len(b.buf) {
			n, err := b.wr.Write(p)
			if err != nil {
				b.err = err
			}
			return n, err
		}
	}
	b.n += copy(b.buf[b.n:], p)
	return len(p), nil
}

// WriteString escribe un texto.
func (b *Writer) WriteString(s string) (int, error) { return b.Write([]byte(s)) }

// WriteByte escribe un byte.
func (b *Writer) WriteByte(c byte) error {
	if b.err != nil {
		return b.err
	}
	if b.n >= len(b.buf) {
		if err := b.Flush(); err != nil {
			return err
		}
	}
	b.buf[b.n] = c
	b.n++
	return nil
}

// ---------------------------------------------------------------- Scanner

// SplitFunc mira el buffer y decide donde termina el token.
type SplitFunc func(data []byte, atEOF bool) (advance int, token []byte, err error)

// Scanner parte un Reader en tokens (lineas, palabras o bytes).
type Scanner struct {
	r        io.Reader
	buf      []byte
	rdStart  int
	rdEnd    int
	split    SplitFunc
	token    []byte
	scanErr  error
	done     bool
	maxToken int
	inicio   bool
}

// MaxScanTokenSize es el tope del token (el mismo que Go).
const MaxScanTokenSize = 64 * 1024

// NewScanner arma un Scanner que parte en lineas (SplitLines, el mismo que usa el motor).
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{
		r:        r,
		buf:      make([]byte, tamanoInicial),
		split:    ScanLines,
		maxToken: MaxScanTokenSize,
		inicio:   true,
	}
}

// Split cambia la forma de partir.
func (s *Scanner) Split(split SplitFunc) { s.split = split }

// Buffer cambia el buffer de trabajo.
func (s *Scanner) Buffer(buf []byte, max int) {
	if s.inicio {
		s.buf = buf
		s.maxToken = max
	}
}

// Err: nil si se llego al final, o el error que corto la lectura.
func (s *Scanner) Err() error {
	if s.scanErr == io.EOF {
		return nil
	}
	return s.scanErr
}

// Bytes: el token que acaba de salir.
func (s *Scanner) Bytes() []byte { return s.token }

// Text: el token como texto.
func (s *Scanner) Text() string { return string(s.token) }

// Scan avanza al proximo token. Devuelve false al final o si hay error (ver Err).
func (s *Scanner) Scan() bool {
	if s.done {
		return false
	}
	// se pide al Reader de abajo, llenando el buffer
	for {
		if s.rdStart > 0 && s.rdEnd == len(s.buf) {
			// se corre lo que quedo al principio para hacer lugar
			copy(s.buf, s.buf[s.rdStart:s.rdEnd])
			s.rdEnd -= s.rdStart
			s.rdStart = 0
		}
		atEOF := s.scanErr == io.EOF
		avanzar, token, err := s.split(s.buf[s.rdStart:s.rdEnd], atEOF)
		if err != nil {
			s.scanErr = err
			return false
		}
		if avanzar > 0 {
			s.rdStart += avanzar
			s.token = token
			// el token se copia porque el buffer se mueve en la proxima vuelta
			copia := make([]byte, len(token))
			copy(copia, token)
			s.token = copia
			return true
		}
		if atEOF {
			s.done = true
			return false
		}
		// hace falta mas datos
		if s.rdEnd == len(s.buf) {
			if len(s.buf) >= s.maxToken {
				s.scanErr = ErrTooLong
				return false
			}
			nuevo := make([]byte, len(s.buf)*2)
			if len(nuevo) > s.maxToken {
				nuevo = make([]byte, s.maxToken)
			}
			copy(nuevo, s.buf[s.rdStart:s.rdEnd])
			s.rdEnd -= s.rdStart
			s.rdStart = 0
			s.buf = nuevo
		}
		n, err := s.r.Read(s.buf[s.rdEnd:])
		if n > 0 {
			s.rdEnd += n
			s.inicio = false
		}
		if err != nil {
			s.scanErr = err
			// se vuelve a probar el split con lo que hay y atEOF, en la proxima vuelta
			s.done = true
			// el ultimo token sin separador final igual sale: se lo pide al split una vez mas
			av2, tok2, err2 := s.split(s.buf[s.rdStart:s.rdEnd], true)
			s.done = false
			if err2 != nil {
				s.scanErr = err2
				return false
			}
			if av2 > 0 && len(tok2) > 0 {
				s.rdStart += av2
				copia := make([]byte, len(tok2))
				copy(copia, tok2)
				s.token = copia
				return true
			}
			s.done = true
			if s.scanErr == io.EOF {
				return false
			}
			return false
		}
	}
}

// ---------------------------------------------------------------- los Split

// ScanLines: una linea por token, sin el salto. Un "\r" pegado al salto SI se saca (dropCR de Go): si
// no, las lineas de un archivo con finales de Windows salen con un byte de mas, y eso se vio en la
// prueba diferencial ("con retorno\r" contra "con retorno").
func ScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			return i + 1, sinCR(data[:i]), nil
		}
	}
	if atEOF {
		return len(data), sinCR(data), nil
	}
	return 0, nil, nil
}

// sinCR saca el "\r" del final (dropCR en Go).
func sinCR(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		return b[:len(b)-1]
	}
	return b
}

// ScanWords: una palabra por token (blancos: espacio, tab, salto y retorno).
//
// La palabra solo sale cuando aparece un blanco o cuando se llego al final: si sale antes, con un pedazo
// de palabra en el buffer, cada trozo se convierte en un token. Ese fue el defecto que cazo la prueba
// diferencial (con el archivo servido de a un byte, "una linea" salia como "u","n","a",... en vez de
// dos palabras: 8 tokens donde Go da 2).
func ScanWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	i := 0
	for i < len(data) && esBlanco(data[i]) {
		i++
	}
	empieza := i
	for i < len(data) {
		if esBlanco(data[i]) {
			return i + 1, data[empieza:i], nil
		}
		i++
	}
	if atEOF && len(data) > empieza {
		return len(data), data[empieza:], nil
	}
	return 0, nil, nil
}

// ScanBytes: un byte por token.
func ScanBytes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return 0, nil, nil
	}
	return 1, data[:1], nil
}

func esBlanco(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// ReadLine: la version de go (devuelve la linea sin el fin de linea y avisa si hubo mas de lo que
// entra). El motor la usa al leer texto.
//
// Se arma leyendo byte a byte con ReadByte, que es lo que este Reader si sabe hacer: no hay ReadSlice
// ni buffer de "hasta el delimitador" en esta implementacion, y agregarlos solo para esto no vale la
// pena mientras el uso sea leer texto de configuracion.
func (b *Reader) ReadLine() (line []byte, isPrefix bool, err error) {
	for {
		c, e := b.ReadByte()
		if e != nil {
			if len(line) > 0 {
				return line, false, nil
			}
			return nil, false, e
		}
		if c == '\n' {
			if n := len(line); n >= 1 && line[n-1] == '\r' {
				line = line[:n-1]
			}
			return line, false, nil
		}
		line = append(line, c)
		if len(line) >= 65536 {
			return line, true, nil
		}
	}
}
