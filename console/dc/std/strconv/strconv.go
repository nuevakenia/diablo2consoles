// Package strconv: el subconjunto que usa el motor (Atoi 173, Itoa 141, ParseFloat 35 ...).
//
// El manejo de enteros es exacto igual que en Go, incluidos los errores (NumError con ErrSyntax
// y ErrRange) y el valor recortado al rango.
//
// Los flotantes van con la cuenta hecha en enteros, para que el redondeo sea el correcto y no el
// que sale de multiplicar floats:
//
//  f = m * 2^e  (m < 2^53, exacto)
//  f * 10^p = m * 5^p * 2^(e+p)   ->  entero exacto mientras no desborde uint64
//
// Eso da el mismo resultado que Go para precisiones 0..4 en el rango que usa el motor
// (porcentajes y multiplicadores: %.0f, %.1f, %.2f, %.3f). FUERA de ese rango (precisones
// grandes, numeros enormes o minusculos) se cae a un camino aproximado: esta documentado y la
// prueba diferencial marca donde termina el dominio bueno.
package strconv

import (
	"d2std/errors"
	"d2std/strings"
	"d2std/unicode/utf8"
	"unsafe"
)

// ---- errores, iguales que en Go ----

var (
	ErrRange  = errors.New("value out of range")
	ErrSyntax = errors.New("invalid syntax")
)

// NumError cuenta que fallo y con que texto, igual que en Go.
type NumError struct {
	Func string
	Num  string
	Err  error
}

func (e *NumError) Error() string {
	return "strconv." + e.Func + ": " + "parsing " + Quote(e.Num) + ": " + e.Err.Error()
}

func (e *NumError) Unwrap() error { return e.Err }

func syntaxError(fn, str string) *NumError { return &NumError{fn, str, ErrSyntax} }
func rangeError(fn, str string) *NumError  { return &NumError{fn, str, ErrRange} }

// ---- enteros ----

const intSize = 32 << (^uint(0) >> 63)

const (
	maxInt  = 1<<(intSize-1) - 1
	minInt  = -1 << (intSize - 1)
	maxUint = 1<<intSize - 1
)

// FormatInt pasa i a texto en la base dada (2 a 36).
func FormatInt(i int64, base int) string {
	if base < 2 || base > 36 {
		panic("strconv: illegal AppendInt/FormatInt base")
	}
	neg := i < 0
	var u uint64
	if neg {
		u = uint64(-i)
		if i == -1<<63 {
			u = 1 << 63
		}
	} else {
		u = uint64(i)
	}
	return formatUint(u, base, neg)
}

// FormatUint pasa u a texto en la base dada.
func FormatUint(u uint64, base int) string {
	return formatUint(u, base, false)
}

