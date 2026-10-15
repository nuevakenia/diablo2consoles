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
	"fmt"
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
	sb.WriteString("package main\n\n")

	// ---- fmt: los verbos, banderas, anchos y precisiones que usa el motor ----
	var casoFmt []cFmt
	verbos := []string{"%d", "%5d", "%-5d|", "%05d", "%+d", "% d", "%2d", "%10d", "%x", "%#x",
		"%04x", "%016x", "%X", "%o", "%b", "%c", "%t", "%q", "%v", "%s", "%10s|", "%-10s|",
		"%.3s", "%f", "%.0f", "%.1f", "%.2f", "%.3f", "%e", "%g", "%2.1f", "%08.2f", "%.0d"}
	enteros := []interface{}{0, 1, -1, 42, -42, 255, 1000, int64(-9223372036854775808), int64(9223372036854775807), uint8(9)}
	textos := []interface{}{"", "hola", "con \"comillas\" y \\barra", "salto\nde linea\ttab", "no ascii: \xc3\xa9", "a"}
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
		for _, f := range []string{"%v", "%s", "%t", "%x", "%X", "%q"} {
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
	fmt.Fprintf(&sb, "type casoItoa struct {\n\tn       int64\n\tesperado string\n}\n\nvar casosItoa = []casoItoa{\n")
	for _, n := range []int64{0, 1, -1, 42, -42, 1000, 9223372036854775807, -9223372036854775808, 255} {
		fmt.Fprintf(&sb, "\t{%d, %q},\n", n, strconv.FormatInt(n, 10))
	}
	fmt.Fprintf(&sb, "}\n\n")

	fmt.Fprintf(&sb, "type casoAtoi struct {\n	s        string\n	esperado int64\n	hayError bool\n}\n\nvar casosAtoi = []casoAtoi{\n")
	for _, s := range []string{"0", "1", "-1", "42", "-42", "9999999", "abc", "", " 7", "12abc", "9223372036854775807", "-9223372036854775808", "9223372036854775808", "007"} {
		v, err := strconv.ParseInt(s, 10, 64)
		fmt.Fprintf(&sb, "\t{%q, %d, %t},\n", s, v, err != nil)
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

	fmt.Print(sb.String())
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

func floatLiteral(f float64) string {
	return "float64(" + strconv.FormatFloat(f, 'g', -1, 64) + ")"
}

func hex2(b byte) string {
	const hex = "0123456789abcdef"
	return string([]byte{hex[b>>4], hex[b&0xF]})
}
