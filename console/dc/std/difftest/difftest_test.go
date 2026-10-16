// Prueba diferencial: la misma entrada por nuestra biblioteca estandar y por la de Go, y la
// salida tiene que ser byte a byte igual.
//
// Por que asi: en el host no tengo Dreamcast, pero si tengo la implementacion de referencia. Un
// "anda" mio no vale nada; que coincida con Go en miles de casos, si. Y lo que no coincida queda
// contado y reportado, no escondido: el dominio declarado (los flotantes exactos, el ASCII en
// mayusculas) esta escrito en el codigo de cada paquete y aca se mide donde termina.
//
// go test ./... dentro de console/dc/std
package difftest

import (
	realerrors "errors"
	realfmt "fmt"
	realstrconv "strconv"
	realstrings "strings"
	"testing"

	myerrors "d2std/errors"
	myfmt "d2std/fmt"
	mystrconv "d2std/strconv"
	mystrings "d2std/strings"
)

// ---- generador determinista: la misma corrida da siempre los mismos casos ----

type rnd struct{ s uint64 }

func (r *rnd) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}

func (r *rnd) intn(n int) int { return int(r.next() % uint64(n)) }

type tipo struct {
	nombre string
	gen    func(*rnd) interface{}
}

var tipos = []tipo{
	{"int", func(r *rnd) interface{} { return int(int64(r.next())) }},
	{"int chico", func(r *rnd) interface{} { return r.intn(200) - 100 }},
	{"int64", func(r *rnd) interface{} { return int64(r.next()) }},
	{"uint", func(r *rnd) interface{} { return uint(r.next()) }},
	{"uint8", func(r *rnd) interface{} { return uint8(r.next()) }},
	{"bool", func(r *rnd) interface{} { return r.next()&1 == 1 }},
	{"string", func(r *rnd) interface{} { return randString(r) }},
	{"float64", func(r *rnd) interface{} { return float64(int64(r.next()%100000)) / 100 }},
	{"float64 chico", func(r *rnd) interface{} { return float64(r.intn(1000)) / 8 }},
	{"[]byte", func(r *rnd) interface{} { return []byte(randString(r)) }},
	{"error", func(r *rnd) interface{} { return myerrors.New(randString(r)) }},
	{"Stringer", func(r *rnd) interface{} { return str{r.intn(1000)} }},
}

type str struct{ n int }

func (s str) String() string { return "str:" + mystrconv.Itoa(s.n) }

func randString(r *rnd) string {
	n := r.intn(12)
	b := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		switch r.intn(5) {
		case 0:
			b = append(b, byte('a'+r.intn(26)))
		case 1:
			b = append(b, byte('A'+r.intn(26)))
		case 2:
			b = append(b, byte('0'+r.intn(10)))
		case 3:
			especiales := []byte(" 	\n\r\x00\x1b\"\\'\xc3\xa9\xff")
			b = append(b, especiales[r.intn(len(especiales))])
		default:
			b = append(b, byte('-'))
		}
	}
	return string(b)
}

func argDe(r *rnd, t tipo) interface{} { return t.gen(r) }

// ---- 1. tabla a mano: los usos que el motor tiene medidos ----