func formatUint(u uint64, base int, neg bool) string {
	var buf [64]byte
	i := len(buf)
	if u == 0 {
		i--
		buf[i] = '0'
	}
	for u > 0 {
		i--
		d := u % uint64(base)
		u /= uint64(base)
		if d < 10 {
			buf[i] = byte('0' + d)
		} else {
			buf[i] = byte('a' + d - 10)
		}
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Itoa es FormatInt(i, 10).
func Itoa(i int) string { return FormatInt(int64(i), 10) }

// ParseInt lee un entero en la base dada. bitSize 0 = int.
func ParseInt(s string, base int, bitSize int) (int64, error) {
	const fn = "ParseInt"
	neg, baseEf, digits, err := parseIntPrefix(s, base, fn)
	if err != nil {
		return 0, err
	}
	// bitSize 0 significa "el ancho de int de ESTA plataforma" (64 bits en el host, 32 en el SH-4:
	// es la misma cuenta que hace Atoi). Antes el tope se calculaba siempre a 64 bits y al desbordar
	// se devolvia maxInt64/minInt64: en la consola, un int de 32 bits, esos topes truncados daban -1
	// y 0 en vez de MaxInt32 y MinInt32 (medido: sonda DC-002.2e, E19-E23).
	if bitSize == 0 {
		bitSize = intSize
	}
	lim := maxForBitSize(neg, bitSize)
	var n uint64
	for i := 0; i < len(digits); i++ {
		v, ok := digitValue(digits[i])
		if !ok || v >= uint64(baseEf) {
			return 0, syntaxError(fn, s)
		}
		if n > (lim-v)/uint64(baseEf) {
			// al desbordar, Go satura al tope DEL ANCHO pedido (con su signo), no al de 64 bits
			return maxInt64For(neg, bitSize), rangeError(fn, s)
		}
		n = n*uint64(baseEf) + v
	}
	if neg {
		return -int64(n), nil
	}
	return int64(n), nil
}

func digitValue(c byte) (uint64, bool) {
	switch {
	case '0' <= c && c <= '9':
		return uint64(c - '0'), true
	case 'a' <= c && c <= 'z':
		return uint64(c-'a') + 10, true
	case 'A' <= c && c <= 'Z':
		return uint64(c-'A') + 10, true
	}
	return 0, false
}

const (
	maxInt64 = 1<<63 - 1
	minInt64 = -1 << 63
)

func maxForBitSize(neg bool, bitSize int) uint64 {
	if neg {
		return 1 << uint(bitSize-1)
	}
	return 1<<uint(bitSize-1) - 1
}

func maxInt64For(neg bool, bitSize int) int64 {
	if bitSize == 0 || bitSize == 64 {
		if neg {
			return minInt64
		}
		return maxInt64
	}
	if neg {
		return -1 << uint(bitSize-1)
	}
	return 1<<uint(bitSize-1) - 1
}

// ParseUint lee un entero sin signo.
func ParseUint(s string, base int, bitSize int) (uint64, error) {
	const fn = "ParseUint"
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		return 0, syntaxError(fn, s)
	}
	_, baseEf, digits, err := parseIntPrefix(s, base, fn)
	if err != nil {
		return 0, err
	}
	lim := ^uint64(0)
	if bitSize == 0 {
		bitSize = intSize // bitSize 0 = uint de esta plataforma (lo mismo que en ParseInt)
	}
	if bitSize < 64 {
		lim = 1<<uint(bitSize) - 1
	}
	var n uint64
	for i := 0; i < len(digits); i++ {
		v, ok := digitValue(digits[i])
		if !ok || v >= uint64(baseEf) {
			return 0, syntaxError(fn, s)
		}
		if n > (lim-v)/uint64(baseEf) {
			return lim, rangeError(fn, s)
		}
		n = n*uint64(baseEf) + v
	}
	return n, nil
}

// parseIntPrefix resuelve el signo, la base efectiva (la de la sintaxis de Go cuando base es 0) y
// los guiones bajos. Devuelve la base efectiva porque con base 0 se deduce del prefijo, y los
// errores se arman con el texto ORIGINAL (es lo que muestra Go).
func parseIntPrefix(orig string, base int, fn string) (neg bool, baseEf int, digits string, err error) {
	s := orig
	baseEf = base
	if s == "" {
		return false, base, "", syntaxError(fn, orig)
	}
	switch base {
	case 0:
		switch {
		case len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X'):
			baseEf, s = 16, s[2:]
		case len(s) >= 2 && s[0] == '0' && (s[1] == 'b' || s[1] == 'B'):
			baseEf, s = 2, s[2:]
		case len(s) >= 2 && s[0] == '0' && (s[1] == 'o' || s[1] == 'O'):
			baseEf, s = 8, s[2:]
		case len(s) >= 2 && s[0] == '0':
			baseEf = 8
		default:
			baseEf = 10
		}
	case 2, 8, 10, 16:
	default:
		panic("strconv: illegal ParseInt base")
	}
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "" {
		return false, baseEf, "", syntaxError(fn, orig)
	}
	if base == 0 {
		// los guiones bajos son parte de la sintaxis de los literales de Go: ni al principio, ni
		// al final, ni pegados a otro guion bajo
		clean := make([]byte, 0, len(s))
		for i := 0; i < len(s); i++ {
			if s[i] == '_' {
				if i == 0 || i == len(s)-1 || s[i-1] == '_' || s[i+1] == '_' {
					return false, baseEf, "", syntaxError(fn, orig)
				}
				continue
			}
			clean = append(clean, s[i])
		}
		if len(clean) == 0 {
			return false, baseEf, "", syntaxError(fn, orig)
		}
		s = string(clean)
	} else if strings.IndexByte(s, '_') >= 0 {
		return false, baseEf, "", syntaxError(fn, orig)
	}
	return neg, baseEf, s, nil
}

// Atoi es ParseInt(s, 10, 0) devuelto como int.
func Atoi(s string) (int, error) {
	const fnAtoi = "Atoi"
	v, err := ParseInt(s, 10, 0)
	if err != nil {
		if ne, ok := err.(*NumError); ok {
			ne.Func = fnAtoi
		}
		return int(v), err
	}
	if int64(int(v)) != v {
		return int(v), rangeError(fnAtoi, s)
	}
	return int(v), nil
}

// ---- boleanos ----

// ParseBool lee un boleano como Go.
func ParseBool(s string) (bool, error) {
	switch s {
	case "1", "t", "T", "true", "TRUE", "True":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False":
		return false, nil
	}
	return false, syntaxError("ParseBool", s)
}

// FormatBool pasa un boleano a texto.
func FormatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ---- flotantes: bits ----

func float64bits(f float64) uint64 { return *(*uint64)(unsafe.Pointer(&f)) }

func float64frombits(b uint64) float64 { return *(*float64)(unsafe.Pointer(&b)) }

// descompone f (positivo, normal) en m * 2^e exacto
func decompose(f float64) (m uint64, e int) {
	b := float64bits(f)
	m = b & 0xFFFFFFFFFFFFF // 52 bits de mantisa
	e = int(b>>52&0x7FF) - 1075
	if b>>52&0x7FF != 0 { // normal: el bit implicito
		m |= 1 << 52
	}
	return m, e
}

func isNaN(f float64) bool { return f != f }

func isInf(f float64) bool { return f > 1.7976931348623157e308 || f < -1.7976931348623157e308 }

var pow5tab = [...]uint64{1, 5, 25, 125, 625, 3125, 15625, 78125, 390625, 1953125, 9765625,
	48828125, 244140625, 1220703125, 6103515625, 30517578125, 152587890625}

var pow10tab = [...]float64{1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12,
	1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20, 1e21, 1e22}

// FormatFloat pasa f a texto. fmt es 'f', 'e' o 'g'; prec < 0 = el mas corto que se lea de vuelta
// igual (el "shortest" de Go).
func FormatFloat(f float64, fmt byte, prec, bitSize int) string {
	if isNaN(f) {
		return "NaN"
	}
	if isInf(f) {
		if f < 0 {
			return "-Inf"
		}
		return "+Inf"
	}
	upper := fmt == 'E' || fmt == 'G'
	switch fmt {
	case 'f', 'F':
		if prec < 0 {
			return shortest(f, false)
		}
		return formatFScaled(f, prec)
	case 'e', 'E':
		if prec < 0 {
			prec = 6
		}
		return formatE(f, prec, upper)
	case 'g', 'G':
		if prec < 0 {
			return shortest(f, upper)
		}
		if prec == 0 {
			prec = 1
		}
		return formatG(f, prec, upper)
	}
	return formatFScaled(f, 6)
}

// formatFScaled: notacion 'f' con precision exacta usando enteros.
func formatFScaled(f float64, prec int) string {
	// el signo se mira en el bit, no con f < 0: -0.0 se imprime "-0.00" igual que en Go
	neg := float64bits(f)>>63 == 1
	if neg {
		f = -f
	}
	digits := fixedDigits(f, prec)
	if neg {
		return "-" + digits
	}
	return digits
}

// fixedDigits devuelve f (>= 0) en notacion fija con prec decimales.
func fixedDigits(f float64, prec int) string {
	if prec < 0 {
		prec = 0
	}
	var n uint64
	exacto := false
	// exacto con enteros si se puede: f*10^prec = m * 5^prec * 2^(e+prec)
	if f != 0 && prec < len(pow5tab) {
		m, e := decompose(f)
		p5 := pow5tab[prec]
		if m <= ^uint64(0)/p5 {
			a := m * p5
			sh := e + prec
			if sh >= 0 {
				if a <= ^uint64(0)>>uint(sh) {
					n = a << uint(sh)
					exacto = true
				}
			} else {
				n = roundShift(a, -sh)
				exacto = true
			}
		}
	}
	if !exacto {
		// camino aproximado (fuera del rango exacto): documentado arriba
		scale := pow10f(prec)
		aprox := f * scale
		if aprox >= 1.8446744073709552e19 {
			return bigApprox(f, prec)
		}
		n = uint64(aprox + 0.5)
	}
	// La insercion del punto va escrita aca en el lugar a proposito: cuando vivia en su propia
	// funcion (insertPoint), gccgo 15.1 para sh-elf la compilaba como una llamada pelada a
	// FormatUint y tiraba el resto del cuerpo, asi que en la consola salia "150" en vez de "1.50".
	// Medido en console/dc/evidencia/dc-002.2d-punto-flotantes.txt. Escrito asi, funciona.
	s := FormatUint(n, 10)
	if prec == 0 {
		return s
	}
	if len(s) <= prec {
		s = strings.Repeat("0", prec-len(s)+1) + s
	}
	return s[:len(s)-prec] + "." + s[len(s)-prec:]
}

// allZero dice si el texto no tiene ningun digito distinto de cero.
func allZero(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= '1' && s[i] <= '9' {
			return false
		}
	}
	return true
}

