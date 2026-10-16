// Package fmt: el subconjunto que usa el motor (Errorf 632, Sprintf 532, Printf 58 ...).
//
// Los verbos que el motor realmente usa, medidos con scripts/console/audit-stdlib-uso.py sobre
// los textos de formato literales: %d 851, %s 495, %v 213, %w 211, %q 169, %f 34, %X 24, %x 18,
// %% 9, %c 3, %t 3, %e 1, %T 1. Con banderas (- + # 0 espacio) y ancho/precision (%.1f, %#x,
// %04x, %+v, %#v, %016x, %2d).
//
// LIMITE DOCUMENTADO: sin reflect no se puede imprimir un struct, un map o un slice de un tipo
// cualquiera, ni deducir el nombre de un tipo arbitrario. %v y %s funcionan con los tipos que el
// motor usa de verdad (numeros, strings, []byte, error, Stringer y unos pocos slices) porque se
// resuelven con un type switch sobre tipos concretos. Para el resto sale un aviso visible
// ("%!v(tipo no soportado)") en vez de una mentira silenciosa.
//
// La salida de Printf/Println va al destino que diga Output: por defecto usa el println del
// compilador, que en la Dreamcast sale por la consola serie (verificado en los POC).
package fmt

import (
	"d2std/errors"
	"d2std/io"
	"d2std/strconv"
	"d2std/strings"
	"d2std/unicode/utf8"
)

// ---- salida ----

// Output es donde escriben Printf/Println/Fprintf cuando no se les da un destino.
var Output io.Writer = builtinOutput{}

// SetOutput cambia el destino (el arranque de la consola le enchufa el driver de pantalla o
// consola serie).
func SetOutput(w io.Writer) { Output = w }

type builtinOutput struct{}

func (builtinOutput) Write(p []byte) (int, error) {
	print(string(p))
	return len(p), nil
}

// ---- banderas ----

type flags struct {
	minus bool // -
	plus  bool // +
	sharp bool // #
	space bool // ' '
	zero  bool // 0
}

func (fl flags) sign(neg bool) string {
	switch {
	case neg:
		return "-"
	case fl.plus:
		return "+"
	case fl.space:
		return " "
	}
	return ""
}

// ---- formateador ----

type formatter struct {
	b       strings.Builder
	wrapped []error // lo que junta %w (solo tiene sentido en Errorf)
	armado  bool    // el ultimo convert devolvio un aviso ya armado (no re-acomodar)
	enAviso bool    // estamos armando un aviso (evita recursion infinita)
	w       int     // ancho del verbo en curso (lo necesitan los avisos de verbo invalido)
}

func newFormatter() *formatter { return &formatter{} }

// Sprintf arma el texto con el formato dado.
func Sprintf(format string, a ...interface{}) string {
	f := newFormatter()
	f.doFormat(format, a)
	return f.b.String()
}

// Sprint pega los operandos, con espacios entre los que no son texto.
func Sprint(a ...interface{}) string {
	f := newFormatter()
	prev := false
	for i, arg := range a {
		isStr := isStringish(arg)
		if i > 0 && !isStr && !prev {
			f.b.WriteByte(' ')
		}
		f.printArg(arg, 'v', flags{}, -1, -1, true)
		prev = isStr
	}
	return f.b.String()
}

// Sprintln pega los operandos separados por espacio y agrega el salto de linea.
func Sprintln(a ...interface{}) string {
	f := newFormatter()
	for i, arg := range a {
		if i > 0 {
			f.b.WriteByte(' ')
		}
		f.printArg(arg, 'v', flags{}, -1, -1, true)
	}
	f.b.WriteByte('\n')
	return f.b.String()
}

// Errorf arma un error; con %w envuelve el error que se le pasa.
func Errorf(format string, a ...interface{}) error {
	f := newFormatter()
	f.doFormat(format, a)
	msg := f.b.String()
	switch len(f.wrapped) {
	case 0:
		return errors.New(msg)
	case 1:
		return &wrapError{msg: msg, err: f.wrapped[0]}
	}
	return &wrapErrors{msg: msg, errs: f.wrapped}
}

type wrapError struct {
	msg string
	err error
}

func (e *wrapError) Error() string { return e.msg }
func (e *wrapError) Unwrap() error { return e.err }

