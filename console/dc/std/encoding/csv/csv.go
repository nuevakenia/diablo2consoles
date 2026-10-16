// Package csv: lectura y escritura de CSV, con la parte que el motor usa.
//
// Por que existe: en sh-elf no hay biblioteca estandar de Go, y el motor lee sus tablas de texto con
// este paquete (8 usos de csv.NewReader medidos con scripts/console/audit-stdlib.py): las tablas de
// itemstats/TXT con separador de tabulador y comillas, y con filas de largo distinto. Los campos que
// el motor toca de verdad son Comma ('\t'), FieldsPerRecord (-1), LazyQuotes (true), ReuseRecord y
// los metodos Read y ReadAll. El Writer no lo usa el motor, pero va igual (es la otra mitad del
// formato y es la mitad que se puede probar facil).
//
// Se escribe portando el encoding/csv de Go 1.27 (reader.go y writer.go) linea por linea: la prueba
// diferencial compara fila por fila y error por error contra el de Go, incluido el TEXTO de los
// errores (que trae linea y columna) y el de un Writer con separador invalido.
//
// LO QUE NO ES IGUAL A GO, y esta declarado:
//
//  1. TrimLeadingSpace solo mira el espacio ASCII (espacio, \t, \n, \v, \f, \r). Go usa
//     unicode.IsSpace, que ademas salta los espacios de otras escrituras (U+00A0, U+2000...): en la
//     consola no hay tablas de Unicode. El motor no usa TrimLeadingSpace.
//  2. fieldNeedsQuotes entrecomilla si el campo EMPIEZA con un espacio ASCII: igual que Go para el
//     espacio ASCII, y no lo hace para un espacio no-ASCII (mismo motivo que el punto 1).
//  3. FieldPos y InputOffset estan (son gratis: las posiciones ya se cuentan para los errores), pero
//     no hay testigo del motor que los use: se prueban en el host en la prueba diferencial.
package csv

import (
	"d2std/bufio"
	"d2std/bytes"
	"d2std/errors"
	"d2std/io"
	"d2std/strconv"
	"d2std/strings"
	"d2std/unicode/utf8"
)

// A ParseError es el error de un registro mal formado. Line y Column arrancan en 1.
type ParseError struct {
	StartLine int   // linea donde arranca el registro
	Line      int   // linea donde ocurrio el error
	Column    int   // columna (indice de byte, desde 1) donde ocurrio el error
	Err       error // el error de fondo
}

func (e *ParseError) Error() string {
	if e.Err == ErrFieldCount {
		return "record on line " + strconv.Itoa(e.Line) + ": " + e.Err.Error()
	}
	if e.StartLine != e.Line {
		return "record on line " + strconv.Itoa(e.StartLine) + "; parse error on line " + strconv.Itoa(e.Line) +
			", column " + strconv.Itoa(e.Column) + ": " + e.Err.Error()
	}
	return "parse error on line " + strconv.Itoa(e.Line) + ", column " + strconv.Itoa(e.Column) + ": " + e.Err.Error()
}

func (e *ParseError) Unwrap() error { return e.Err }

// Los errores que pueden venir en ParseError.Err.
var (
	ErrBareQuote  = errors.New("bare \" in non-quoted-field")
	ErrQuote      = errors.New("extraneous or missing \" in quoted-field")
	ErrFieldCount = errors.New("wrong number of fields")

	// ErrTrailingComma: Go lo dejo de usar (queda por compatibilidad).
	ErrTrailingComma = errors.New("extra delimiter at end of line")
)

var errInvalidDelim = errors.New("csv: invalid field or comment delimiter")

func validDelim(r rune) bool {
	return r != 0 && r != '"' && r != '\r' && r != '\n' && utf8.RuneLen(r) > 0 && r != utf8.RuneError
}

// esEspacioASCII: el espacio de Go que se puede ver sin tablas de Unicode (ver el punto 1).
func esEspacioASCII(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

// indexRune: el primer indice de r en b, o -1. (bytes.IndexRune de Go no esta en la stdlib minima.)
func indexRune(b []byte, r rune) int {
	if r < utf8.RuneSelf {
		return bytes.IndexByte(b, byte(r))
	}
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], r)
	if n <= 0 {
		return -1
	}
	return bytes.Index(b, buf[:n])
}