// roundShift divide a entre 2^s redondeando al par mas cercano (como Go).
func roundShift(a uint64, s int) uint64 {
	if s <= 0 {
		return a
	}
	if s >= 64 {
		// a < 2^64 <= 2^s: el cociente es 0, y el resto decide
		if s == 64 {
			return 0
		}
		half := uint64(1) << uint(s-1)
		if a > half {
			return 1
		}
		return 0
	}
	q := a >> uint(s)
	rem := a & (1<<uint(s) - 1)
	half := uint64(1) << uint(s-1)
	if rem > half || (rem == half && q&1 == 1) {
		q++
	}
	return q
}

// insertPoint mete el punto decimal en un entero que ya viene escalado por 10^prec.
func insertPoint(n uint64, prec int) string {
	s := FormatUint(n, 10)
	if prec == 0 {
		return s
	}
	if len(s) <= prec {
		s = strings.Repeat("0", prec-len(s)+1) + s
	}
	return s[:len(s)-prec] + "." + s[len(s)-prec:]
}

func pow10f(n int) float64 {
	if n >= 0 && n < len(pow10tab) {
		return pow10tab[n]
	}
	r := 1.0
	if n >= 0 {
		for i := 0; i < n; i++ {
			r *= 10
		}
		return r
	}
	for i := 0; i < -n; i++ {
		r /= 10
	}
	return r
}

