// Package strings: el subconjunto que usa el motor (TrimSpace 230, ToLower 192, Split 103,
// EqualFold 94, Join 73, HasPrefix 65, Contains 61, ReplaceAll 61 ...) mas Builder, que el motor
// usa mucho para armar rutas.
//
// LIMITE DOCUMENTADO: el manejo de mayusculas y de "espacio" es ASCII (el motor trabaja con
// rutas y nombres de archivo del juego, todos ASCII). Con texto no ASCII, ToLower/ToUpper
// devuelven el caracter tal cual y TrimSpace no lo reconoce como espacio. La prueba diferencial
// del host cubre el dominio ASCII y deja constancia del limite.
package strings

import (
	"d2std/io"
	"d2std/unicode/utf8"
)

// ---- consultas ----

// Contains informa si substr esta en s.
func Contains(s, substr string) bool { return Index(s, substr) >= 0 }

// ContainsAny informa si algun caracter de chars esta en s.
func ContainsAny(s, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}

// ContainsRune informa si el caracter r esta en s.
func ContainsRune(s string, r rune) bool { return IndexRune(s, r) >= 0 }

// Count cuenta las apariciones (no solapadas) de sep en s. Con sep vacio, el numero de caracteres + 1.
func Count(s, sep string) int {
	if sep == "" {
		return utf8.RuneCountInString(s) + 1
	}
	n := 0
	for {
		i := Index(s, sep)
		if i < 0 {
			return n
		}
		n++
		s = s[i+len(sep):]
	}
}

// Index devuelve el indice de la primera aparicion de substr, o -1.
func Index(s, substr string) int {
	n := len(substr)
	switch {
	case n == 0:
		return 0
	case n > len(s):
		return -1
	case n == 1:
		return IndexByte(s, substr[0])
	}
	c0 := substr[0]
	c1 := substr[1]
	for i := 0; i+n <= len(s); i++ {
		if s[i] != c0 {
			continue
		}
		if s[i+1] == c1 && s[i:i+n] == substr {
			return i
		}
	}
	return -1
}

// IndexByte devuelve el indice de la primera aparicion de c, o -1.
func IndexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// IndexRune devuelve el indice de la primera aparicion de r, o -1.
func IndexRune(s string, r rune) int {
	if r < utf8.RuneSelf {
		return IndexByte(s, byte(r))
	}
	for i, c := range s {
		if c == r {
			return i
		}
	}
	return -1
}

// LastIndex devuelve el indice de la ultima aparicion de substr, o -1.
func LastIndex(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return len(s)
	}
	if n > len(s) {
		return -1
	}
	for i := len(s) - n; i >= 0; i-- {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}

// LastIndexByte devuelve el indice de la ultima aparicion de c, o -1.
func LastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// HasPrefix informa si s empieza con prefix.
func HasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// HasSuffix informa si s termina con suffix.
func HasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// ---- mayusculas (ASCII) ----

// ToLower pasa las letras ASCII a minuscula.
func ToLower(s string) string { return cambiarCaso(s, 'A', 'Z', 'a'-'A') }

// ToUpper pasa las letras ASCII a mayuscula.
func ToUpper(s string) string { return cambiarCaso(s, 'a', 'z', -('a' - 'A')) }

// cambiarCaso hace el cambio de mayusculas ASCII del rango [desde, hasta] sumando delta.
//
// Con texto no ASCII no hay tabla de mayusculas Unicode (no entra en la consola), pero SI se
// iguala a Go en un detalle que importa: los bytes que no son UTF-8 valido se reemplazan por el
// caracter de reemplazo, porque Go tambien lo hace al recorrer el texto por caracteres.
func cambiarCaso(s string, desde, hasta byte, delta int8) string {
	hayAlto := false
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			hayAlto = true
			break
		}
	}
	if !hayAlto {
		hay := false
		for i := 0; i < len(s); i++ {
			if desde <= s[i] && s[i] <= hasta {
				hay = true
				break
			}
		}
		if !hay {
			return s
		}
		b := []byte(s)
		for i := 0; i < len(b); i++ {
			if desde <= b[i] && b[i] <= hasta {
				b[i] += byte(delta)
			}
		}
		return string(b)
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b = utf8.AppendRune(b, utf8.RuneError)
			i++
			continue
		}
		if desde <= byte(r) && byte(r) <= hasta && r < utf8.RuneSelf {
			r += rune(delta)
		}
		b = utf8.AppendRune(b, r)
		i += size
	}
	return string(b)
}