func TestFormatoAMano(t *testing.T) {
	casos := []struct {
		formato string
		args    []interface{}
	}{
		// los verbos del motor, con sus banderas y precisiones reales
		{"%d", []interface{}{0}},
		{"%d", []interface{}{-1}},
		{"%d", []interface{}{42}},
		{"%d", []interface{}{int64(-9223372036854775808)}},
		{"%d", []interface{}{int64(9223372036854775807)}},
		{"%5d", []interface{}{42}},
		{"%-5d|", []interface{}{42}},
		{"%05d", []interface{}{42}},
		{"%05d", []interface{}{-42}},
		{"%+d", []interface{}{42}},
		{"%+d", []interface{}{-42}},
		{"% d", []interface{}{42}},
		{"%2d", []interface{}{7}},
		{"%10d", []interface{}{-7}},
		{"%d%%", []interface{}{50}},
		{"%d/%t/%s", []interface{}{3, true, "hola"}},
		{"%d%c", []interface{}{65, 'B'}},
		{"%s", []interface{}{"hola"}},
		{"%s", []interface{}{""}},
		{"%10s|", []interface{}{"hola"}},
		{"%-10s|", []interface{}{"hola"}},
		{"%.3s", []interface{}{"holamundo"}},
		{"%.0s", []interface{}{"hola"}},
		{"%q", []interface{}{"hola"}},
		{"%q", []interface{}{"con \"comillas\" y \\barra"}},
		{"%q", []interface{}{"salto\nde linea\ttab"}},
		{"%q", []interface{}{"no ascii: \xc3\xa9"}},
		{"%q", []interface{}{"\x00\x1b\x7f"}},
		{"%q", []interface{}{"\xff\xfe"}},
		{"%q", []interface{}{'A'}},
		{"%q", []interface{}{'\n'}},
		{"%x", []interface{}{255}},
		{"%#x", []interface{}{255}},
		{"%04x", []interface{}{255}},
		{"%016x", []interface{}{uint64(0xdeadbeef)}},
		{"%X", []interface{}{255}},
		{"%x", []interface{}{"abc"}},
		{"%x", []interface{}{[]byte{0xde, 0xad}}},
		{"%c", []interface{}{65}},
		{"%c", []interface{}{'\u00e9'}},
		{"%t", []interface{}{true}},
		{"%t", []interface{}{false}},
		{"%v", []interface{}{42}},
		{"%v", []interface{}{-42}},
		{"%v", []interface{}{"hola"}},
		{"%v", []interface{}{true}},
		{"%v", []interface{}{nil}},
		{"%v", []interface{}{uint8(9)}},
		{"%v", []interface{}{[]byte("ab")}},
		{"%s", []interface{}{myerrors.New("un error")}},
		{"%v", []interface{}{myerrors.New("un error")}},
		{"%s", []interface{}{str{7}}},
		{"%v", []interface{}{str{7}}},
		{"%s", []interface{}{42}},
		{"%d", []interface{}{"hola"}},
		{"%z", []interface{}{42}},
		{"%d %d", []interface{}{1}},
		{"sin verbos", []interface{}{}},
		{"100%% seguro", []interface{}{}},
		{"%[2]d %[1]d", []interface{}{1, 2}},
		{"%*d", []interface{}{6, 42}},
		{"%.*f", []interface{}{2, 3.14159}},
		// flotantes en el rango del juego (porcentajes y multiplicadores)
		{"%f", []interface{}{3.14159265}},
		{"%.0f", []interface{}{99.5}},
		{"%.0f", []interface{}{98.5}},
		{"%.0f", []interface{}{2.5}},
		{"%.0f", []interface{}{1.5}},
		{"%.0f", []interface{}{0.5}},
		{"%.1f", []interface{}{0.1}},
		{"%.1f", []interface{}{99.95}},
		{"%.2f", []interface{}{0.145}},
		{"%.2f", []interface{}{8.835}},
		{"%.2f", []interface{}{100}},
		{"%.3f", []interface{}{1.0 / 3.0}},
		{"%.2f%%", []interface{}{75.5}},
		{"%.0f", []interface{}{-0.0}},
		{"%f", []interface{}{-0.0}},
		{"%.2f", []interface{}{-1.005}},
		{"%.1f", []interface{}{1e10}},
		{"%.3f", []interface{}{1e14}},
		{"%f", []interface{}{0.000001}},
		{"%.2f", []interface{}{1e15}},
		{"%.0f", []interface{}{1e18}},
		{"%2.1f", []interface{}{3.14}},
		{"%08.2f", []interface{}{3.14}},
		{"%.1f", []interface{}{float32(3.14)}},

		// %c fuera del rango de un caracter valido: Go imprime U+FFFD (mira el valor sin signo,
		// asi que un int64 negativo tambien cae ahi). Aca el corpus de la consola fallaba con un
		// byte nulo porque nuestro %c confiaba en la conversion rune -> texto sin ese control.
		{"%c", []interface{}{int64(-9223372036854775808)}},
		{"%c", []interface{}{int64(9223372036854775807)}},
		{"%c", []interface{}{int(255)}},
		{"%c", []interface{}{int(233)}},
		{"%c", []interface{}{uint64(1 << 63)}},
		{"%c", []interface{}{uint32(0x1F600)}},
		{"%c", []interface{}{0xD800}},
		{"%c", []interface{}{-1}},
		{"%c", []interface{}{0x4E2D}},

		// un verbo que no aplica al tipo del elemento: Go recorre el arreglo y aplica el verbo a
		// cada elemento ("[%!t(uint8=97) ...]"), no al arreglo entero.
		{"%t", []interface{}{[]byte("abc")}},
		{"%t", []interface{}{[]byte{0x00, 0xff, 0x7f}}},
		{"%d", []interface{}{[]byte("abc")}},
		{"%c", []interface{}{[]byte("abc")}},
		{"%b", []interface{}{[]byte{1, 2}}},
		{"%5d", []interface{}{[]byte{1, 2}}},
		{"%.2d", []interface{}{[]byte{1, 2}}},
		{"%t", []interface{}{[]string{"a", "b"}}},
	}

	ok, malos := 0, 0
	for _, c := range casos {
		quiero := realfmt.Sprintf(c.formato, c.args...)
		tengo := myfmt.Sprintf(c.formato, c.args...)
		if quiero == tengo {
			ok++
			continue
		}
		malos++
		t.Errorf("Sprintf(%q, %v):\n  Go     %q\n  nuestro %q", c.formato, c.args, quiero, tengo)
	}
	t.Logf("a mano: %d/%d coinciden", ok, ok+malos)
}