func bigApprox(f float64, prec int) string {
	// numero enorme: se arma la parte entera con el exponente decimal
	e := 0
	x := f
	for x >= 10 {
		x /= 10
		e++
	}
	var b strings.Builder
	for i := 0; i <= e; i++ {
		d := int(x)
		if d > 9 {
			d = 9
		}
		b.WriteByte(byte('0' + d))
		x = (x - float64(d)) * 10
	}
	if prec > 0 {
		b.WriteByte('.')
		for i := 0; i < prec; i++ {
			d := int(x)
			if d > 9 {
				d = 9
			}
			b.WriteByte(byte('0' + d))
			x = (x - float64(d)) * 10
		}
	}
	return b.String()
}

// shortest busca la representacion mas corta que se lea de vuelta igual (lo de Go con prec -1).
func shortest(f float64, upper bool) string {
	if f == 0 {
		if float64bits(f)>>63 == 1 {
			return "-0"
		}
		return "0"
	}
	for p := 0; p <= 17; p++ {
		s := formatFScaled(f, p)
		if v, err := ParseFloat(s, 64); err == nil && v == f {
			return canonical(f, s, upper)
		}
	}
	return formatFScaled(f, 17)
}

// canonical decide la notacion como Go (fija para exponentes chicos, cientifica para el resto).
func canonical(f float64, s string, upper bool) string {
	neg := false
	t := s
	if len(t) > 0 && t[0] == '-' {
		neg = true
		t = t[1:]
	}
	intLen := strings.IndexByte(t, '.')
	if intLen < 0 {
		intLen = len(t)
	}
	// exponente decimal
	exp := intLen - 1
	first := 0
	for first < len(t) && (t[first] == '0' || t[first] == '.') {
		first++
	}
	if first >= len(t) {
		return s
	}
	for i := 0; i < len(t); i++ {
		if t[i] >= '1' && t[i] <= '9' {
			first = i
			break
		}
	}
	if intLen == 1 && t[0] == '0' {
		// 0.000ddd: exponente negativo
		e := 0
		for i := intLen + 1; i < len(t) && t[i] == '0'; i++ {
			e--
		}
		exp = e - 1
	}
	if exp >= -4 && exp < 21 {
		return s
	}
	// notacion cientifica: primer digito, punto, el resto sin ceros de cola
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	digits := strings.ReplaceAll(t, ".", "")
	digits = strings.TrimRight(digits, "0")
	if len(digits) == 0 {
		return s
	}
	b.WriteByte(digits[0])
	if len(digits) > 1 {
		b.WriteByte('.')
		b.WriteString(digits[1:])
	}
	if upper {
		b.WriteByte('E')
	} else {
		b.WriteByte('e')
	}
	if exp >= 0 {
		b.WriteByte('+')
	} else {
		b.WriteByte('-')
		exp = -exp
	}
	es := Itoa(exp)
	if len(es) < 2 {
		es = "0" + es
	}
	b.WriteString(es)
	return b.String()
}

