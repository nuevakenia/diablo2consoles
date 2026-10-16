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
	"encoding/csv"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"math/bits"
	"sort"
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

	fmt.Fprintf(&sb, "var casosFormatFloat = []struct {\n\tf       float64\n\tform    byte\n\tprec    int\n\tesperado string\n}{\n")
	flotantesFmt := []float64{0, 1.5, 2.5, 3.14159265, 99.95, 0.145, 8.835, 1.0 / 3.0, -1.005, 100, 1e10, 1e14, -0.0}
	for _, f := range flotantesFmt {
		for _, p := range []int{0, 1, 2, 3, 4, 6} {
			if f == 1e14 && p > 2 {
				continue
			}
			fmt.Fprintf(&sb, "\t{%s, 'f', %d, %q},\n", floatLiteral(f), p, strconv.FormatFloat(f, 'f', p, 64))
		}
	}
	// 'g': la forma mas corta y con precision explicita. El exponente decide la notacion (cientifica
	// si es menor que -4 o mayor o igual que 6 con precision corta, o que la precision pedida).
	// Esta parte del corpus es la que destapo el defecto: la forma 'g' no se probaba.
	for _, f := range []float64{0, 1.5, 2.5, 3.14159265, 99.95, 0.145, 8.835, 1.0 / 3.0, -1.005, 100, 1e10, 1e14, 1e-5, 1e-4, 123456789.0, 5e-324, 1234.5678, -0.0, 1e21} {
		for _, p := range []int{-1, 1, 2, 3, 5} {
			fmt.Fprintf(&sb, "\t{%s, 'g', %d, %q},\n", floatLiteral(f), p, strconv.FormatFloat(f, 'g', p, 64))
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
	// ---- strings: lo que se sumo en DC-002.5 (Compare, IndexAny, LastIndexAny, Title, Cut) y
	// ---- bytes: las mismas operaciones sobre []byte (el campo a y b son los bytes como texto y el
	// POC los pasa a []byte) ----
	for _, s := range textosStr {
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "Compare", s, "a", strconv.Itoa(strings.Compare(s, "a")))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "IndexAny", s, "a,", strconv.Itoa(strings.IndexAny(s, "a,")))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "LastIndexAny", s, "a,", strconv.Itoa(strings.LastIndexAny(s, "a,")))
		if esASCII(s) {
			// Title de Go usa las tablas de Unicode: en la consola no hay y el limite esta declarado
			fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "Title", s, "", strings.Title(s))
		}
		antes, despues, hay := strings.Cut(s, ",")
		encontrado := 0
		if hay {
			encontrado = 1
		}
		fmt.Fprintf(&sb, "	{%q, %q, %q, %d, %q},\n", "Cut", s, ",", encontrado, antes+"\x01"+despues)

		bs := []byte(s)
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.Contains", s, "a", strconv.FormatBool(bytes.Contains(bs, []byte("a"))))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.Equal", s, s, strconv.FormatBool(bytes.Equal(bs, []byte(s))))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.HasPrefix", s, "a", strconv.FormatBool(bytes.HasPrefix(bs, []byte("a"))))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.HasSuffix", s, "a", strconv.FormatBool(bytes.HasSuffix(bs, []byte("a"))))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.IndexByte", s, "a", strconv.Itoa(bytes.IndexByte(bs, 'a')))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.Index", s, "a", strconv.Itoa(bytes.Index(bs, []byte("a"))))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.LastIndex", s, "a", strconv.Itoa(bytes.LastIndex(bs, []byte("a"))))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.ReplaceAll", s, ",", string(bytes.ReplaceAll(bs, []byte(","), []byte("-"))))
		fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.TrimSpace", s, "", string(bytes.TrimSpace(bs)))
		if esASCII(s) {
			// bytes.ToUpper de Go, con texto no ASCII, usa las tablas de Unicode
			fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.ToUpper", s, "", string(bytes.ToUpper(bs)))
			fmt.Fprintf(&sb, "	{%q, %q, %q, 0, %q},\n", "b.ToLower", s, "", string(bytes.ToLower(bs)))
		}
		{
			var trozos []string
			for _, t := range bytes.Split(bs, []byte(",")) {
				trozos = append(trozos, string(t))
			}
			fmt.Fprintf(&sb, "	{%q, %q, %q, -1, %q},\n", "b.Split", s, ",", strings.Join(trozos, "|"))
		}
	}

	fmt.Fprintf(&sb, "}\n")

	// ---- sort (DC-002.5) ----
	// El orden NO se puede comparar con Go cuando hay claves repetidas y el algoritmo no es estable
	// (Slice, Sort e Ints no lo son: dos ordenes distintos son los dos validos). Por eso:
	//   - casosOrden y casosInterfaz usan claves DISTINTAS: ahi el resultado es unico;
	//   - casosEstable usa claves REPETIDAS y guarda la posicion original de cada elemento: es el
	//     unico resultado que SliceStable puede dar.
	fmt.Fprintf(&sb, "type casoOrden struct {\n	nombre   string\n	datos    []int\n	esperado []int\n}\n\nvar casosOrden = []casoOrden{\n")
	secuencias := [][]int{
		{}, {1}, {2, 1}, {3, 1, 2}, {1, 2, 3, 4, 5}, {5, 4, 3, 2, 1},
		{9, 7, 5, 3, 1, 2, 4, 6, 8, 0},
		{100, -3, 0, 7, -2147483648, 2147483647, 42, -1},
	}
	for _, sec := range secuencias {
		d := append([]int(nil), sec...)
		sort.Ints(d)
		fmt.Fprintf(&sb, "	{%q, %s, %s},\n", "ints "+strconv.Itoa(len(sec)), intsLiteral(sec), intsLiteral(d))
	}
	// una secuencia larga con claves distintas (para que el quicksort haga trabajo de verdad)
	larga := make([]int, 200)
	for i := range larga {
		larga[i] = (i*7919 + 13) % 200
	}
	largaOrd := append([]int(nil), larga...)
	sort.Ints(largaOrd)
	fmt.Fprintf(&sb, "	{%q, %s, %s},\n", "ints 200", intsLiteral(larga), intsLiteral(largaOrd))
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "type casoEstable struct {\n	nombre   string\n	claves   []int\n	esperado []int\n}\n\nvar casosEstable = []casoEstable{\n")
	clavesCasos := [][]int{
		{2, 1, 2, 1, 2, 1},
		{5, 5, 5, 5},
		{3, 1, 2, 3, 1, 2, 3, 1},
		{1, 2, 3, 4, 5, 6, 7, 8, 9},
		{9, 8, 7, 6, 5, 4, 3, 2, 1},
	}
	// 60 claves con repetidos, deterministicas
	var largaRep []int
	for i := 0; i < 60; i++ {
		largaRep = append(largaRep, (i*13+5)%7)
	}
	clavesCasos = append(clavesCasos, largaRep)
	for _, cl := range clavesCasos {
		type par struct {
			clave   int
			orden   int
			relleno [2]int // para que el elemento no sea de 8 bytes justos
		}
		pares := make([]par, len(cl))
		for i, c := range cl {
			pares[i] = par{clave: c, orden: i}
		}
		sort.SliceStable(pares, func(i, j int) bool { return pares[i].clave < pares[j].clave })
		orden := make([]int, len(pares))
		for i, p := range pares {
			orden[i] = p.orden
		}
		fmt.Fprintf(&sb, "	{%q, %s, %s},\n", "estable "+strconv.Itoa(len(cl)), intsLiteral(cl), intsLiteral(orden))
	}
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "type casoBusca struct {\n	ordenados []int\n	x         int\n	esperado  int\n}\n\nvar casosBusca = []casoBusca{\n")
	busca := []int{1, 3, 5, 7, 9, 11, 13, 13, 20}
	for _, x := range []int{-5, 0, 1, 2, 3, 13, 14, 20, 21, 100} {
		fmt.Fprintf(&sb, "	{%s, %d, %d},\n", intsLiteral(busca), x, sort.SearchInts(busca, x))
	}
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "// resumen de lo medido en el host: orden %d casos, estable %d, busca %d\n", len(secuencias)+1, len(clavesCasos), 10)

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
	// ---- flag y encoding/csv (DC-002.5b): los dos paquetes de formato ----
	//
	// El armado de las banderas y la lectura de csv son los MISMOS en las dos puntas: el POC de la
	// consola tiene copiado el mismo codigo (poc/stdlib/main.go, armarFlag / leerCSV) pero con
	// d2std/flag y d2std/encoding/csv. El corpus compara el TEXTO armado (valores, argumentos que
	// quedan, cuantas se pusieron, el error y la salida de la ayuda) y, en csv, las filas, el texto
	// del error y el avance de la entrada.
	//
	// Lo que NO va aca: los casos cuyo resultado depende del ANCHO DE INT (un Int con un valor que no
	// entra en 32 bits, o el tope de int64). Este corpus se calcula con el int de 64 bits del host y
	// en la consola el int es de 32: esos casos viven en la prueba diferencial del host
	// (difftest/dc0025b_test.go), que corre en los dos anchos. Tampoco va el Writer de csv: el motor
	// no lo usa (solo lee tablas) y sus bytes se comparan en el host.
	fmt.Fprintf(&sb, "type casoFlag struct {\n	argv     []string\n	esperado string\n}\n\nvar casosFlag = []casoFlag{\n")
	for _, argv := range argvsFlag {
	fmt.Fprintf(&sb, "	{%s, %q},\n", strsLiteral(argv), armarFlag(argv))
	}
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "type casoCSV struct {\n	nombre    string\n	datos     string\n	coma      rune\n	comentario rune\n	campos    int\n	perezoso  bool\n	espacios  bool\n	reusa     bool\n	esperado  string\n}\n\nvar casosCSV = []casoCSV{\n")
	for _, e := range entradasCSV {
	for _, op := range opcionesCSV {
		fmt.Fprintf(&sb, "	{%q, %q, rune(%d), rune(%d), %d, %t, %t, %t, %q},\n",
			e.nombre + " / " + op.nombre, e.datos, op.coma, op.comentario, op.campos, op.perezoso, op.espacios, op.reusa,
			leerCSVHost(e.datos, op))
	}
	}
	fmt.Fprintf(&sb, "}\n\n")
	fmt.Fprintf(&sb, "// resumen de lo medido en el host: flag %d casos, csv %d combinaciones\\n", len(argvsFlag), len(entradasCSV)*len(opcionesCSV))

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