// ---- 2. bateria generada ----

var verbosEnteros = []byte{'d', 'x', 'X', 'b', 'o', 'c', 'q', 'v', 't'}
var verbosTexto = []byte{'s', 'q', 'v', 'x', 'X'}
var verbosFlotante = []byte{'f', 'e', 'g'}

func TestFormatoGenerado(t *testing.T) {
	r := &rnd{s: 20261015}
	const total = 40000
	ok, malos, fueraDominio := 0, 0, 0
	primeros := 0
	for i := 0; i < total; i++ {
		tp := tipos[r.intn(len(tipos))]
		arg := argDe(r, tp)

		var verbo byte
		switch {
		case tp.nombre == "Stringer" || tp.nombre == "error":
			// estos dos se imprimen llamando a String()/Error(); nada de verbos numericos
			verbo = []byte{'v', 's', 'q'}[r.intn(3)]
		case tp.nombre == "float64" || tp.nombre == "float64 chico":
			verbo = verbosFlotante[r.intn(len(verbosFlotante))]
		case tp.nombre == "string" || tp.nombre == "[]byte":
			verbo = verbosTexto[r.intn(len(verbosTexto))]
		default:
			verbo = verbosEnteros[r.intn(len(verbosEnteros))]
		}

		formato := "%"
		if r.intn(3) == 0 && !(tp.nombre != "string") {
			// %#v solo se prueba con textos: la sintaxis Go de structs/slices necesita reflect
			formato += []string{"-", "+", "#", " ", "0"}[r.intn(5)]
		}
		if r.intn(3) == 0 {
			formato += mystrconv.Itoa(r.intn(10))
		}
		if r.intn(3) == 0 {
			formato += "." + mystrconv.Itoa(r.intn(7))
		}
		formato += string(verbo)

		quiero := realfmt.Sprintf(formato, arg)
		tengo := myfmt.Sprintf(formato, arg)
		if quiero == tengo {
			ok++
			continue
		}
		// el dominio declarado: %v/%g/%e de flotantes fuera del rango exacto puede divergir
		if verbo == 'g' || verbo == 'e' ||
			(verbo == 'v' && (tp.nombre == "float64" || tp.nombre == "float64 chico")) ||
			((verbo == 'f') && !flotanteEnDominio(arg, formato)) {
			fueraDominio++
			continue
		}
		malos++
		if primeros < 12 {
			primeros++
			t.Errorf("Sprintf(%q, %#v [%s]):\n  Go     %q\n  nuestro %q", formato, arg, tp.nombre, quiero, tengo)
		}
	}
	t.Logf("generado: %d/%d coinciden, %d fuera del dominio declarado (flotantes), %d mal",
		ok, total, fueraDominio, malos)
	if fueraDominio > total/10 {
		t.Errorf("demasiados casos fuera del dominio (%d de %d): dejar de declararlo y arreglarlo", fueraDominio, total)
	}
}