// formatE: notacion cientifica con precision.
func formatE(f float64, prec int, upper bool) string {
	neg := false
	if f < 0 {
		neg = true
		f = -f
	}
	if f == 0 {
		return sign(neg) + zeroMantissa(prec) + expSuffix(0, upper)
	}
	e := 0
	x := f
	for x >= 10 {
		x /= 10
		e++
	}
	for x < 1 {
		x *= 10
		e--
	}
	// redondear la mantisa a prec decimales con el camino exacto
	digits := fixedDigits(x, prec)
	// si el redondeo subio a 10.0, se corre el exponente
	if digits[0] == '1' && len(digits) > 1 && digits[1] == '0' {
		sub := strings.ReplaceAll(digits, ".", "")
		sub = strings.TrimLeft(sub, "0")
		if len(sub) > 0 && sub[0] == '1' && allZero(sub[1:]) {
			x /= 10
			e++
			digits = fixedDigits(x, prec)
		}
	}
	return sign(neg) + digits + expSuffix(e, upper)
}

func sign(neg bool) string {
	if neg {
		return "-"
	}
	return ""
}

func zeroMantissa(prec int) string {
	if prec == 0 {
		return "0"
	}
	return "0." + strings.Repeat("0", prec)
}

func expSuffix(e int, upper bool) string {
	c := byte('e')
	if upper {
		c = 'E'
	}
	s := "+"
	if e < 0 {
		s = "-"
		e = -e
	}
	es := Itoa(e)
	if len(es) < 2 {
		es = "0" + es
	}
	return string(c) + s + es
}

// formatG: la mas corta de 'f' y 'e'.
func formatG(f float64, prec int, upper bool) string {
	if f == 0 {
		return "0"
	}
	a := shortest(f, upper)
	_ = prec
	return a
}