// nextRune: el rune que sigue, o RuneError si los bytes no son UTF-8 valido.
func nextRune(b []byte) rune {
	r, _ := utf8.DecodeRune(b)
	return r
}

// A Reader lee registros de una entrada con formato CSV.
type Reader struct {
	// Comma es el separador de campos. NewReader lo deja en la coma.
	Comma rune
	// Comment, si no es 0, es el caracter de comentario: las lineas que arrancan con el, sin espacios
	// adelante, se saltean.
	Comment rune
	// FieldsPerRecord es cuantos campos se esperan por registro: positivo exige ese numero, 0 lo fija
	// con el primer registro, negativo no controla nada.
	FieldsPerRecord int
	// LazyQuotes permite una comilla suelta adentro de un campo.
	LazyQuotes bool
	// TrimLeadingSpace saltea el espacio al principio de cada campo.
	TrimLeadingSpace bool
	// ReuseRecord hace que Read devuelva una rebanada que comparte memoria con la anterior.
	ReuseRecord bool

	r        *bufio.Reader
	numLine  int
	offset   int64

	recordBuffer []byte
	fieldIndexes []int
	fieldPositions []position
	lastRecord   []string
}

type position struct{ line, col int }

// NewReader devuelve un Reader que lee de r.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		Comma: ',',
		r:     bufio.NewReader(r),
	}
}

// Read lee un registro. Si no queda nada devuelve nil, io.EOF.
func (r *Reader) Read() (record []string, err error) {
	if r.ReuseRecord {
		record, err = r.readRecord(r.lastRecord)
		r.lastRecord = record
	} else {
		record, err = r.readRecord(nil)
	}
	return record, err
}

// FieldPos devuelve la linea y la columna donde arranca el campo i del ultimo registro leido.
func (r *Reader) FieldPos(field int) (line, column int) {
	if field < 0 || field >= len(r.fieldPositions) {
		panic("out of range index passed to FieldPos")
	}
	p := &r.fieldPositions[field]
	return p.line, p.col
}

// InputOffset devuelve la posicion en bytes del final del ultimo registro leido.
func (r *Reader) InputOffset() int64 { return r.offset }

// ReadAll lee todo lo que queda. Terminar la entrada no es un error: devuelve err == nil.
func (r *Reader) ReadAll() (records [][]string, err error) {
	for {
		record, err := r.readRecord(nil)
		if err == io.EOF {
			return records, nil
		}
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
}

// readLine lee la linea que sigue, CON el fin de linea. Si se acaba la entrada sin fin de linea, el
// salto no viene y el error es nil (con los bytes leidos, el error nunca es io.EOF).
func (r *Reader) readLine() ([]byte, error) {
	line, err := r.r.ReadBytes('\n')
	readSize := len(line)
	if readSize > 0 && err == io.EOF {
		err = nil
		// por compatibilidad con Go: al final de la entrada se cae el \r del final
		if line[readSize-1] == '\r' {
			line = line[:readSize-1]
		}
	}
	r.numLine++
	r.offset += int64(readSize)
	// se normaliza \r\n a \n en TODAS las lineas
	if n := len(line); n >= 2 && line[n-2] == '\r' && line[n-1] == '\n' {
		line[n-2] = '\n'
		line = line[:n-1]
	}
	return line, err
}

// lengthNL: cuantos bytes tiene el \n del final.
func lengthNL(b []byte) int {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return 1
	}
	return 0
}