// flotanteEnDominio dice si nuestro camino exacto cubre el caso (precision <= 4 y sin desborde).
func flotanteEnDominio(arg interface{}, formato string) bool {
	var f float64
	switch v := arg.(type) {
	case float64:
		f = v
	case float32:
		f = float64(v)
	default:
		return true
	}
	prec := 6
	if i := realstrings.IndexByte(formato, '.'); i >= 0 {
		prec = 0
		for j := i + 1; j < len(formato) && '0' <= formato[j] && formato[j] <= '9'; j++ {
			prec = prec*10 + int(formato[j]-'0')
		}
	}
	if prec > 4 {
		return false
	}
	if f < 0 {
		f = -f
	}
	esc := 1.0
	for i := 0; i < prec; i++ {
		esc *= 10
	}
	return f*esc < 1.8e19
}

// ---- 3. strconv ----

func TestStrconvGenerado(t *testing.T) {
	r := &rnd{s: 424242}
	ok, malos := 0, 0

	// enteros: ida y vuelta y lectura
	for i := 0; i < 8000; i++ {
		n := int64(r.next())
		if i%3 == 0 {
			n = n % 100000
		}
		ok, malos = comparar(t, "Itoa/FormatInt", realstrconv.FormatInt(n, 10), mystrconv.FormatInt(n, 10), ok, malos)
		s := realstrconv.FormatInt(n, 10)
		rn, en := realstrconv.ParseInt(s, 10, 64)
		mn, em := mystrconv.ParseInt(s, 10, 64)
		if rn != mn || (en == nil) != (em == nil) {
			malos++
			if malos < 10 {
				t.Errorf("ParseInt(%q): Go (%d,%v) nuestro (%d,%v)", s, rn, en, mn, em)
			}
		} else {
			ok++
		}
	}

	// textos raros de entrada
	entradas := []string{"", " ", "+", "-", "abc", "12abc", "0x10", "010", "1_000", " 7",
		"9223372036854775807", "9223372036854775808", "-9223372036854775808", "-9223372036854775809",
		"999999999999999999999999", "0", "-0", "+0", "1e3", "1.5", "--5", "5-"}
	for _, s := range entradas {
		for _, base := range []int{0, 2, 8, 10, 16} {
			rn, en := realstrconv.ParseInt(s, base, 64)
			mn, em := mystrconv.ParseInt(s, base, 64)
			if rn != mn || errTexto(en) != errTexto(em) {
				malos++
				if malos < 20 {
					t.Errorf("ParseInt(%q, %d): Go (%d, %v) nuestro (%d, %v)", s, base, rn, en, mn, em)
				}
			} else {
				ok++
			}
			if base == 10 {
				ra, ea := realstrconv.Atoi(s)
				ma, eb := mystrconv.Atoi(s)
				if ra != ma || errTexto(ea) != errTexto(eb) {
					malos++
					if malos < 20 {
						t.Errorf("Atoi(%q): Go (%d, %v) nuestro (%d, %v)", s, ra, ea, ma, eb)
					}
				} else {
					ok++
				}
			}
		}
	}

	// comillas
	for i := 0; i < 4000; i++ {
		s := randString(r)
		ok, malos = comparar(t, "Quote", realstrconv.Quote(s), mystrconv.Quote(s), ok, malos)
	}

	// flotantes: en el dominio declarado (hasta 4 decimales, valores del tamano del juego)
	for i := 0; i < 4000; i++ {
		f := float64(int64(r.next()%100000000)) / 1000
		if r.intn(4) == 0 {
			f = -f
		}
		prec := r.intn(5)
		ok, malos = comparar(t, "FormatFloat f",
			realstrconv.FormatFloat(f, 'f', prec, 64),
			mystrconv.FormatFloat(f, 'f', prec, 64), ok, malos)
	}

	// lectura de flotantes
	fueraDeDominio := 0
	for _, s := range []string{"0", "1", "-1", "3.14159", "1e3", "1E3", "1e-3", "-2.5e-4",
		"0.1", "100000", "1234567890.12345", "1e308", "1e-308", ".5", "5.", "1e", "e3", "",
		"+1", "1.2.3", "0.000001", "9007199254740993"} {
		rf, ef := realstrconv.ParseFloat(s, 64)
		mf, em := mystrconv.ParseFloat(s, 64)
		if rf != mf || errTexto(ef) != errTexto(em) {
			if !parseFloatEnDominio(s) {
				fueraDeDominio++
				continue
			}
			malos++
			if malos < 30 {
				t.Errorf("ParseFloat(%q): Go (%v, %v) nuestro (%v, %v)", s, rf, ef, mf, em)
			}
		} else {
			ok++
		}
	}

	t.Logf("strconv: %d coinciden, %d mal, %d fuera del dominio declarado (flotantes fuera del camino exacto)",
		ok, malos, fueraDeDominio)
}