// ---- flotantes: lectura ----

// ParseFloat lee un flotante. Correctamente redondeado cuando la mantisa entra en 53 bits y el
// exponente decimal esta entre -22 y 22 (el camino rapido de Go); fuera de ahi es aproximado.
func ParseFloat(s string, bitSize int) (float64, error) {
	const fnParseFloat = "ParseFloat"
	if s == "" {
		return 0, syntaxError(fnParseFloat, s)
	}
	// inf / nan
	if v, ok := specialFloat(s); ok {
		return v, nil
	}
	neg := false
	if s[0] == '+' || s[0] == '-' {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "" {
		return 0, syntaxError(fnParseFloat, s)
	}
	var mant uint64
	digits := 0
	exp := 0
	sawDigit := false
	sawPoint := false
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if sawPoint {
				return 0, syntaxError(fnParseFloat, s)
			}
			sawPoint = true
			continue
		}
		if c == 'e' || c == 'E' {
			break
		}
		if c < '0' || c > '9' {
			return 0, syntaxError(fnParseFloat, s)
		}
		sawDigit = true
		if digits < 19 {
			mant = mant*10 + uint64(c-'0')
			digits++
			if sawPoint {
				exp--
			}
		} else if !sawPoint {
			exp++
		}
	}
	if !sawDigit {
		return 0, syntaxError(fnParseFloat, s)
	}
	if i < len(s) {
		// exponente
		i++
		if i >= len(s) {
			return 0, syntaxError(fnParseFloat, s)
		}
		eneg := false
		if s[i] == '+' || s[i] == '-' {
			eneg = s[i] == '-'
			i++
		}
		if i >= len(s) {
			return 0, syntaxError(fnParseFloat, s)
		}
		e := 0
		for ; i < len(s); i++ {
			c := s[i]
			if c < '0' || c > '9' {
				return 0, syntaxError(fnParseFloat, s)
			}
			if e < 10000 {
				e = e*10 + int(c-'0')
			}
		}
		if eneg {
			e = -e
		}
		exp += e
	}
	var v float64
	if mant < 1<<53 && exp >= -22 && exp <= 22 {
		// camino rapido exacto
		if exp >= 0 {
			v = float64(mant) * pow10f(exp)
		} else {
			v = float64(mant) / pow10f(-exp)
		}
	} else {
		v = float64(mant)
		if exp > 0 {
			for ; exp > 0; exp -= 22 {
				n := 22
				if exp < 22 {
					n = exp
				}
				v *= pow10f(n)
			}
		} else {
			for ; exp < 0; exp += 22 {
				n := 22
				if -exp < 22 {
					n = -exp
				}
				v /= pow10f(n)
			}
		}
	}
	if neg {
		v = -v
	}
	if v == 0 && mant != 0 {
		return v, rangeError(fnParseFloat, s)
	}
	if isInf(v) {
		return v, rangeError(fnParseFloat, s)
	}
	return v, nil
}

func specialFloat(s string) (float64, bool) {
	switch s {
	case "nan", "NaN":
		return float64frombits(0x7FF8000000000001), true
	case "inf", "+inf", "Inf", "+Inf":
		return float64frombits(0x7FF0000000000000), true
	case "-inf", "-Inf":
		return float64frombits(0xFFF0000000000000), true
	}
	return 0, false
}

// ---- comillas (lo que necesita %q) ----

// IsPrint dice si el caracter es imprimible (ASCII, mas 0xA0-0xFF como Go).
func IsPrint(r rune) bool {
	if r < 0x20 || r > 0x10FFFF {
		return false
	}
	if r == 0x7F { // DEL: Go no lo considera imprimible
		return false
	}
	if r < 0x7F {
		return r >= 0x20
	}
	if r >= 0x80 && r < 0xA0 {
		return false
	}
	// en Latin-1 Go tampoco considera imprimibles el espacio duro ni el guion blando
	if r == 0xA0 || r == 0xAD {
		return false
	}
	return true
}