type wrapErrors struct {
	msg  string
	errs []error
}

func (e *wrapErrors) Error() string   { return e.msg }
func (e *wrapErrors) Unwrap() []error { return e.errs }

// ---- las que escriben ----

// Fprintf escribe en w.
func Fprintf(w io.Writer, format string, a ...interface{}) (int, error) {
	s := Sprintf(format, a...)
	return io.WriteString(w, s)
}

// Fprint escribe los operandos en w.
func Fprint(w io.Writer, a ...interface{}) (int, error) {
	return io.WriteString(w, Sprint(a...))
}

// Fprintln escribe los operandos y el salto en w.
func Fprintln(w io.Writer, a ...interface{}) (int, error) {
	return io.WriteString(w, Sprintln(a...))
}

// Printf escribe en Output.
func Printf(format string, a ...interface{}) (int, error) { return Fprintf(Output, format, a...) }

// Print escribe los operandos en Output.
func Print(a ...interface{}) (int, error) { return Fprint(Output, a...) }

// Println escribe los operandos y el salto en Output.
func Println(a ...interface{}) (int, error) { return Fprintln(Output, a...) }

// Sprintln... (ya esta arriba)

// ---- el motor de formato ----

func (f *formatter) doFormat(format string, a []interface{}) {
	argNum := 0
	for i := 0; i < len(format); {
		c := format[i]
		if c != '%' {
			f.b.WriteByte(c)
			i++
			continue
		}
		i++
		if i >= len(format) {
			f.b.WriteString("%!(NOVERB)")
			return
		}
		if format[i] == '%' {
			f.b.WriteByte('%')
			i++
			continue
		}

		var fl flags
		for i < len(format) {
			switch format[i] {
			case '-':
				fl.minus = true
			case '+':
				fl.plus = true
			case '#':
				fl.sharp = true
			case ' ':
				fl.space = true
			case '0':
				fl.zero = true
			default:
				goto ancho
			}
			i++
		}

	ancho:
		// indice de argumento explicito: %[2]d
		explicit := 0
		if i < len(format) && format[i] == '[' {
			j := i + 1
			n := 0
			for j < len(format) && '0' <= format[j] && format[j] <= '9' {
				n = n*10 + int(format[j]-'0')
				j++
			}
			if j < len(format) && format[j] == ']' {
				explicit = n
				i = j + 1
			}
		}

		width := -1
		if i < len(format) && format[i] == '*' {
			width = f.nextIntArg(a, &argNum)
			i++
		} else {
			for i < len(format) && '0' <= format[i] && format[i] <= '9' {
				if width < 0 {
					width = 0
				}
				width = width*10 + int(format[i]-'0')
				i++
			}
		}

		prec := -1
		if i < len(format) && format[i] == '.' {
			i++
			prec = 0
			if i < len(format) && format[i] == '*' {
				prec = f.nextIntArg(a, &argNum)
				i++
			} else {
				for i < len(format) && '0' <= format[i] && format[i] <= '9' {
					prec = prec*10 + int(format[i]-'0')
					i++
				}
			}
		}

		if i >= len(format) {
			f.b.WriteString("%!(NOVERB)")
			return
		}
		verb := format[i]
		i++

		var arg interface{}
		has := false
		if explicit > 0 {
			if explicit <= len(a) {
				arg, has = a[explicit-1], true
			}
			if explicit > argNum {
				argNum = explicit
			}
		} else if argNum < len(a) {
			arg, has = a[argNum], true
			argNum++
		}
		f.printArg(arg, verb, fl, width, prec, has)
	}
}

func (f *formatter) nextIntArg(a []interface{}, argNum *int) int {
	if *argNum < len(a) {
		v, ok := toInt64(a[*argNum])
		*argNum++
		if ok {
			return int(v)
		}
	}
	return -1
}

// printArg formatea un argumento con su verbo.
func (f *formatter) printArg(arg interface{}, verb byte, fl flags, width, prec int, has bool) {
	if !has {
		f.b.WriteString("%!")
		f.b.WriteByte(verb)
		f.b.WriteString("(MISSING)")
		return
	}

	// verbos que consumen el error envuelto de Errorf
	switch verb {
	case 'w':
		if err, ok := arg.(error); ok {
			f.wrapped = append(f.wrapped, err)
			f.write(err.Error(), fl, width, prec, false)
			return
		}
	}

	f.armado = false
	f.w = width
	s, numeric := f.convert(arg, verb, fl, prec)
	if f.armado {
		// los avisos de verbo invalido ya vienen armados (llevan el ancho adentro, como Go)
		f.b.WriteString(s)
		return
	}
	f.write(s, fl, width, prec, numeric)
}