// intsLiteral escribe un []int como expresion Go.
func intsLiteral(v []int) string {
	if len(v) == 0 {
		return "[]int{}"
	}
	s := "[]int{"
	for i, n := range v {
		if i > 0 {
			s += ", "
		}
		s += strconv.Itoa(n)
	}
	return s + "}"
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

// ============================================================================
// flag y encoding/csv (DC-002.5b)
//
// El armado de abajo (armarFlag y leerCSVHost) esta COPIADO en el POC de la consola
// (console/dc/poc/stdlib/main.go) con d2std/flag y d2std/encoding/csv: son las dos puntas del mismo
// instrumento. Si se cambia aca, hay que cambiarlo alla: si divergen, el corpus deja de comparar el
// paquete y empieza a comparar dos armados distintos.
// ============================================================================

// acumulaFlag es el flag.Value de las dos puntas (String y Set, que es el contrato de flag.Value).
type acumulaFlag struct{ vals []string }

func (a *acumulaFlag) String() string { return strings.Join(a.vals, ",") }

func (a *acumulaFlag) Set(s string) error {
	if s == "error" {
		return errors.New("valor que no acepta")
	}
	a.vals = append(a.vals, s)
	return nil
}

// armarFlag corre el argv con el flag REAL de Go y devuelve el texto medible.
func armarFlag(args []string) string {
	fs := flag.NewFlagSet("corpus", flag.ContinueOnError)
	var buf strings.Builder
	fs.SetOutput(&buf)
	var a acumulaFlag
	fb := fs.Bool("b", false, "bandera b")
	fo := fs.Bool("otra", true, "bandera otra")
	fstr := fs.String("s", "def", "texto de la bandera s")
	fn := fs.Int("n", 7, "numero de la bandera n")
	fs.Var(&a, "v", "valor acumulado")
	err := fs.Parse(args)
	errTexto := ""
	if err != nil {
		errTexto = err.Error()
	}
	return fmt.Sprintf("b=%t otra=%t s=%q n=%d v=%q err=%q args=%q puestas=%d parsed=%t salida=%q",
		*fb, *fo, *fstr, *fn, a.String(), errTexto, strings.Join(fs.Args(), "|"), fs.NFlag(), fs.Parsed(), buf.String())
}

// argvsFlag: las lineas de comandos que se prueban en las DOS puntas. Las mismas del difftest del
// host, menos las que dependen del ancho de int (ahi estan marcadas soloHost).
var argvsFlag = [][]string{
	nil,
	{},
	{"-b"},
	{"-b=true"},
	{"-b=false"},
	{"-b=0"},
	{"-b=1"},
	{"-b=T"},
	{"--b"},
	{"-b=quizas"},
	{"-s=hola"},
	{"-s", "hola mundo"},
	{"-s="},
	{"-s", "-b"},
	{"--s=a\\b"},
	{"-s=con\nsalto"},
	{"-n=42"},
	{"-n", "-42"},
	{"-n=0x1f"},
	{"-n=0o17"},
	{"-n=017"},
	{"-n=+5"},
	{"-n=0b101"},
	{"-n=abc"},
	// el tope exacto del int de 32 bits SI va: entra en los dos anchos y da el mismo valor en texto.
	// El de 12 digitos (123456789012) NO: en el host (int de 64 bits) entra y en la consola (int de 32)
	// es "value out of range". Medido: lo cazo la corrida en la consola (2355 casos, 1 en rojo) y no lo
	// puede cazar el difftest, que compara Go contra nosotros CON EL MISMO ancho. Ese caso vive solo en
	// el difftest con GOARCH=386, que es el ancho de la consola.
	{"-n=2147483647"},
	{"-n=1", "-n=2"},
	{"-v=a", "-v=b"},
	{"-v=a", "-v=error"},
	{"-v"},
	{"-z"},
	{"--zz=1"},
	{"-h"},
	{"--help"},
	{"-"},
	{"-b", "--", "-n", "3"},
	{"---b"},
	{"-=1"},
	{"uno", "dos"},
	{"uno", "-b"},
	{"-1"},
	{"-s"},
	{"-b", "-n", "3", "arg1", "-s=x"},
}

func strsLiteral(v []string) string {
	partes := make([]string, 0, len(v))
	for _, s := range v {
		partes = append(partes, strconv.Quote(s))
	}
	return "[]string{" + strings.Join(partes, ", ") + "}"
}

// opcionesCSV / entradasCSV: los mismos juegos de opciones y las mismas entradas que el difftest del
// host (difftest/dc0025b_test.go). El del motor va primero: tabulador, campos libres y comillas
// perezosas, que es como el motor lee sus tablas de texto.
type opcionCSV struct {
	nombre     string
	coma       rune
	comentario rune
	campos     int
	perezoso   bool
	espacios   bool
	reusa      bool
}

var opcionesCSV = []opcionCSV{
	{"por defecto (coma)", ',', 0, 0, false, false, false},
	{"del motor (tabulador, campos libres, perezoso)", '	', 0, -1, true, false, false},
	{"campos fijos en 3", ',', 0, 3, false, false, false},
	{"con comentario #", ',', '#', 0, false, false, false},
	{"salteando el espacio del principio", ',', 0, 0, false, true, false},
	{"reusando la rebanada", ',', 0, 0, false, false, true},
	{"punto y coma", ';', 0, 0, false, false, false},
	{"separador invalido (la coma es el comentario)", ',', ',', 0, false, false, false},
	{"separador invalido (el comentario es una comilla)", ',', '"', 0, false, false, false},
	{"separador cero", 0, 0, 0, false, false, false},
	{"separador de mas de un byte", '\u2022', 0, 0, false, false, false},
}

var entradasCSV = []struct {
	nombre string
	datos  string
}{
	{"vacia", ""},
	{"solo salto", "\n"},
	{"solo dos saltos", "\n\n"},
	{"una fila", "a,b,c\n"},
	{"una fila sin salto final", "a,b,c"},
	{"dos filas", "a,b,c\nd,e,f\n"},
	{"dos filas sin salto final", "a,b,c\nd,e,f"},
	{"finales de Windows", "a,b\r\nc,d\r\n"},
	{"finales de Windows sin salto final", "a,b\r\nc,d"},
	{"campo entre comillas", "a,\"b\",c\n"},
	{"coma adentro de comillas", "\"a,b\",c\n"},
	{"comilla doble adentro", "\"a\"\"b\",c\n"},
	{"campo de varias lineas", "\"multi\nlinea\",x\n"},
	{"campo de varias lineas con Windows", "\"multi\r\nlinea\",x\r\n"},
	{"varias lineas y salto final", "\"a\nb\nc\",d\n"},
	{"comilla que cierra y sigue texto", "\"a\"b,c\n"},
	{"comilla suelta en el medio", "a\"b,c\n"},
	{"comilla suelta al final", "ab\",c\n"},
	{"campo vacio entre comillas", "\"\"\n"},
	{"cuatro comillas", "\"\"\"\"\n"},
	{"comilla sola", "\"\n"},
	{"comilla sin cerrar con linea nueva", "\"a\n"},
	{"comilla sin cerrar al final", "\"a"},
	{"comilla y nada", "\""},
	{"fila de distinto largo", "a,b\nc\n"},
	{"fila de distinto largo al reves", "a\nc,d,e\n"},
	{"lineas vacias en el medio", "a,b\n\n\nd,e\n"},
	{"saltos de sobra al final", "a,b\n\n\n"},
	{"espacios alrededor", "  a , b \n"},
	{"espacios adentro del campo", "a b,c\n"},
	{"comentario", "#comentario\na,b\n"},
	{"comentario con espacio adelante", " #no comentario\na,b\n"},
	{"comentario al final", "a,b\n#chau\n"},
	{"todo comentarios", "#uno\n#dos\n"},
	{"con tabuladores", "a	b	c\nd	e	f\n"},
	{"tabulador adentro de comillas", "\"a	b\",c\n"},
	{"campo vacio al final", "a,b,\n"},
	{"fila de un separador", ",\n"},
	{"no ascii", "café,niño\n"},
	{"BOM al principio", "\ufeffa,b\n"},
	{"byte nulo adentro", "a,b\x00c\n"},
	{"campo con salto de Windows suelto", "a\rb,c\n"},
}

// leerCSVHost: la misma corrida que hace el POC de la consola, con el encoding/csv REAL de Go.
func leerCSVHost(datos string, op opcionCSV) string {
	r := csv.NewReader(strings.NewReader(datos))
	r.Comma, r.Comment, r.FieldsPerRecord = op.coma, op.comentario, op.campos
	r.LazyQuotes, r.TrimLeadingSpace, r.ReuseRecord = op.perezoso, op.espacios, op.reusa
	filas, err := r.ReadAll()
	errTexto := ""
	if err != nil {
		errTexto = err.Error()
	}
	return "filas=" + serialFilasHost(filas) + " err=" + errTexto + " offset=" + strconv.FormatInt(r.InputOffset(), 10)
}

func serialFilasHost(filas [][]string) string {
	trozos := make([]string, len(filas))
	for i, f := range filas {
		trozos[i] = strings.Join(f, "\x01")
	}
	return strings.Join(trozos, "\x02")
}