func (r *Reader) readRecord(dst []string) ([]string, error) {
	if r.Comma == r.Comment || !validDelim(r.Comma) || (r.Comment != 0 && !validDelim(r.Comment)) {
		return nil, errInvalidDelim
	}

	// la linea, salteando vacias y comentarios
	var line []byte
	var errRead error
	for errRead == nil {
		line, errRead = r.readLine()
		if r.Comment != 0 && nextRune(line) == r.Comment {
			line = nil
			continue // linea de comentario
		}
		if errRead == nil && len(line) == lengthNL(line) {
			line = nil
			continue // linea vacia
		}
		break
	}
	if errRead == io.EOF {
		return nil, errRead
	}

	// cada campo del registro
	var err error
	const quoteLen = 1
	commaLen := utf8.RuneLen(r.Comma)
	recLine := r.numLine
	r.recordBuffer = r.recordBuffer[:0]
	r.fieldIndexes = r.fieldIndexes[:0]
	r.fieldPositions = r.fieldPositions[:0]
	pos := position{line: r.numLine, col: 1}
parseField:
	for {
		if r.TrimLeadingSpace {
			i := indexNoEspacio(line)
			if i < 0 {
				i = len(line)
				pos.col -= lengthNL(line)
			}
			line = line[i:]
			pos.col += i
		}
		if len(line) == 0 || line[0] != '"' {
			// campo sin comillas
			i := indexRune(line, r.Comma)
			field := line
			if i >= 0 {
				field = field[:i]
			} else {
				field = field[:len(field)-lengthNL(field)]
			}
			// una comilla adentro del campo es un error (salvo LazyQuotes)
			if !r.LazyQuotes {
				if j := bytes.IndexByte(field, '"'); j >= 0 {
					col := pos.col + j
					err = &ParseError{StartLine: recLine, Line: r.numLine, Column: col, Err: ErrBareQuote}
					break parseField
				}
			}
			r.recordBuffer = append(r.recordBuffer, field...)
			r.fieldIndexes = append(r.fieldIndexes, len(r.recordBuffer))
			r.fieldPositions = append(r.fieldPositions, pos)
			if i >= 0 {
				line = line[i+commaLen:]
				pos.col += i + commaLen
				continue parseField
			}
			break parseField
		} else {
			// campo entre comillas
			fieldPos := pos
			line = line[quoteLen:]
			pos.col += quoteLen
			for {
				i := bytes.IndexByte(line, '"')
				if i >= 0 {
					// viene otra comilla
					r.recordBuffer = append(r.recordBuffer, line[:i]...)
					line = line[i+quoteLen:]
					pos.col += i + quoteLen
					switch rn := nextRune(line); {
					case rn == '"':
						// secuencia `""` (una comilla adentro)
						r.recordBuffer = append(r.recordBuffer, '"')
						line = line[quoteLen:]
						pos.col += quoteLen
					case rn == r.Comma:
						// secuencia `",` (fin del campo)
						line = line[commaLen:]
						pos.col += commaLen
						r.fieldIndexes = append(r.fieldIndexes, len(r.recordBuffer))
						r.fieldPositions = append(r.fieldPositions, fieldPos)
						continue parseField
					case lengthNL(line) == len(line):
						// secuencia `"\n` (fin de la linea)
						r.fieldIndexes = append(r.fieldIndexes, len(r.recordBuffer))
						r.fieldPositions = append(r.fieldPositions, fieldPos)
						break parseField
					case r.LazyQuotes:
						// comilla suelta
						r.recordBuffer = append(r.recordBuffer, '"')
					default:
						// comilla sin cerrar
						err = &ParseError{StartLine: recLine, Line: r.numLine, Column: pos.col - quoteLen, Err: ErrQuote}
						break parseField
					}
				} else if len(line) > 0 {
					// se acabo la linea con el campo abierto: sigue en la linea que viene
					r.recordBuffer = append(r.recordBuffer, line...)
					if errRead != nil {
						break parseField
					}
					pos.col += len(line)
					line, errRead = r.readLine()
					if len(line) > 0 {
						pos.line++
						pos.col = 1
					}
					if errRead == io.EOF {
						errRead = nil
					}
				} else {
					// fin de la entrada de golpe
					if !r.LazyQuotes && errRead == nil {
						err = &ParseError{StartLine: recLine, Line: pos.line, Column: pos.col, Err: ErrQuote}
						break parseField
					}
					r.fieldIndexes = append(r.fieldIndexes, len(r.recordBuffer))
					r.fieldPositions = append(r.fieldPositions, fieldPos)
					break parseField
				}
			}
		}
	}
	if err == nil {
		err = errRead
	}

	// todos los campos en un solo texto, y cada uno apuntando a su pedazo
	str := string(r.recordBuffer)
	dst = dst[:0]
	if cap(dst) < len(r.fieldIndexes) {
		dst = make([]string, len(r.fieldIndexes))
	}
	dst = dst[:len(r.fieldIndexes)]
	var preIdx int
	for i, idx := range r.fieldIndexes {
		dst[i] = str[preIdx:idx]
		preIdx = idx
	}

	// el control (o la fijacion) de cuantos campos por registro
	if r.FieldsPerRecord > 0 {
		if len(dst) != r.FieldsPerRecord && err == nil {
			err = &ParseError{StartLine: recLine, Line: recLine, Column: 1, Err: ErrFieldCount}
		}
	} else if r.FieldsPerRecord == 0 {
		r.FieldsPerRecord = len(dst)
	}
	return dst, err
}