// EqualFold compara sin distinguir mayusculas (ASCII).
func EqualFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c, d := s[i], t[i]
		if c == d {
			continue
		}
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if 'A' <= d && d <= 'Z' {
			d += 'a' - 'A'
		}
		if c != d {
			return false
		}
	}
	return true
}

// ---- corte y union ----

// Split corta s por sep. Con sep vacio corta caracter por caracter.
func Split(s, sep string) []string { return genSplit(s, sep, 0, -1) }

// SplitN corta s en a lo sumo n partes (n < 0: todas).
func SplitN(s, sep string, n int) []string { return genSplit(s, sep, 0, n) }

// SplitAfter corta s por sep conservando el separador al final de cada parte.
func SplitAfter(s, sep string) []string { return genSplit(s, sep, len(sep), -1) }

func genSplit(s, sep string, sepSave, n int) []string {
	if n == 0 {
		return nil
	}
	if sep == "" {
		return explode(s, n)
	}
	if n < 0 {
		n = Count(s, sep) + 1
	}
	a := make([]string, 0, n)
	i := 0
	for k := 1; k < n; k++ {
		j := Index(s, sep)
		if j < 0 {
			break
		}
		a = append(a, s[:j+sepSave])
		s = s[j+len(sep):]
		i++
	}
	return append(a, s)
}

func explode(s string, n int) []string {
	l := utf8.RuneCountInString(s)
	if n < 0 || n > l {
		n = l
	}
	a := make([]string, 0, n)
	for _, r := range s {
		if n <= 0 {
			break
		}
		a = append(a, string(r))
		n--
	}
	return a
}

// Fields corta s por espacios (ASCII) descartando los vacios.
func Fields(s string) []string {
	a := make([]string, 0, 16)
	i := 0
	for {
		for i < len(s) && isSpace(s[i]) {
			i++
		}
		if i >= len(s) {
			return a
		}
		j := i
		for j < len(s) && !isSpace(s[j]) {
			j++
		}
		a = append(a, s[i:j])
		i = j
	}
}

// Join une los elementos con sep.
func Join(elems []string, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}
	n := len(sep) * (len(elems) - 1)
	for _, e := range elems {
		n += len(e)
	}
	var b Builder
	b.Grow(n)
	b.WriteString(elems[0])
	for _, e := range elems[1:] {
		b.WriteString(sep)
		b.WriteString(e)
	}
	return b.String()
}

// Repeat repite s n veces.
func Repeat(s string, count int) string {
	if count < 0 {
		panic("strings: negative Repeat count")
	}
	if count == 0 || s == "" {
		return ""
	}
	var b Builder
	b.Grow(len(s) * count)
	for i := 0; i < count; i++ {
		b.WriteString(s)
	}
	return b.String()
}

// Replace cambia las primeras n apariciones (n < 0: todas).
func Replace(s, old, new string, n int) string {
	if old == new || n == 0 {
		return s
	}
	if m := Count(s, old); m == 0 {
		return s
	} else if n < 0 || m < n {
		n = m
	}
	var b Builder
	b.Grow(len(s) + n*(len(new)-len(old)))
	start := 0
	for i := 0; i < n; i++ {
		j := start
		if len(old) == 0 {
			// con texto a buscar vacio se avanza un caracter por vuelta (asi lo hace Go)
			if i > 0 {
				_, wid := utf8.DecodeRuneInString(s[start:])
				j += wid
			}
		} else {
			j += Index(s[start:], old)
		}
		b.WriteString(s[start:j])
		b.WriteString(new)
		start = j + len(old)
	}
	b.WriteString(s[start:])
	return b.String()
}

// ReplaceAll cambia todas las apariciones.
func ReplaceAll(s, old, new string) string { return Replace(s, old, new, -1) }

