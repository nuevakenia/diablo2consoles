// Comando que genera la bateria de casos del autotest de la consola.
//
// En la Dreamcast no esta la stdlib de Go, asi que no hay contra que comparar. La unica forma
// honesta de verificar alla es traer las respuestas YA CALCULADAS por la stdlib real: este
// programa corre en el host, arma los casos, calcula la respuesta con la biblioteca estandar de
// verdad y escribe un archivo Go con la tabla (formato, argumentos, esperado). El POC de la
// consola compila esa tabla y comprueba que su implementacion da lo mismo.
//
// Uso:  go run ./cmd/gen-casos > <build>/gen/casos_gen.go
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

type cFmt struct {
	formato string
	args    []interface{}
}

func main() {
	var sb strings.Builder
	sb.WriteString("// GENERADO por console/dc/std/cmd/gen-casos: NO EDITAR A MANO.\n")
	sb.WriteString("// Las respuestas esperadas las calculo la biblioteca estandar real de Go en el host.\n")
	// El archivo generado lleva SUS imports (en Go son por archivo): math para los literales de
	// Inf/NaN y del cero negativo.
	sb.WriteString("//go:build dreamcast\n\n")
	sb.WriteString("package main\n\nimport \"math\"\n\n")

	// ---- fmt: los verbos, banderas, anchos y precisiones que usa el motor ----
	var casoFmt []cFmt
	verbos := []string{"%d", "%5d", "%-5d|", "%05d", "%+d", "% d", "%2d", "%10d", "%x", "%#x",
		"%04x", "%016x", "%X", "%o", "%b", "%c", "%t", "%q", "%v", "%s", "%10s|", "%-10s|",
		"%.3s", "%f", "%.0f", "%.1f", "%.2f", "%.3f", "%e", "%g", "%2.1f", "%08.2f", "%.0d"}
	enteros := []interface{}{0, 1, -1, 42, -42, 255, 1000, 233, 0x4E2D, int64(-9223372036854775808), int64(9223372036854775807), uint8(9)}
	textos := []interface{}{"", "hola", "con \"comillas\" y \\barra", "salto\nde linea	tab", "no ascii: \xc3\xa9", "a"}
	flotantes := []interface{}{0.0, 1.5, 2.5, 3.14159265, 99.95, 0.145, 8.835, 1.0 / 3.0, -1.005, 100.0, 1e10, 1e15, -0.0}
	otros := []interface{}{true, false, []byte("abc"), []byte{0x00, 0xff, 0x7f}}

	for _, f := range verbos {
		casoFmt = append(casoFmt, cFmt{f, []interface{}{42}})
		casoFmt = append(casoFmt, cFmt{f, []interface{}{-7}})
	}
	for _, v := range enteros {
		for _, f := range []string{"%d", "%x", "%X", "%o", "%b", "%v", "%q", "%c", "%05d", "%.0d"} {
			casoFmt = append(casoFmt, cFmt{f, []interface{}{v}})
		}
	}
	for _, v := range textos {
		for _, f := range []string{"%s", "%q", "%v", "%x", "%X", "%.3s", "%10s|", "%-10s|"} {
			casoFmt = append(casoFmt, cFmt{f, []interface{}{v}})
		}
	}
	for _, v := range flotantes {
		for _, f := range []string{"%f", "%.0f", "%.1f", "%.2f", "%.3f", "%2.1f", "%08.2f"} {
			casoFmt = append(casoFmt, cFmt{f, []interface{}{v}})
		}
	}
	for _, v := range otros {
		// %d y %c sobre un []byte: el verbo aplica al elemento, no al arreglo (Go recorre)
		for _, f := range []string{"%v", "%s", "%t", "%x", "%X", "%q", "%d", "%c"} {
			casoFmt = append(casoFmt, cFmt{f, []interface{}{v}})
		}
	}
	casoFmt = append(casoFmt,
		cFmt{"%d%%", []interface{}{50}},
		cFmt{"%d/%t/%s", []interface{}{3, true, "hola"}},
		cFmt{"%d%c", []interface{}{65, 'B'}},
		cFmt{"sin verbos", nil},
		cFmt{"%[2]d %[1]d", []interface{}{1, 2}},
		cFmt{"%*d", []interface{}{6, 42}},
		cFmt{"%d %d", []interface{}{1}},
		cFmt{"%!d(int=5)", nil},
	)

	fmt.Fprintf(&sb, "type casoFmt struct {\n\tformato  string\n\targs     []interface{}\n\tesperado string\n}\n\n")
	fmt.Fprintf(&sb, "var casosFmt = []casoFmt{\n")
	for _, c := range casoFmt {
		fmt.Fprintf(&sb, "\t{%q, %s, %q},\n", c.formato, argsLiteral(c.args), fmt.Sprintf(c.formato, c.args...))
	}
	fmt.Fprintf(&sb, "}\n\n")

	// ---- strconv ----
	// El esperado de Atoi se calcula con el ancho de int de la CONSOLA (32 bits), no con el del
	// host: Atoi devuelve un int y en el SH-4 ese int es de 32 bits. Calculado con el int de 64 bits
	// del host, el corpus exigia que "9223372036854775807" entrara sin error, cosa que en la consola
	// es imposible (Go de 32 bits tampoco lo hace: satura a MaxInt32 con error de rango).
	// Medido: sonda DC-002.2e, E22-E24.
	const anchoIntConsola = 32
	fmt.Fprintf(&sb, "type casoItoa struct {\n	n       int64\n	esperado string\n}\n\nvar casosItoa = []casoItoa{\n")
	for _, n := range []int64{0, 1, -1, 42, -42, 1000, 9223372036854775807, -9223372036854775808, 255} {
		fmt.Fprintf(&sb, "	{%d, %q},\n", n, strconv.FormatInt(n, 10))
	}
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "type casoAtoi struct {\n	s        string\n	esperado int64\n	hayError bool\n}\n\nvar casosAtoi = []casoAtoi{\n")
	for _, s := range []string{"0", "1", "-1", "42", "-42", "9999999", "abc", "", " 7", "12abc", "9223372036854775807", "-9223372036854775808", "9223372036854775808", "007", "2147483647", "2147483648"} {
		v, err := strconv.ParseInt(s, 10, anchoIntConsola)
		fmt.Fprintf(&sb, "	{%q, %d, %t},\n", s, v, err != nil)
	}
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "var casosQuote = []struct {\n\ts       string\n\tesperado string\n}{\n")
	for _, s := range []string{"", "hola", "con \"comillas\"", "barra \\", "salto\nnueva", "tab\taca", "\x00\x1b\x7f", "\xff\xfe", "no ascii \xc3\xa9"} {
		fmt.Fprintf(&sb, "\t{%q, %q},\n", s, strconv.Quote(s))
	}
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "var casosFormatFloat = []struct {\n\tf       float64\n\tprec    int\n\tesperado string\n}{\n")
	for _, f := range []float64{0, 1.5, 2.5, 3.14159265, 99.95, 0.145, 8.835, 1.0 / 3.0, -1.005, 100, 1e10, 1e14, -0.0} {
		for _, p := range []int{0, 1, 2, 3, 4, 6} {
			if f == 1e14 && p > 2 {
				continue
			}
			fmt.Fprintf(&sb, "\t{%s, %d, %q},\n", floatLiteral(f), p, strconv.FormatFloat(f, 'f', p, 64))
		}
	}
	fmt.Fprintf(&sb, "}\n\n")

	// ---- strings ----
	fmt.Fprintf(&sb, "type casoStr struct {\n\tfn       string\n\ta, b     string\n\tn        int\n\tesperado string\n}\n\nvar casosStr = []casoStr{\n")
	textosStr := []string{"", "  hola  ", "HOLA", "hola mundo", "a,b,,c", "  ", "\t\n x \r\n",
		"Diablo II", "data\\global\\ui", "\xc3\xa9\xc3\x89"}
	for _, s := range textosStr {
		fmt.Fprintf(&sb, "	{%q, %q, \"\", 0, %q},\n", "TrimSpace", s, strings.TrimSpace(s))
		if esASCII(s) {
			// el cambio de mayusculas solo se prueba en ASCII: no hay tablas Unicode en la
			// consola y el limite esta declarado en el paquete
			fmt.Fprintf(&sb, "	{%q, %q, \"\", 0, %q},\n", "ToLower", s, strings.ToLower(s))
			fmt.Fprintf(&sb, "	{%q, %q, \"\", 0, %q},\n", "ToUpper", s, strings.ToUpper(s))
		}
		fmt.Fprintf(&sb, "\t{%q, %q, \"a\", 0, %q},\n", "Contains", s, strconv.FormatBool(strings.Contains(s, "a")))
		fmt.Fprintf(&sb, "\t{%q, %q, \"a\", 0, %q},\n", "HasPrefix", s, strconv.FormatBool(strings.HasPrefix(s, "a")))
		fmt.Fprintf(&sb, "\t{%q, %q, \"a\", 0, %q},\n", "HasSuffix", s, strconv.FormatBool(strings.HasSuffix(s, "a")))
		fmt.Fprintf(&sb, "\t{%q, %q, \"a\", 0, %q},\n", "Index", s, strconv.Itoa(strings.Index(s, "a")))
		// EqualFold se prueba con el mismo texto en los dos lados (el campo b tiene que ser s, no "a")
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "EqualFold", s, s, strconv.FormatBool(strings.EqualFold(s, s)))
		fmt.Fprintf(&sb, "\t{%q, %q, \",\", -1, %q},\n", "Split", s, strings.Join(strings.Split(s, ","), "|"))
		fmt.Fprintf(&sb, "\t{%q, %q, \",\", -1, %q},\n", "ReplaceAll", s, strings.ReplaceAll(s, ",", "-"))
		fmt.Fprintf(&sb, "\t{%q, %q, \"\", 0, %q},\n", "Fields", s, strconv.Itoa(len(strings.Fields(s))))
	}
	for _, j := range [][]string{{"a", "b", "c"}, nil, {"uno"}, {"", ""}} {
		fmt.Fprintf(&sb, "\t{%q, %q, %q, 0, %q},\n", "Join", strings.Join(j, "|"), "", strings.Join(j, "|"))
	}
	fmt.Fprintf(&sb, "}\n")

	// ---- math, math/bits, hash/fnv, encoding/hex, encoding/binary (DC-002.4) ----
	// Los resultados de math se comparan por BITS (math.Float64bits), no por texto: asi el corpus
	// no depende de fmt y se ve el ultimo bit. Las funciones exactas van con modo "bits" (tienen que
	// dar identico); las trascendentales con "aprox" (polinomios propios: el POC acepta 1e-12
	// relativo, que es el limite declarado en el paquete).
	fmt.Fprintf(&sb, "type casoMath struct {\n	fn    string\n	a, b  float64\n	modo  string\n	ebits uint64\n}\n\n")
	fmt.Fprintf(&sb, "var casosMath = []casoMath{\n")
	puntosMath := []float64{0, -0.0, 0.1, -0.1, 0.5, -0.5, 1, -1, 1.5, -1.5, 2, -2, 2.5, -2.5,
		3.7, -3.7, 42, -42, 100, -100, 12345.678, -12345.678, 1e10, 1e-10, 0.3333333333333333,
		0.7071067811865476, 3.141592653589793, 0.987654321012345}
	unarias := []struct {
		nombre  string
		f       func(float64) float64
		modo    string
		dominio func(float64) bool
	}{
		{"Abs", math.Abs, "bits", siempre},
		{"Floor", math.Floor, "bits", siempre},
		{"Ceil", math.Ceil, "bits", siempre},
		{"Trunc", math.Trunc, "bits", siempre},
		{"Round", math.Round, "bits", siempre},
		{"RoundToEven", math.RoundToEven, "bits", siempre},
		{"Sqrt", math.Sqrt, "aprox", mayorQue0},
		{"Exp", math.Exp, "aprox", siempre},
		{"Log", math.Log, "aprox", mayorQue0},
		{"Sin", math.Sin, "aprox", hasta100},
		{"Cos", math.Cos, "aprox", hasta100},
		{"Atan", math.Atan, "aprox", siempre},
		{"Acos", math.Acos, "aprox", entreMenos1y1},
	}
	nMath := 0
	for _, u := range unarias {
		for _, x := range puntosMath {
			if !u.dominio(x) {
				continue
			}
			y := u.f(x)
			if math.IsNaN(y) || math.IsInf(y, 0) {
				continue // los especiales van en la tabla de clases (los bits del NaN no son comparables)
			}
			fmt.Fprintf(&sb, "	{%q, %s, 0, %q, 0x%016x},\n", u.nombre, floatLiteral(x), u.modo, math.Float64bits(y))
			nMath++
		}
	}
	binarias := []struct {
		nombre string
		f      func(float64, float64) float64
		modo   string
	}{
		{"Atan2", math.Atan2, "aprox"},
		{"Hypot", math.Hypot, "aprox"},
		{"Pow", math.Pow, "aprox"},
		{"Min", math.Min, "bits"},
		{"Max", math.Max, "bits"},
		{"Ldexp", func(x, e float64) float64 { return math.Ldexp(x, int(e)) }, "bits"},
	}
	binarios := [][2]float64{{2, 2}, {2, 3}, {2, 0.5}, {10, 3}, {1.5, 2.5}, {0.5, -2}, {3, 12},
		{7, -0.3333333333}, {3, 4}, {-3, 4}, {1, 1}, {0, 1}, {-1, 1}, {1, 10}, {1.5, -5}, {3, 1023}, {0.1, 0}}
	for _, b := range binarias {
		for _, p := range binarios {
			if b.nombre == "Pow" && (p[0] < 0 || (p[1] != math.Trunc(p[1]) && p[0] < 0)) {
				continue
			}
			y := b.f(p[0], p[1])
			if math.IsNaN(y) || math.IsInf(y, 0) {
				continue
			}
			fmt.Fprintf(&sb, "	{%q, %s, %s, %q, 0x%016x},\n", b.nombre, floatLiteral(p[0]), floatLiteral(p[1]), b.modo, math.Float64bits(y))
			nMath++
		}
	}
	// Min y Max con los dos ordenes y con NaN a un lado (Go devuelve el otro)
	for _, b := range []struct {
		nombre string
		f      func(float64, float64) float64
	}{{"Min", math.Min}, {"Max", math.Max}} {
		for _, p := range [][2]float64{{3, 7}, {7, 3}, {-3, 7}, {-7, -3}, {0, -0.0}, {-0.0, 0}} {
			y := b.f(p[0], p[1])
			fmt.Fprintf(&sb, "	{%q, %s, %s, %q, 0x%016x},\n", b.nombre, floatLiteral(p[0]), floatLiteral(p[1]), "bits", math.Float64bits(y))
			nMath++
		}
	}
	fmt.Fprintf(&sb, "}\n\n")

	// clases: los especiales (Inf, NaN, el cero con signo) se comparan por CLASE, no por bits
	fmt.Fprintf(&sb, "type casoClase struct {\n	fn       string\n	a        float64\n	esperado string\n}\n\nvar casosClase = []casoClase{\n")
	clases := []struct {
		fn string
		a  float64
	}{{"Clase", math.Inf(1)}, {"Clase", math.Inf(-1)}, {"Clase", math.NaN()}, {"Clase", 1.0},
		{"Clase", 0.0}, {"Clase", math.Copysign(0, -1)}, {"Clase", -3.5},
		{"IsInf+", math.Inf(1)}, {"IsInf-", math.Inf(-1)}, {"IsNaN", math.NaN()}, {"IsNaN", 2.0},
		{"Signbit", math.Copysign(0, -1)}, {"Signbit", 0.0}, {"Signbit", -1.0}, {"Signbit", 1.0}}
	for _, c := range clases {
		fmt.Fprintf(&sb, "	{%q, %s, %q},\n", c.fn, floatLiteral(c.a), claseDe(c.fn, c.a))
	}
	fmt.Fprintf(&sb, "}\n\n")

	// math/bits: OnesCount8 con los 256 valores; las rotaciones y los ceros con puntos elegidos
	fmt.Fprintf(&sb, "type casoBits struct {\n	fn       string\n	x        uint64\n	k        int\n	esperado uint64\n}\n\nvar casosBits = []casoBits{\n")
	nBits := 0
	for v := 0; v < 256; v++ {
		fmt.Fprintf(&sb, "	{%q, 0x%02x, 0, %d},\n", "OnesCount8", v, bits.OnesCount8(uint8(v)))
		nBits++
	}
	for _, v := range []uint32{0, 1, 0x80000000, 0xdeadbeef, 0xffffffff, 0x12345678} {
		for _, k := range []int{0, 1, 7, 8, 16, 31, 32, 33, -1, -5} {
			fmt.Fprintf(&sb, "	{%q, 0x%08x, %d, %d},\n", "RotateLeft32", v, k, bits.RotateLeft32(v, k))
			fmt.Fprintf(&sb, "	{%q, 0x%08x, %d, %d},\n", "RotateRight32", v, k, bits.RotateLeft32(v, -k))
			nBits += 2
		}
		for _, fn := range []struct {
			nombre string
			f      func(uint32) int
		}{{"LeadingZeros32", bits.LeadingZeros32}, {"TrailingZeros32", bits.TrailingZeros32}, {"Len32", bits.Len32}} {
			fmt.Fprintf(&sb, "	{%q, 0x%08x, 0, %d},\n", fn.nombre, v, fn.f(v))
			nBits++
		}
	}
	fmt.Fprintf(&sb, "}\n\n")

	// hash/fnv y encoding/hex: con las cadenas que usa el motor (rutas de archivos y claves)
	fmt.Fprintf(&sb, "type casoHash struct {\n	texto  string\n	e32    uint32\n	e64    uint64\n	hex    string\n}\n\nvar casosHash = []casoHash{\n")
	textosHash := []string{"", "a", "data\\global\\ui\\logo.dc6", "Diablo II", "MPQ", "0x1234567890abcdef",
		"palette\\act1\\pal.pl2", "d2data.mpq", "12345678901234567890", "\x00\x01\x02\xff"}
	bytesHash := [][]byte{{}, {0}, {0, 0, 0}, {0xff, 0xff, 0xff, 0xff, 0xff}, {1, 2, 3, 4, 5, 6, 7, 8, 9}}
	for _, s := range textosHash {
		h32 := fnv.New32a()
		h32.Write([]byte(s))
		h64 := fnv.New64a()
		h64.Write([]byte(s))
		fmt.Fprintf(&sb, "	{%q, %d, %d, %q},\n", s, h32.Sum32(), h64.Sum64(), hex.EncodeToString([]byte(s)))
	}
	for _, b := range bytesHash {
		h32 := fnv.New32a()
		h32.Write(b)
		h64 := fnv.New64a()
		h64.Write(b)
		fmt.Fprintf(&sb, "	{%q, %d, %d, %q},\n", string(b), h32.Sum32(), h64.Sum64(), hex.EncodeToString(b))
	}
	fmt.Fprintf(&sb, "}\n\n")

	// encoding/binary: los enteros de las cabeceras del juego, y Read desde un bytes.Reader
	fmt.Fprintf(&sb, "type casoBinary struct {\n	bytes    []byte\n	u16, u32 uint64\n	u64      string\n}\n\nvar casosBinary = []casoBinary{\n")
	for _, b := range [][]byte{{0, 0}, {1, 0}, {0xff, 0xff}, {0x34, 0x12}, {0xef, 0xbe, 0xad, 0xde},
		{0, 0, 0, 0x80}, {0xff, 0xff, 0xff, 0x7f}, {0, 1, 2, 3, 4, 5, 6, 7}, {8, 7, 6, 5, 4, 3, 2, 1},
		{0xef, 0xbe, 0xad, 0xde, 0xef, 0xbe, 0xad, 0xde}} {
		var u16, u32 uint64
		if len(b) >= 2 {
			u16 = uint64(binary.LittleEndian.Uint16(b))
		}
		if len(b) >= 4 {
			u32 = uint64(binary.LittleEndian.Uint32(b))
		}
		u64 := ""
		if len(b) >= 8 {
			u64 = strconv.FormatUint(binary.LittleEndian.Uint64(b), 10)
		}
		fmt.Fprintf(&sb, "	{%s, %d, %d, %q},\n", bytesLiteral(b), u16, u32, u64)
	}
	fmt.Fprintf(&sb, "}\n\n")

	// encoding/binary.Read: los tipos que el motor lee del MPQ y de los DC6
	fmt.Fprintf(&sb, "type casoRead struct {\n	dato  []byte\n	tipo  string\n	valor string\n}\n\nvar casosRead = []casoRead{\n")
	readBytes := []byte{0x34, 0x12, 0x78, 0x56, 0xbc, 0x9a, 0xef, 0xbe, 0xde}
	for _, t := range []string{"uint16", "int16", "uint32", "int32", "uint64", "int64", "float32", "float64"} {
		var valor string
		r := bytes.NewReader(readBytes)
		switch t {
		case "uint16":
			var v uint16
			binary.Read(r, binary.LittleEndian, &v)
			valor = strconv.FormatUint(uint64(v), 10)
		case "int16":
			var v int16
			binary.Read(r, binary.LittleEndian, &v)
			valor = strconv.FormatInt(int64(v), 10)
		case "uint32":
			var v uint32
			binary.Read(r, binary.LittleEndian, &v)
			valor = strconv.FormatUint(uint64(v), 10)
		case "int32":
			var v int32
			binary.Read(r, binary.LittleEndian, &v)
			valor = strconv.FormatInt(int64(v), 10)
		case "uint64":
			var v uint64
			binary.Read(r, binary.LittleEndian, &v)
			valor = strconv.FormatUint(v, 10)
		case "int64":
			var v int64
			binary.Read(r, binary.LittleEndian, &v)
			valor = strconv.FormatInt(v, 10)
		case "float32":
			var v float32
			binary.Read(r, binary.LittleEndian, &v)
			valor = "bits " + strconv.FormatUint(math.Float64bits(float64(v)), 16)
		case "float64":
			var v float64
			binary.Read(r, binary.LittleEndian, &v)
			valor = "bits " + strconv.FormatUint(math.Float64bits(v), 16)
		}
		fmt.Fprintf(&sb, "	{%s, %q, %q},\n", bytesLiteral(readBytes), t, valor)
	}
	fmt.Fprintf(&sb, "}\n\n")
	fmt.Fprintf(&sb, "// resumen de lo medido en el host: math %d casos, bits %d casos\n", nMath, nBits)

	fmt.Print(sb.String())
}