// Quote pone comillas y escapa como Go.
func Quote(s string) string { return quoteWith(s, false) }

// QuoteToASCII es como Quote pero con salida solo ASCII (lo que pide la bandera + de %q en Go).
func QuoteToASCII(s string) string { return quoteWith(s, true) }

func quoteWith(s string, ascii bool) string {
	const hexdigits = "0123456789abcdef"
	b := make([]byte, 0, len(s)+2)
	b = append(b, '"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		// byte suelto que no es UTF-8 valido: Go lo escapa como \xNN
		if r == utf8.RuneError && size == 1 && s[i] >= 0x80 {
			b = append(b, '\\', 'x', hexdigits[s[i]>>4], hexdigits[s[i]&0xF])
			i++
			continue
		}
		switch {
		case r == '"' || r == '\\':
			b = append(b, '\\', byte(r))
		case size == 1 && !IsPrint(r):
			switch r {
			case '\a':
				b = append(b, '\\', 'a')
			case '\b':
				b = append(b, '\\', 'b')
			case '\f':
				b = append(b, '\\', 'f')
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			case '\v':
				b = append(b, '\\', 'v')
			case '\\':
				b = append(b, '\\', '\\')
			case '"':
				b = append(b, '\\', '"')
			default:
				b = append(b, '\\', 'x', hexdigits[byte(r)>>4], hexdigits[byte(r)&0xF])
			}
		default:
			if ascii && r > 0x7F {
				b = appendRuneEscape(b, r)
			} else {
				b = append(b, s[i:i+size]...)
			}
		}
		i += size
	}
	return string(b) + `"`
}

// QuoteRune pone comillas simples a un caracter.
func QuoteRune(r rune) string { return quoteRuneWith(r, false) }

// QuoteRuneToASCII es como QuoteRune con salida solo ASCII.
func QuoteRuneToASCII(r rune) string { return quoteRuneWith(r, true) }

func quoteRuneWith(r rune, ascii bool) string {
	if r == '\'' {
		return `'\''`
	}
	if r == '\\' {
		return `'\\'`
	}
	switch r {
	case '\a':
		return `'\a'`
	case '\b':
		return `'\b'`
	case '\f':
		return `'\f'`
	case '\n':
		return `'\n'`
	case '\r':
		return `'\r'`
	case '\t':
		return `'\t'`
	case '\v':
		return `'\v'`
	}
	if ascii && r > 0x7F {
		return "'" + string(appendRuneEscape(nil, r)) + "'"
	}
	if IsPrint(r) && !(r >= 0xD800 && r <= 0xDFFF) {
		return "'" + string(r) + "'"
	}
	// como Go: \xNN solo para los de control ASCII, \uXXXX para el resto del BMP y \UXXXXXXXX
	// para lo que esta mas arriba; un rune invalido se muestra como el de reemplazo.
	switch {
	case r < 0x20 || r == 0x7F:
		return "'\\x" + hex2(byte(r)) + "'"
	case r < 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF):
		return "'\\u" + hex4(0xFFFD) + "'"
	case r > 0xFFFF:
		return "'\\U" + hex8(uint32(r)) + "'"
	default:
		return "'\\u" + hex4(uint16(r)) + "'"
	}
}

// appendRuneEscape agrega r en su forma escapada (\uXXXX o \UXXXXXXXX) con los bytes ASCII.
func appendRuneEscape(b []byte, r rune) []byte {
	if r < 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF) {
		r = 0xFFFD
	}
	if r > 0xFFFF {
		return append(b, []byte("\\U"+hex8(uint32(r)))...)
	}
	return append(b, []byte("\\u"+hex4(uint16(r)))...)
}

func hex2(b byte) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0xF]})
}

func hex4(v uint16) string {
	return hex2(byte(v>>8)) + hex2(byte(v))
}

func hex8(v uint32) string {
	return hex4(uint16(v>>16)) + hex4(uint16(v))
}