// Cut corta s en la primera aparicion de sep.
func Cut(s, sep string) (before, after string, found bool) {
	if i := Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

// ---- recorte ----

// IsSpace dice si el byte es espacio ASCII.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

// TrimSpace saca los espacios de los dos extremos (ASCII).
func TrimSpace(s string) string {
	start := 0
	for start < len(s) && isSpace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func trimLeftCutset(s, cutset string) string {
	i := 0
	for i < len(s) && IndexByte(cutset, s[i]) >= 0 {
		i++
	}
	return s[i:]
}

func trimRightCutset(s, cutset string) string {
	i := len(s)
	for i > 0 && IndexByte(cutset, s[i-1]) >= 0 {
		i--
	}
	return s[:i]
}

// Trim saca de los dos extremos los bytes que esten en cutset.
func Trim(s, cutset string) string { return trimRightCutset(trimLeftCutset(s, cutset), cutset) }

// TrimLeft saca del comienzo los bytes que esten en cutset.
func TrimLeft(s, cutset string) string { return trimLeftCutset(s, cutset) }

// TrimRight saca del final los bytes que esten en cutset.
func TrimRight(s, cutset string) string { return trimRightCutset(s, cutset) }

// TrimPrefix saca prefix del comienzo si esta.
func TrimPrefix(s, prefix string) string {
	if HasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

// TrimSuffix saca suffix del final si esta.
func TrimSuffix(s, suffix string) string {
	if HasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// ---- Builder ----

// Builder arma un string sin andar copiando el buffer en cada paso.
type Builder struct {
	buf []byte
}

// String devuelve lo armado.
func (b *Builder) String() string { return string(b.buf) }

// Len devuelve el largo actual.
func (b *Builder) Len() int { return len(b.buf) }

// Cap devuelve la capacidad actual.
func (b *Builder) Cap() int { return cap(b.buf) }

// Reset vacia el builder.
func (b *Builder) Reset() { b.buf = b.buf[:0] }

// Grow reserva espacio para n bytes mas.
func (b *Builder) Grow(n int) {
	if n < 0 {
		panic("strings.Builder.Grow: negative count")
	}
	if cap(b.buf)-len(b.buf) < n {
		buf := make([]byte, len(b.buf), 2*cap(b.buf)+n)
		copy(buf, b.buf)
		b.buf = buf
	}
}

// Write agrega bytes.
func (b *Builder) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// WriteByte agrega un byte.
func (b *Builder) WriteByte(c byte) error {
	b.buf = append(b.buf, c)
	return nil
}

// WriteString agrega un string.
func (b *Builder) WriteString(s string) (int, error) {
	b.buf = append(b.buf, s...)
	return len(s), nil
}

// WriteRune agrega un caracter.
func (b *Builder) WriteRune(r rune) (int, error) {
	n := len(b.buf)
	b.buf = utf8.AppendRune(b.buf, r)
	return len(b.buf) - n, nil
}

// el Builder tambien sirve donde se espera un io.Writer
var _ io.Writer = (*Builder)(nil)
var _ io.StringWriter = (*Builder)(nil)

// ---------------------------------------------------------------- Replacer

// Replacer: el reemplazador de Go, con la misma idea y las mismas reglas (los pares se prueban en el
// orden en que se dieron, y el que gana es el primero que coincide en esa posicion).
//
// Límite declarado: se implementa con una lista de pares y una busqueda por posicion; el de Go usa
// tablas de bytes para ir rapido. El RESULTADO es el mismo (y se prueba contra el de verdad en el
// host), lo que cambia es la velocidad: para los textos del juego (rutas y tablas de textos) alcanza.
type Replacer struct {
	pares []string
}

// NewReplacer arma el reemplazador con los pares viejo, nuevo, viejo, nuevo...
func NewReplacer(oldnew ...string) *Replacer {
	if len(oldnew)%2 != 0 {
		panic("strings.NewReplacer: cantidad impar de argumentos")
	}
	return &Replacer{pares: append([]string(nil), oldnew...)}
}

// Replace devuelve s con todas las sustituciones aplicadas.
func (r *Replacer) Replace(s string) string {
	if len(r.pares) == 0 || s == "" {
		return s
	}
	// el largo del reemplazo mas largo: no hace falta mirar mas alla
	maxLargo := 0
	for i := 0; i < len(r.pares); i += 2 {
		if l := len(r.pares[i]); l > maxLargo {
			maxLargo = l
		}
	}
	var b []byte
	for i := 0; i < len(s); {
		cual := -1
		for p := 0; p < len(r.pares); p += 2 {
			if r.pares[p] == "" {
				continue
			}
			if len(r.pares[p]) > len(s)-i {
				continue
			}
			if s[i:i+len(r.pares[p])] == r.pares[p] {
				cual = p
				break
			}
		}
		if cual >= 0 {
			if b == nil {
				b = []byte(s[:i])
			}
			b = append(b, r.pares[cual+1]...)
			i += len(r.pares[cual])
		} else {
			if b != nil {
				b = append(b, s[i])
			}
			i++
		}
	}
	if b == nil {
		return s
	}
	return string(b)
}

// WriteString escribe el reemplazo en un Writer.
func (r *Replacer) WriteString(w interface{ Write([]byte) (int, error) }, s string) (int, error) {
	return w.Write([]byte(r.Replace(s)))
}

// ---------------------------------------------------------------- Reader

// Reader: la version de strings.Reader de Go (un texto que se lee como si fuera un archivo). La usa
// d2mpq para leer el (listfile) del MPQ como si fuera un archivo.
//
// Límite declarado: estan Read, ReadByte, ReadAt, Seek, Len, Size y Reset; no estan ReadRune ni
// ReadLastRune (el motor no las usa, y en la consola no hay utf8 completo).
type Reader struct {
	s        string
	i        int64
	prevRune int
}

// NewReader devuelve un Reader que lee s.
func NewReader(s string) *Reader { return &Reader{s: s, i: 0, prevRune: -1} }

// Len: lo que queda por leer.
func (r *Reader) Len() int {
	if r.i >= int64(len(r.s)) {
		return 0
	}
	return int(int64(len(r.s)) - r.i)
}

// Size: el largo original (no cambia con la lectura).
func (r *Reader) Size() int64 { return int64(len(r.s)) }

func (r *Reader) Read(b []byte) (n int, err error) {
	if r.i >= int64(len(r.s)) {
		return 0, ioEOF
	}
	n = copy(b, r.s[r.i:])
	r.i += int64(n)
	return n, nil
}

func (r *Reader) ReadByte() (byte, error) {
	if r.i >= int64(len(r.s)) {
		return 0, ioEOF
	}
	b := r.s[r.i]
	r.i++
	return b, nil
}

func (r *Reader) ReadAt(b []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errorsNew("strings.Reader.ReadAt: offset negativo")
	}
	if off >= int64(len(r.s)) {
		return 0, ioEOF
	}
	n = copy(b, r.s[off:])
	if n < len(b) {
		err = ioEOF
	}
	return n, nil
}

func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	abs := r.i
	switch whence {
	case ioSeekStart:
		abs = offset
	case ioSeekCurrent:
		abs += offset
	case ioSeekEnd:
		abs = int64(len(r.s)) + offset
	default:
		return 0, errorsNew("strings.Reader.Seek: whence invalido")
	}
	if abs < 0 {
		return 0, errorsNew("strings.Reader.Seek: posicion negativa")
	}
	r.i = abs
	return abs, nil
}

// Reset vuelve al principio.
func (r *Reader) Reset(s string) {
	r.s = s
	r.i = 0
	r.prevRune = -1
}

// WriteTo: para que el Reader sirva donde se pide un io.WriterTo.
func (r *Reader) WriteTo(w interface{ Write([]byte) (int, error) }) (int64, error) {
	if r.i >= int64(len(r.s)) {
		return 0, nil
	}
	n, err := w.Write([]byte(r.s[r.i:]))
	r.i += int64(n)
	return int64(n), err
}

// las constantes y el error, sin depender de io ni de errors (regla de la casa: los paquetes de aca no
// importan la biblioteca estandar de verdad)
const (
	ioSeekStart   = 0
	ioSeekCurrent = 1
	ioSeekEnd     = 2
)

// ioEOF es el error de fin de archivo, con el mismo texto que el de Go.
type errorString string

func (e errorString) Error() string { return string(e) }

func errorsNew(s string) error { return errorString(s) }

var ioEOF error = errorString("EOF")

// FieldsFunc: parte el texto por los separadores que diga la funcion. La forma es la de Go:
// `strings.FieldsFunc(spec, func(r rune) bool { return r == '+' || r == ' ' })`. Lo pide
// d2game/d2gamescreen/game_ai_also.go:29 (parte una lista de tags).
//
// OJO CON UNA COSA QUE EN GO CAMBIA (y aca se copia igual): los separadores del principio y del final NO
// producen campos vacios, y dos separadores seguidos tampoco. O sea que "a++b" da ["a","b"], no
// ["a","","b"]. Si no fuera asi, el motor veria tags vacios.
func FieldsFunc(s string, separador func(rune) bool) []string {
	// nil quiere decir "el separador de siempre" (los espacios), como en Go
	if separador == nil {
		separador = func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r' ||
				r == 0x85 || r == 0xA0
		}
	}

	var campos []string
	enCampo := false
	inicio := 0
	for i, r := range s {
		if separador(r) {
			if enCampo {
				campos = append(campos, s[inicio:i])
				enCampo = false
			}
			continue
		}
		if !enCampo {
			inicio = i
			enCampo = true
		}
	}
	if enCampo {
		campos = append(campos, s[inicio:])
	}
	return campos
}