// dominios: que puntos tiene sentido pedirle a cada funcion (lo de afuera esta declarado como
// limite en el paquete, y el corpus no exige mas de lo declarado)
func siempre(x float64) bool       { return true }
func mayorQue0(x float64) bool     { return x > 0 }
func hasta100(x float64) bool      { return math.Abs(x) <= 100 }
func entreMenos1y1(x float64) bool { return x >= -1 && x <= 1 }

// claseDe es la semantica de los casos de clase (Inf, NaN, el cero con signo): la misma cuenta
// hace el POC en la consola, pero aca con la math de Go, para tener el esperado.
func claseDe(fn string, a float64) string {
	switch fn {
	case "Clase":
		switch {
		case math.IsNaN(a):
			return "nan"
		case math.IsInf(a, 1):
			return "+inf"
		case math.IsInf(a, -1):
			return "-inf"
		case a == 0 && math.Signbit(a):
			return "-0"
		case a == 0:
			return "+0"
		}
		return "num"
	case "IsInf+":
		return strconv.FormatBool(math.IsInf(a, 1))
	case "IsInf-":
		return strconv.FormatBool(math.IsInf(a, -1))
	case "IsNaN":
		return strconv.FormatBool(math.IsNaN(a))
	case "Signbit":
		return strconv.FormatBool(math.Signbit(a))
	}
	return "fn desconocida"
}