// parseFloatEnDominio: nuestra lectura es correctamente redondeada cuando la mantisa entra en 53
// bits y el exponente decimal esta entre -22 y 22 (el camino exacto). Fuera de ahi es aproximada y
// esta declarado en el paquete.
func parseFloatEnDominio(s string) bool {
	t := s
	if len(t) > 0 && (t[0] == '+' || t[0] == '-') {
		t = t[1:]
	}
	if i := realstrings.IndexAny(t, "eE"); i >= 0 {
		exp := t[i+1:]
		t = t[:i]
		e := 0
		for j := 0; j < len(exp); j++ {
			if exp[j] == '+' || exp[j] == '-' {
				continue
			}
			if exp[j] < '0' || exp[j] > '9' {
				return false
			}
			e = e*10 + int(exp[j]-'0')
			if e > 100 {
				return false
			}
		}
		if e > 22 {
			return false
		}
	}
	digitos := 0
	for i := 0; i < len(t); i++ {
		if t[i] >= '1' && t[i] <= '9' {
			digitos++
		}
	}
	if digitos > 15 {
		return false
	}
	// |valor| dentro de 1e-22 y 1e22
	intParte := 0
	punto := realstrings.IndexByte(t, '.')
	if punto < 0 {
		punto = len(t)
	}
	for i := 0; i < punto; i++ {
		if t[i] >= '0' && t[i] <= '9' {
			intParte++
		}
	}
	if intParte > 22 {
		return false
	}
	return true
}