// indexNoEspacio: el primer byte que NO es espacio (ASCII), o -1 (ver el punto 1 de la cabecera).
func indexNoEspacio(b []byte) int {
	for i := 0; i < len(b); i++ {
		if !esEspacioASCII(b[i]) {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------- Writer

// A Writer escribe registros en formato CSV.
type Writer struct {
	Comma   rune // separador de campos (la coma por defecto)
	UseCRLF bool // true para terminar cada linea con \r\n
	w       *bufio.Writer
}

// NewWriter devuelve un Writer que escribe en w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{Comma: ',', w: bufio.NewWriter(w)}
}

// writeRune escribe un rune codificado en UTF-8 (bufio.Writer de la stdlib minima no tiene WriteRune).
func (w *Writer) writeRune(r rune) error {
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], r)
	if n == 0 {
		return errors.New("csv: rune invalido en el separador")
	}
	_, err := w.w.Write(buf[:n])
	return err
}

// Write escribe un registro, entrecomillando lo que haga falta. La salida va con buffer: hay que
// llamar a Flush.
func (w *Writer) Write(record []string) error {
	if !validDelim(w.Comma) {
		return errInvalidDelim
	}

	for n, field := range record {
		if n > 0 {
			if err := w.writeRune(w.Comma); err != nil {
				return err
			}
		}
		if !w.fieldNeedsQuotes(field) {
			if _, err := w.w.WriteString(field); err != nil {
				return err
			}
			continue
		}
		if err := w.w.WriteByte('"'); err != nil {
			return err
		}
		for len(field) > 0 {
			// los caracteres especiales
			i := strings.IndexAny(field, "\"\r\n")
			if i < 0 {
				i = len(field)
			}
			if _, err := w.w.WriteString(field[:i]); err != nil {
				return err
			}
			field = field[i:]
			if len(field) > 0 {
				var err error
				switch field[0] {
				case '"':
					_, err = w.w.WriteString("\"\"")
				case '\r':
					if !w.UseCRLF {
						err = w.w.WriteByte('\r')
					}
				case '\n':
					if w.UseCRLF {
						_, err = w.w.WriteString("\r\n")
					} else {
						err = w.w.WriteByte('\n')
					}
				}
				field = field[1:]
				if err != nil {
					return err
				}
			}
		}
		if err := w.w.WriteByte('"'); err != nil {
			return err
		}
	}
	var err error
	if w.UseCRLF {
		_, err = w.w.WriteString("\r\n")
	} else {
		err = w.w.WriteByte('\n')
	}
	return err
}

// Flush manda a la salida lo que quedo en el buffer.
func (w *Writer) Flush() { w.w.Flush() }

// Error: el primer error que haya pasado en Write o Flush.
func (w *Writer) Error() error {
	_, err := w.w.Write(nil)
	return err
}

// WriteAll escribe varios registros y despues vacia el buffer.
func (w *Writer) WriteAll(records [][]string) error {
	for _, record := range records {
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return w.w.Flush()
}

// fieldNeedsQuotes: hace falta entrecomillar si el campo trae el separador, una comilla o un salto, o
// si arranca con un espacio (ver el punto 2 de la cabecera).
func (w *Writer) fieldNeedsQuotes(field string) bool {
	if field == "" {
		return false
	}
	if field == `\.` {
		return true
	}
	if w.Comma < utf8.RuneSelf {
		for i := 0; i < len(field); i++ {
			c := field[i]
			if c == '\n' || c == '\r' || c == '"' || c == byte(w.Comma) {
				return true
			}
		}
	} else {
		if strings.ContainsRune(field, w.Comma) || strings.ContainsAny(field, "\"\r\n") {
			return true
		}
	}
	r1, _ := utf8.DecodeRuneInString(field)
	return r1 < utf8.RuneSelf && esEspacioASCII(byte(r1))
}