// write aplica ancho y banderas.
func (f *formatter) write(s string, fl flags, width, prec int, numeric bool) {
	// el ancho se cuenta en caracteres, no en bytes (como Go)
	car := utf8.RuneCountInString(s)
	if width <= car {
		f.b.WriteString(s)
		return
	}
	n := width - car
	if fl.minus {
		f.b.WriteString(s)
		f.b.WriteString(strings.Repeat(" ", n))
		return
	}
	if fl.zero {
		// los ceros van despues del signo: -42 con %05d = "-0042" (y tambien rellenan textos)
		sign := ""
		if numeric && len(s) > 0 && (s[0] == '-' || s[0] == '+') {
			sign = s[:1]
			s = s[1:]
		}
		f.b.WriteString(sign)
		f.b.WriteString(strings.Repeat("0", n))
		f.b.WriteString(s)
		return
	}
	f.b.WriteString(strings.Repeat(" ", n))
	f.b.WriteString(s)
}

// badVerb arma el aviso de verbo invalido como Go: %!verbo(tipo=valor), con el valor impreso
// como %v y las banderas/ancho/precision vigentes (asi lo hace Go: el ancho se le aplica al
// valor de adentro, no al aviso).
func (f *formatter) badVerb(verb byte, arg interface{}, fl flags, width, prec int) string {
	f.armado = true
	if f.enAviso {
		return "%!" + string(verb) + "(" + typeName(arg) + "=" + valueString(arg) + ")"
	}
	f.enAviso = true
	val, _ := f.convert(arg, 'v', fl, prec)
	f.enAviso = false
	if val == "" {
		val = valueString(arg)
	}
	car := utf8.RuneCountInString(val)
	if width > car {
		relleno := strings.Repeat(" ", width-car)
		if fl.minus {
			val = val + relleno
		} else {
			val = relleno + val
		}
	}
	return "%!" + string(verb) + "(" + typeName(arg) + "=" + val + ")"
}

// formatIntBase: entero en la base dada, con signo, prefijo (#) y precision = minimo de digitos.
func formatIntBase(arg interface{}, base int, upper bool, fl flags, prec int) (string, bool) {
	var u uint64
	neg := false
	if isUnsigned(arg) {
		u, _ = toUint64(arg)
	} else {
		n, ok := toInt64(arg)
		if !ok {
			return "", false
		}
		if n < 0 {
			neg = true
			u = uint64(-n)
		} else {
			u = uint64(n)
		}
	}
	prefix := ""
	if fl.sharp {
		switch base {
		case 2:
			prefix = "0b"
		case 16:
			if upper {
				prefix = "0X"
			} else {
				prefix = "0x"
			}
		}
	}
	digits := formatBase(u, base, upper)
	if prec == 0 && u == 0 {
		digits = "" // Go: con precision 0 el valor 0 no imprime ningun digito
	}
	if base == 8 && fl.sharp && u != 0 {
		digits = "0" + digits // el prefijo octal cuenta dentro de la precision
	}
	if prec > len(digits) {
		digits = strings.Repeat("0", prec-len(digits)) + digits
	}
	sign := ""
	switch {
	case neg:
		sign = "-"
	case fl.plus:
		sign = "+"
	case fl.space:
		sign = " "
	}
	return sign + prefix + digits, true
}

// quoteTexto aplica las dos banderas que cambian el %q: # (comillas invertidas si se puede) y
// + (salida solo ASCII).
func (f *formatter) quoteTexto(s string, fl flags) string {
	if fl.sharp {
		return quoteSharp(s)
	}
	if fl.plus {
		return strconv.QuoteToASCII(s)
	}
	return strconv.Quote(s)
}