func errTexto(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

func comparar(t *testing.T, que, quiero, tengo string, ok, malos int) (int, int) {
	if quiero == tengo {
		return ok + 1, malos
	}
	if malos < 10 {
		t.Errorf("%s:\n  Go     %q\n  nuestro %q", que, quiero, tengo)
	}
	return ok, malos + 1
}

// ---- 4. strings ----

func TestStringsGenerado(t *testing.T) {
	r := &rnd{s: 777}
	ok, malos, fueraDeDominio := 0, 0, 0

	for i := 0; i < 6000; i++ {
		s := randString(r)
		sep := string([]byte{'a', '-', ' ', '\t'}[r.intn(4)])
		ok, malos = comparar(t, "TrimSpace", realstrings.TrimSpace(s), mystrings.TrimSpace(s), ok, malos)
		if esASCII(s) {
			ok, malos = comparar(t, "ToLower", realstrings.ToLower(s), mystrings.ToLower(s), ok, malos)
			ok, malos = comparar(t, "ToUpper", realstrings.ToUpper(s), mystrings.ToUpper(s), ok, malos)
		} else {
			fueraDeDominio++
		}
		ok, malos = comparar(t, "Join(Split)", realstrings.Join(realstrings.Split(s, sep), "|"),
			mystrings.Join(mystrings.Split(s, sep), "|"), ok, malos)
		ok, malos = comparar(t, "HasPrefix", boolStr(realstrings.HasPrefix(s, "a")),
			boolStr(mystrings.HasPrefix(s, "a")), ok, malos)
		ok, malos = comparar(t, "Contains", boolStr(realstrings.Contains(s, sep)),
			boolStr(mystrings.Contains(s, sep)), ok, malos)
		if esASCII(s) {
			ok, malos = comparar(t, "EqualFold", boolStr(realstrings.EqualFold(s, mystrings.ToLower(s))),
				boolStr(mystrings.EqualFold(s, mystrings.ToLower(s))), ok, malos)
		} else {
			fueraDeDominio++
		}
		ok, malos = comparar(t, "Index", realstrconv.Itoa(realstrings.Index(s, sep)),
			realstrconv.Itoa(mystrings.Index(s, sep)), ok, malos)
		rep := r.intn(5)
		ok, malos = comparar(t, "Repeat", realstrings.Repeat("ab", rep), mystrings.Repeat("ab", rep), ok, malos)
		ok, malos = comparar(t, "Replace", realstrings.Replace(s, "a", "XY", 2), mystrings.Replace(s, "a", "XY", 2), ok, malos)
		ok, malos = comparar(t, "ReplaceAll", realstrings.ReplaceAll(s, "a", "b"), mystrings.ReplaceAll(s, "a", "b"), ok, malos)
		ok, malos = comparar(t, "Fields", realstrconv.Itoa(len(realstrings.Fields(s))),
			realstrconv.Itoa(len(mystrings.Fields(s))), ok, malos)
	}
	t.Logf("strings: %d coinciden, %d mal, %d fuera del dominio declarado (cambio de mayusculas fuera de ASCII)",
		ok, malos, fueraDeDominio)
}

// esASCII: las funciones de mayusculas y de comparacion sin distinguirlas solo se exigen en ASCII
// (no hay tablas Unicode en la consola; esta declarado en el paquete)
func esASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ---- 4b. el ancho del entero: bitSize 0 significa "el ancho de int de la plataforma" ----

// TestEnterosPorAncho: Atoi es ParseInt(s, 10, 0), y el bitSize 0 quiere decir "el ancho de int de
// la plataforma": 64 bits en el host, 32 en la consola (medido en la sonda DC-002.2e, E24). Por eso
// el camino honesto de probarlo en el host es por bitSize explicito: 32 hace la misma cuenta que
// Atoi en la consola. Al desbordar, Go satura al tope DEL ANCHO pedido y devuelve error de rango;
// nosotros devolviamos los topes de 64 bits, que en un int de 32 bits quedan truncados a -1 y 0.
func TestEnterosPorAncho(t *testing.T) {
	muestras := []string{"0", "-0", "42", "-42", "127", "128", "255", "32767", "32768",
		"2147483647", "2147483648", "-2147483648", "-2147483649",
		"9223372036854775807", "9223372036854775808", "-9223372036854775808", "-9223372036854775809",
		"18446744073709551615", "18446744073709551616", "999999999999999999999999", "abc", ""}
	ok, malos := 0, 0
	for _, s := range muestras {
		for _, bits := range []int{0, 8, 16, 32, 64} {
			ri, re := realstrconv.ParseInt(s, 10, bits)
			mi, me := mystrconv.ParseInt(s, 10, bits)
			if ri != mi || (re == nil) != (me == nil) {
				malos++
				if malos < 15 {
					t.Errorf("ParseInt(%q, 10, %d): Go (%d, %v) nuestro (%d, %v)", s, bits, ri, re, mi, me)
				}
			} else {
				ok++
			}
			ru, rue := realstrconv.ParseUint(s, 10, bits)
			mu, mue := mystrconv.ParseUint(s, 10, bits)
			if ru != mu || (rue == nil) != (mue == nil) {
				malos++
				if malos < 15 {
					t.Errorf("ParseUint(%q, 10, %d): Go (%d, %v) nuestro (%d, %v)", s, bits, ru, rue, mu, mue)
				}
			} else {
				ok++
			}
		}
		// Atoi, con el ancho de int del host (64). El de 32 bits de la consola lo cubre la misma
		// cuenta via ParseInt(...,32) de arriba, y la corrida entera en 386 (ver test-std-host.sh).
		ra, rea := realstrconv.Atoi(s)
		ma, mea := mystrconv.Atoi(s)
		if ra != ma || (rea == nil) != (mea == nil) {
			malos++
			if malos < 15 {
				t.Errorf("Atoi(%q): Go (%d, %v) nuestro (%d, %v)", s, ra, rea, ma, mea)
			}
		} else {
			ok++
		}
	}
	t.Logf("anchos de entero: %d coinciden, %d mal", ok, malos)
}

// ---- 5. errores y el %w ----

func TestErrorfConW(t *testing.T) {
	base := myerrors.New("base")
	realBase := realerrors.New("base")

	quiero := realfmt.Errorf("contexto: %w (codigo %d)", realBase, 7)
	tengo := myfmt.Errorf("contexto: %w (codigo %d)", base, 7)
	if quiero.Error() != tengo.Error() {
		t.Errorf("Errorf con %%w:\n  Go     %q\n  nuestro %q", quiero.Error(), tengo.Error())
	}
	if !myerrors.Is(tengo, base) {
		t.Errorf("errors.Is no encuentra el error envuelto: %v", tengo)
	}
	if myerrors.Unwrap(tengo) != base {
		t.Errorf("Unwrap no devuelve el error original")
	}

	// sin %w no hay cadena
	simple := myfmt.Errorf("solo texto %d", 1)
	if myerrors.Unwrap(simple) != nil {
		t.Errorf("Errorf sin %%w no debe envolver nada")
	}
	if simple.Error() != "solo texto 1" {
		t.Errorf("Errorf sin %%w: %q", simple.Error())
	}
}

// ---- 6. los limites declarados se comportan como estan documentados ----

func TestLimitesDeclarados(t *testing.T) {
	// mayusculas no ASCII: nuestra version las deja como estan (documentado)
	noAscii := "CAFÉ-\xc3\x89"
	if got := mystrings.ToLower(noAscii); got != "cafÉ-\xc3\x89" && got != "café-\xc3\x89" {
		t.Errorf("ToLower no ASCII: %q (documentado: deja el byte como esta)", got)
	}

	// %v sobre un tipo que no sabemos imprimir: aviso visible, no una mentira
	estructura := struct{ A int }{1}
	got := myfmt.Sprintf("%v", estructura)
	if !realstrings.HasPrefix(got, "%!v(") {
		t.Errorf("%%v sobre struct deberia avisar: %q", got)
	}

	// %T sin reflect: nombre para los tipos conocidos, aviso para el resto
	if n := myfmt.Sprintf("%T", 42); n != "int" {
		t.Errorf("%%T de int: %q", n)
	}
	if n := myfmt.Sprintf("%T", estructura); n != "tipo no soportado" {
		t.Errorf("%%T de struct: %q", n)
	}
}