func bytesLiteral(b []byte) string {
	s := "[]byte{"
	for i, c := range b {
		if i > 0 {
			s += ", "
		}
		s += "0x" + hex2(c)
	}
	return s + "}"
}

// argsLiteral imprime los argumentos como expresion Go, respetando el tipo de cada uno.
func argsLiteral(args []interface{}) string {
	if len(args) == 0 {
		return "nil"
	}
	s := "[]interface{}{"
	for i, a := range args {
		if i > 0 {
			s += ", "
		}
		s += literal(a)
	}
	return s + "}"
}

func esASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func literal(v interface{}) string {
	switch x := v.(type) {
	case int:
		return "int(" + strconv.Itoa(x) + ")"
	case int64:
		return "int64(" + strconv.FormatInt(x, 10) + ")"
	case uint8:
		return "uint8(" + strconv.Itoa(int(x)) + ")"
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return floatLiteral(x)
	case string:
		return strconv.Quote(x)
	case []byte:
		s := "[]byte{"
		for i, c := range x {
			if i > 0 {
				s += ", "
			}
			s += "0x" + hex2(c)
		}
		return s + "}"
	case rune:
		return "rune(" + strconv.Itoa(int(x)) + ")"
	}
	return "nil"
}

// floatLiteral escribe un float64 como expresion Go. Tres casos que no son un literal comun:
// Inf y NaN (strconv los escribe +Inf/NaN, que no son Go valido) y el CERO NEGATIVO (la constante
// -0.0 de Go es +0: el signo del cero no existe en las constantes, hay que pedirlo con Copysign).
func floatLiteral(f float64) string {
	switch {
	case math.IsNaN(f):
		return "math.NaN()"
	case math.IsInf(f, 1):
		return "math.Inf(1)"
	case math.IsInf(f, -1):
		return "math.Inf(-1)"
	case f == 0 && math.Signbit(f):
		return "math.Copysign(0, -1)"
	}
	return "float64(" + strconv.FormatFloat(f, 'g', -1, 64) + ")"
}

func hex2(b byte) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0xF]})
}