// quoteSharp: %#q usa comillas invertidas cuando el texto lo permite (como Go).
func quoteSharp(s string) string {
	ok := true
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		// Go (strconv.CanBackquote) acepta el tab dentro de comillas invertidas y rechaza el
		// resto de los caracteres de control, el BOM y los bytes que no son UTF-8 valido
		if (r == utf8.RuneError && size == 1) || r == '`' || r == '\n' || r == '\ufeff' ||
			!strconv.IsPrint(r) && r != '	' {
			ok = false
			break
		}
		i += size
	}
	if ok {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

// valueString: el valor como lo imprimiria %v (para el aviso de verbo invalido).
func valueString(arg interface{}) string {
	switch x := arg.(type) {
	case nil:
		return "<nil>"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case error:
		return x.Error()
	case Stringer:
		return x.String()
	case []byte:
		return byteList(x)
	}
	if isUnsigned(arg) {
		if u, ok := toUint64(arg); ok {
			return strconv.FormatUint(u, 10)
		}
	}
	if n, ok := toInt64(arg); ok {
		return strconv.FormatInt(n, 10)
	}
	if f, ok := toFloat64(arg); ok {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return "tipo no soportado"
}

func isUnsigned(arg interface{}) bool {
	switch arg.(type) {
	case uint, uint8, uint16, uint32, uint64:
		return true
	}
	return false
}

// byteListFormateada imprime un []byte con %v aplicando las banderas, el ancho y la precision a
// cada elemento, que es lo que hace Go ("[   45    45]" con %5v).
func byteListFormateada(b []byte, fl flags, width, prec int) string {
	if len(b) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, c := range b {
		if i > 0 {
			sb.WriteByte(' ')
		}
		s, _ := formatIntBase(int64(c), 10, false, fl, prec)
		car := utf8.RuneCountInString(s)
		if width > car {
			relleno := strings.Repeat(" ", width-car)
			if fl.zero {
				relleno = strings.Repeat("0", width-car)
			}
			if fl.minus {
				s = s + relleno
			} else {
				s = relleno + s
			}
		}
		sb.WriteString(s)
	}
	sb.WriteByte(']')
	return sb.String()
}

func byteList(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, c := range b {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.Itoa(int(c)))
	}
	sb.WriteByte(']')
	return sb.String()
}

// listaDeElementos arma un arreglo elemento por elemento, que es lo que hace Go cuando el verbo no
// trata al arreglo como un todo: Sprintf("%t", []byte("abc")) da
// "[%!t(uint8=97) %!t(uint8=98) %!t(uint8=99)]", no el aviso del arreglo entero. Medido en la
// consola (sonda DC-002.2e, E15-E17), donde dabamos "%!t([]uint8=[97 98 99])".
//
// Solo []byte y []string: son los arreglos que el motor usa y los unicos que se pueden recorrer sin
// reflect. Los verbos que tratan al []byte como texto o como bytes crudos (%v, %s, %q, %x, %X) no
// pasan por aca: ya tienen su camino en convert.
func (f *formatter) listaDeElementos(arg interface{}, verb byte, fl flags, width, prec int) (string, bool) {
	var elems []interface{}
	switch x := arg.(type) {
	case []byte:
		switch verb {
		case 'v', 's', 'q', 'x', 'X':
			return "", false
		}
		elems = make([]interface{}, len(x))
		for i := range x {
			elems[i] = x[i]
		}
	case []string:
		switch verb {
		case 'v', 's', 'q', 'x', 'X':
			return "", false
		}
		elems = make([]interface{}, len(x))
		for i := range x {
			elems[i] = x[i]
		}
	default:
		return "", false
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, e := range elems {
		if i > 0 {
			sb.WriteByte(' ')
		}
		// cada elemento con su propio formateador: asi el ancho, las banderas y la precision caen
		// sobre el elemento y no sobre el arreglo, y los avisos de verbo invalido salen por dentro
		// ("[%!t(uint8=   97)]" con ancho 5, como Go).
		sub := newFormatter()
		sub.printArg(e, verb, fl, width, prec, true)
		sb.WriteString(sub.b.String())
	}
	sb.WriteByte(']')
	return sb.String(), true
}

// precBytes: en %x/%X la precision cuenta bytes mostrados, no caracteres del texto hex.
func precBytes(b []byte, prec int) []byte {
	if prec >= 0 && prec < len(b) {
		return b[:prec]
	}
	return b
}

// truncar aplica la precision a un texto, contando caracteres (como Go).
func truncar(s string, prec int) string {
	if prec < 0 || utf8.RuneCountInString(s) <= prec {
		return s
	}
	i := 0
	for n := 0; n < prec && i < len(s); n++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s[:i]
}

// asFloat: solo flotantes de verdad (%f sobre un int es un error en Go, no una conversion).
func asFloat(arg interface{}) (float64, bool) {
	switch v := arg.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	}
	return 0, false
}

// typeName: sin reflect, los tipos que se conocen por type switch; el resto no tiene nombre.
func typeName(arg interface{}) string {
	switch arg.(type) {
	case nil:
		return "<nil>"
	case bool:
		return "bool"
	case int:
		return "int"
	case int8:
		return "int8"
	case int16:
		return "int16"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case uint:
		return "uint"
	case uint8:
		return "uint8"
	case uint16:
		return "uint16"
	case uint32:
		return "uint32"
	case uint64:
		return "uint64"
	case float32:
		return "float32"
	case float64:
		return "float64"
	case string:
		return "string"
	case []byte:
		return "[]uint8"
	case []string:
		return "[]string"
	case error:
		return "error"
	case Stringer:
		return "fmt.Stringer"
	}
	return "tipo no soportado"
}

// Stringer es lo que implementa String() (equivalente al de Go).
type Stringer interface {
	String() string
}

func isStringish(arg interface{}) bool {
	switch arg.(type) {
	case string, []byte:
		return true
	}
	return false
}

func toInt64(arg interface{}) (int64, bool) {
	switch v := arg.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return int64(v), true
	}
	return 0, false
}

func toUint64(arg interface{}) (uint64, bool) {
	switch v := arg.(type) {
	case int:
		return uint64(v), true
	case int8:
		return uint64(v), true
	case int16:
		return uint64(v), true
	case int32:
		return uint64(v), true
	case int64:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	}
	return 0, false
}

func toFloat64(arg interface{}) (float64, bool) {
	switch v := arg.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	}
	return 0, false
}

// convert pasa el argumento a texto segun el verbo. numeric dice si el ancho con ceros aplica.
func (f *formatter) convert(arg interface{}, verb byte, fl flags, prec int) (string, bool) {
	// primero Stringer y error, que es lo que Go hace con %v y %s
	switch verb {
	case 'v', 's':
		if fl.sharp && verb == 'v' {
			if s, ok := arg.(string); ok {
				return strconv.Quote(truncar(s, prec)), false // %#v de un texto
			}
			// %#v de un struct/map/slice necesita reflect: se imprime como %v y el limite esta
			// documentado (la prueba diferencial no lo exige)
			fl.sharp = false
		}
		switch x := arg.(type) {
		case nil:
			if verb == 'v' {
				return "<nil>", false
			}
			return f.badVerb(verb, arg, fl, f.w, prec), false
		case string:
			return truncar(x, prec), false
		case error:
			return truncar(x.Error(), prec), false
		case Stringer:
			return truncar(x.String(), prec), false
		case []byte:
			if verb == 'v' {
				if len(x) == 0 {
					f.armado = true // Go no rellena el slice vacio: imprime "[]" a secas
					return "[]", false
				}
				return byteListFormateada(x, fl, f.w, prec), false
			}
			return truncar(string(x), prec), false
		case bool:
			if verb == 's' {
				return f.badVerb(verb, arg, fl, f.w, prec), false
			}
			return strconv.FormatBool(x), false
		case float32:
			return strconv.FormatFloat(float64(x), 'g', -1, 32), true
		case float64:
			return strconv.FormatFloat(x, 'g', -1, 64), true
		}
		if verb == 's' {
			return f.badVerb(verb, arg, fl, f.w, prec), false
		}
		if s, ok := formatIntBase(arg, 10, false, fl, prec); ok {
			return s, true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	}

	// un arreglo con un verbo que no lo trata como texto/binario (los de %v, %s, %q, %x y %X ya
	// volvieron arriba) se arma elemento por elemento, como en Go
	if s, ok := f.listaDeElementos(arg, verb, fl, f.w, prec); ok {
		f.armado = true // el ancho ya se aplico a cada elemento
		return s, false
	}

	switch verb {
	case 'd':
		if s, ok := formatIntBase(arg, 10, false, fl, prec); ok {
			return s, true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'c':
		if n, ok := toInt64(arg); ok {
			// Go mira el valor SIN SIGNO y, si se sale del rango de un caracter valido, imprime
			// U+FFFD. Sin ese control, un int64 negativo (o un uint64 enorme) se truncaba a 32 bits
			// al convertirlo a rune y salia un byte cualquiera: en la consola %c de
			// int64(-9223372036854775808) daba "\x00" en vez de "\uFFFD" (sonda DC-002.2e, E19).
			r := rune(n)
			if uint64(n) > utf8.MaxRune {
				r = utf8.RuneError
			}
			return string(r), false
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'b':
		if s, ok := formatIntBase(arg, 2, false, fl, prec); ok {
			return s, true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'o':
		if s, ok := formatIntBase(arg, 8, false, fl, prec); ok {
			return s, true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'x', 'X':
		upper := verb == 'X'
		if s, ok := arg.(string); ok {
			b := precBytes([]byte(s), prec)
			if len(b) == 0 {
				return "", false // Go no pone el prefijo si no muestra ningun byte
			}
			return hexString(b, upper, fl.sharp, fl.space), false
		}
		if b, ok := arg.([]byte); ok {
			b = precBytes(b, prec)
			if len(b) == 0 {
				return "", false
			}
			return hexString(b, upper, fl.sharp, fl.space), false
		}
		if s, ok := formatIntBase(arg, 16, upper, fl, prec); ok {
			return s, true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 't':
		if b, ok := arg.(bool); ok {
			return strconv.FormatBool(b), false
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'q':
		switch x := arg.(type) {
		case string:
			return f.quoteTexto(truncar(x, prec), fl), false
		case []byte:
			return f.quoteTexto(truncar(string(x), prec), fl), false
		case error:
			return f.quoteTexto(truncar(x.Error(), prec), fl), false
		case Stringer:
			return f.quoteTexto(truncar(x.String(), prec), fl), false
		}
		if u, ok := toUint64(arg); ok {
			r := rune(u) // trunca a 32 bits, como Go
			if u > 0x10FFFF {
				r = 0xFFFD // es lo que muestra Go cuando el valor no es un caracter valido
			}
			if fl.plus {
				return strconv.QuoteRuneToASCII(r), false
			}
			return strconv.QuoteRune(r), false
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'f', 'F':
		if v, ok := asFloat(arg); ok {
			p := prec
			if p < 0 {
				p = 6
			}
			s := strconv.FormatFloat(v, 'f', p, 64)
			return signPrefix(s, fl), true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'e', 'E':
		if v, ok := asFloat(arg); ok {
			p := prec
			if p < 0 {
				p = 6
			}
			s := strconv.FormatFloat(v, verb, p, 64)
			return signPrefix(s, fl), true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'g', 'G':
		if v, ok := asFloat(arg); ok {
			s := strconv.FormatFloat(v, verb, prec, 64)
			return signPrefix(s, fl), true
		}
		return f.badVerb(verb, arg, fl, f.w, prec), false
	case 'T':
		return typeName(arg), false
	}
	return f.badVerb(verb, arg, fl, f.w, prec), false
}

// signPrefix agrega el signo que piden las banderas + o espacio.
func signPrefix(s string, fl flags) string {
	if len(s) > 0 && s[0] == '-' {
		return s
	}
	if fl.plus {
		return "+" + s
	}
	if fl.space {
		return " " + s
	}
	return s
}

func formatBase(u uint64, base int, upper bool) string {
	s := strconv.FormatUint(u, base)
	if upper {
		return strings.ToUpper(s)
	}
	return s
}

func hexString(b []byte, upper, sharp, sep bool) string {
	const lower = "0123456789abcdef"
	const upperHex = "0123456789ABCDEF"
	tab := lower
	if upper {
		tab = upperHex
	}
	var sb strings.Builder
	sb.Grow(len(b)*3 + 2)
	if sharp {
		if upper {
			sb.WriteString("0X")
		} else {
			sb.WriteString("0x")
		}
	}
	for i, c := range b {
		if sep && i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteByte(tab[c>>4])
		sb.WriteByte(tab[c&0xF])
	}
	return sb.String()
}
