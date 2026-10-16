//go:build dreamcast

// DC-002.2: la biblioteca estandar minima corriendo en la Dreamcast emulada.
//
// Corre la misma familia de casos que la prueba diferencial del host (console/dc/std/difftest),
// pero aca no esta la stdlib de Go para comparar: las respuestas esperadas vienen calculadas por
// la biblioteca estandar REAL en el host y horneadas en casos_gen.go durante el build (lo genera
// scripts/console/gen-std-casos.sh con console/dc/std/cmd/gen-casos).
//
// La evidencia es la consola serie (numeros) y el color de pantalla: verde si coincidieron todos,
// rojo si alguno no.
package main

import (
	"bytes"
	"container/ring"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/bits"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

// extern: el nombre del simbolo de C es tal cual (con go:extern gccgo lo mangla)
//
//extern d2c_serial_init
func d2c_serial_init()

//extern d2c_pantalla_init
func d2c_pantalla_init()

//extern d2c_pantalla_color
func d2c_pantalla_color(r, g, b float32)

//extern d2c_frame
func d2c_frame()

//extern d2c_us
func d2c_us() uint64

var bien, mal int
var fallos []string

func check(familia, caso, esperado, tengo string) {
	if esperado == tengo {
		bien++
		return
	}
	mal++
	if len(fallos) < 12 {
		fallos = append(fallos, familia+" "+caso+" esperaba "+strconv.Quote(esperado)+" y dio "+strconv.Quote(tengo))
	}
}

func main() {
	d2c_serial_init()
	println("STD-TEST: arranca el autotest de la stdlib minima (DC-002.2)")

	correrFmt()
	correrStrconv()
	correrStrings()
	correrBytes()
	correrErrores()
	correrMath()
	correrClase()
	correrBits()
	correrHash()
	correrBinary()
	correrSort()
	correrPropiedades()
	medirCostoFloat64()

	for _, f := range fallos {
		println("STD-TEST FALLO:", f)
	}
	println("STD-TEST: casos", strconv.Itoa(bien+mal), "bien", strconv.Itoa(bien), "mal", strconv.Itoa(mal))

	d2c_pantalla_init()
	if mal == 0 {
		println("STD-TEST: RESULTADO OK - coincidio con la stdlib real en la consola")
		d2c_pantalla_color(0.0, 0.7, 0.0)
	} else {
		println("STD-TEST: RESULTADO MAL - hubo casos que no coincidieron")
		d2c_pantalla_color(0.7, 0.0, 0.0)
	}

	// el PVR necesita que el programa siga vivo
	for {
		d2c_frame()
	}
}

func correrFmt() {
	for _, c := range casosFmt {
		tengo := fmt.Sprintf(c.formato, c.args...)
		check("fmt", strconv.Quote(c.formato), c.esperado, tengo)
	}
}

func correrStrconv() {
	for _, c := range casosItoa {
		check("strconv", "Itoa", c.esperado, strconv.FormatInt(c.n, 10))
	}
	for _, c := range casosAtoi {
		v, err := strconv.Atoi(c.s)
		tengo := strconv.FormatInt(int64(v), 10) + " error=" + strconv.FormatBool(err != nil)
		esperado := strconv.FormatInt(c.esperado, 10) + " error=" + strconv.FormatBool(c.hayError)
		check("strconv", "Atoi("+strconv.Quote(c.s)+")", esperado, tengo)
	}
	for _, c := range casosQuote {
		check("strconv", "Quote", c.esperado, strconv.Quote(c.s))
	}
	for _, c := range casosFormatFloat {
		check("strconv", "FormatFloat "+string([]byte{c.form}), c.esperado, strconv.FormatFloat(c.f, c.form, c.prec, 64))
	}
}

func correrStrings() {
	for _, c := range casosStr {
		if len(c.fn) > 2 && c.fn[0] == 'b' && c.fn[1] == '.' {
			continue // los casos de bytes los corre correrBytes
		}
		var tengo string
		switch c.fn {
		case "TrimSpace":
			tengo = strings.TrimSpace(c.a)
		case "ToLower":
			tengo = strings.ToLower(c.a)
		case "ToUpper":
			tengo = strings.ToUpper(c.a)
		case "Contains":
			tengo = strconv.FormatBool(strings.Contains(c.a, c.b))
		case "HasPrefix":
			tengo = strconv.FormatBool(strings.HasPrefix(c.a, c.b))
		case "HasSuffix":
			tengo = strconv.FormatBool(strings.HasSuffix(c.a, c.b))
		case "Index":
			tengo = strconv.Itoa(strings.Index(c.a, c.b))
		case "EqualFold":
			tengo = strconv.FormatBool(strings.EqualFold(c.a, c.b))
		case "Split":
			tengo = strings.Join(strings.Split(c.a, c.b), "|")
		case "ReplaceAll":
			tengo = strings.ReplaceAll(c.a, c.b, "-")
		case "Fields":
			tengo = strconv.Itoa(len(strings.Fields(c.a)))
		case "Join":
			tengo = strings.Join(strings.Split(c.a, "|"), "|")
		case "Compare":
			tengo = strconv.Itoa(strings.Compare(c.a, c.b))
		case "IndexAny":
			tengo = strconv.Itoa(strings.IndexAny(c.a, c.b))
		case "LastIndexAny":
			tengo = strconv.Itoa(strings.LastIndexAny(c.a, c.b))
		case "Title":
			tengo = strings.Title(c.a)
		case "Cut":
			antes, despues, hay := strings.Cut(c.a, c.b)
			tengo = antes + "\x01" + despues
			if hay {
				tengo = tengo // el esperado lleva el 1 en n, no en el texto
			}
		default:
			tengo = "funcion desconocida: " + c.fn
		}
		check("strings", c.fn, c.esperado, tengo)
	}
}

// correrBytes: las mismas operaciones de busqueda sobre []byte (DC-002.5). El campo a es el texto y
// se pasa a []byte; el resultado se compara como texto.
func correrBytes() {
	for _, c := range casosStr {
		if len(c.fn) < 2 || c.fn[0] != 'b' || c.fn[1] != '.' {
			continue
		}
		a := []byte(c.a)
		b := []byte(c.b)
		var tengo string
		switch c.fn[2:] {
		case "Contains":
			tengo = strconv.FormatBool(bytes.Contains(a, b))
		case "Equal":
			tengo = strconv.FormatBool(bytes.Equal(a, b))
		case "HasPrefix":
			tengo = strconv.FormatBool(bytes.HasPrefix(a, b))
		case "HasSuffix":
			tengo = strconv.FormatBool(bytes.HasSuffix(a, b))
		case "IndexByte":
			tengo = strconv.Itoa(bytes.IndexByte(a, b[0]))
		case "Index":
			tengo = strconv.Itoa(bytes.Index(a, b))
		case "LastIndex":
			tengo = strconv.Itoa(bytes.LastIndex(a, b))
		case "ReplaceAll":
			tengo = string(bytes.ReplaceAll(a, b, []byte("-")))
		case "TrimSpace":
			tengo = string(bytes.TrimSpace(a))
		case "ToUpper":
			tengo = string(bytes.ToUpper(a))
		case "ToLower":
			tengo = string(bytes.ToLower(a))
		case "Split":
			var trozos []string
			for _, t := range bytes.Split(a, b) {
				trozos = append(trozos, string(t))
			}
			tengo = strings.Join(trozos, "|")
		default:
			tengo = "funcion desconocida: " + c.fn
		}
		check("bytes", c.fn, c.esperado, tengo)
	}
}

// correrSort: el orden (DC-002.5). El contraste con Go es el siguiente:
//   - casosOrden/casosInterfaz/casosBusca: claves DISTINTAS o listas ya ordenadas, donde el resultado
//     es unico (Slice y Sort no son estables: con claves repetidas dos ordenes distintos valen).
//   - casosEstable: claves REPETIDAS y la posicion original de cada una: es lo que tiene que dar
//     SliceStable (ese SI es estable, y su resultado es unico).
//
// Ademas Slice se prueba con elementos de 12 bytes (una estructura) y con textos, para que el ancho
// del elemento salga del descriptor del tipo y no de un supuesto (es lo que se midio en la sonda-h).
func correrSort() {
	for _, c := range casosOrden {
		a := append([]int(nil), c.datos...)
		sort.Ints(a)
		check("sort", "Ints "+c.nombre, intsTexto(c.esperado), intsTexto(a))

		// el mismo caso por las tres puertas: Slice, Sort (interfaz) y SliceStable
		b := append([]int(nil), c.datos...)
		sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
		check("sort", "Slice "+c.nombre, intsTexto(c.esperado), intsTexto(b))

		d := append([]int(nil), c.datos...)
		sort.Sort(tipoLista(d))
		check("sort", "Sort "+c.nombre, intsTexto(c.esperado), intsTexto(d))

		e := append([]int(nil), c.datos...)
		sort.SliceStable(e, func(i, j int) bool { return e[i] < e[j] })
		check("sort", "SliceStable "+c.nombre, intsTexto(c.esperado), intsTexto(e))

		// la lista ordenada tiene que dar IsSorted verdadero
		check("sort", "IsSorted "+c.nombre, "true", strconv.FormatBool(sort.IntsAreSorted(a)))
		if !sort.SliceIsSorted(a, func(i, j int) bool { return a[i] < a[j] }) {
			check("sort", "SliceIsSorted "+c.nombre, "true", "false")
		} else {
			check("sort", "SliceIsSorted "+c.nombre, "true", "true")
		}
	}

	// SliceStable con claves repetidas: el resultado es unico y tiene que dar EXACTO
	for _, c := range casosEstable {
		type par struct {
			clave   int
			orden   int
			relleno [2]int
		}
		pares := make([]par, len(c.claves))
		for i, k := range c.claves {
			pares[i] = par{clave: k, orden: i}
		}
		sort.SliceStable(pares, func(i, j int) bool { return pares[i].clave < pares[j].clave })
		orden := make([]int, len(pares))
		for i, p := range pares {
			orden[i] = p.orden
		}
		check("sort", "SliceStable "+c.nombre, intsTexto(c.esperado), intsTexto(orden))
	}

	// la busqueda binaria
	for _, c := range casosBusca {
		tengo := sort.SearchInts(c.ordenados, c.x)
		check("sort", "SearchInts "+strconv.Itoa(c.x), strconv.Itoa(c.esperado), strconv.Itoa(tengo))
	}

	// un slice de estructuras de 12 bytes y uno de textos (el ancho del elemento cambia)
	tipoPunto := []punto{{3, 1, 9}, {1, 2, 8}, {2, 3, 7}, {-1, 0, 6}, {3, 0, 5}}
	sort.Slice(tipoPunto, func(i, j int) bool { return tipoPunto[i].id < tipoPunto[j].id })
	tengo := ""
	for i, p := range tipoPunto {
		if i > 0 {
			tengo += "|"
		}
		tengo += strconv.Itoa(int(p.id))
	}
	check("sort", "Slice []punto (12 bytes)", "5|6|7|8|9", tengo)

	ss := []string{"pera", "manzana", "banana", "kiwi"}
	sort.Slice(ss, func(i, j int) bool { return ss[i] < ss[j] })
	check("sort", "Slice []string", "banana|kiwi|manzana|pera", strings.Join(ss, "|"))

	// la interfaz de Go, con un tipo de la consola
	fl := []float64{2.5, -1.5, 0, 1e10, -0.25}
	sort.Sort(tipoFloat(fl))
	check("sort", "Sort []float64 por interfaz", "-1.5|-0.25|0|2.5|1e+10", floatsTexto(fl))
}

// tipoLista y tipoFloat implementan sort.Interface (Len/Less/Swap).
type tipoLista []int

func (t tipoLista) Len() int           { return len(t) }
func (t tipoLista) Less(i, j int) bool { return t[i] < t[j] }
func (t tipoLista) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

type tipoFloat []float64

func (t tipoFloat) Len() int           { return len(t) }
func (t tipoFloat) Less(i, j int) bool { return t[i] < t[j] }
func (t tipoFloat) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

type punto struct {
	x, y int32
	id   int32
}

func intsTexto(v []int) string {
	var b []byte
	for i, n := range v {
		if i > 0 {
			b = append(b, '|')
		}
		b = strconv.AppendInt(b, int64(n), 10)
	}
	return string(b)
}

func floatsTexto(v []float64) string {
	var b []byte
	for i, f := range v {
		if i > 0 {
			b = append(b, '|')
		}
		b = append(b, strconv.FormatFloat(f, 'g', -1, 64)...)
	}
	return string(b)
}

// ---- DC-002.4: math, math/bits, hash/fnv, encoding/hex, encoding/binary ----

// bits64 y deBits reinterpretan un float64 a sus bits: los casos de math se comparan por BITS (asi
// el corpus no depende de fmt y un ultimo bit distinto se ve).
func bits64(f float64) uint64 { return *(*uint64)(unsafe.Pointer(&f)) }
func deBits(b uint64) float64 { return *(*float64)(unsafe.Pointer(&b)) }
func hex64(b uint64) string   { return strconv.FormatUint(b, 16) }

// llamarMath despacha por nombre (no hay reflexion en la consola).
func llamarMath(fn string, a, b float64) float64 {
	switch fn {
	case "Abs":
		return math.Abs(a)
	case "Floor":
		return math.Floor(a)
	case "Ceil":
		return math.Ceil(a)
	case "Trunc":
		return math.Trunc(a)
	case "Round":
		return math.Round(a)
	case "RoundToEven":
		return math.RoundToEven(a)
	case "Sqrt":
		return math.Sqrt(a)
	case "Exp":
		return math.Exp(a)
	case "Log":
		return math.Log(a)
	case "Sin":
		return math.Sin(a)
	case "Cos":
		return math.Cos(a)
	case "Atan":
		return math.Atan(a)
	case "Acos":
		return math.Acos(a)
	case "Atan2":
		return math.Atan2(a, b)
	case "Hypot":
		return math.Hypot(a, b)
	case "Pow":
		return math.Pow(a, b)
	case "Min":
		return math.Min(a, b)
	case "Max":
		return math.Max(a, b)
	case "Ldexp":
		return math.Ldexp(a, int(b))
	}
	return 0
}

func correrMath() {
	for _, c := range casosMath {
		tengo := llamarMath(c.fn, c.a, c.b)
		if c.modo == "bits" {
			check("math", c.fn, hex64(c.ebits), hex64(bits64(tengo)))
			continue
		}
		// aprox: las trascendentales son polinomios propios y coinciden a 12 cifras (declarado)
		esp := deBits(c.ebits)
		if dentroDe(tengo, esp, 1e-12) {
			bien++
			continue
		}
		mal++
		if len(fallos) < 12 {
			fallos = append(fallos, "math "+c.fn+" aprox: mio="+hex64(bits64(tengo))+" go="+hex64(c.ebits))
		}
	}
}

// dentroDe dice si a esta a menos de tol relativo de b.
func dentroDe(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	if b < 0 {
		b = -b
	}
	return d <= b*tol
}

// claseDe es la misma cuenta que hizo el host para armar los esperados.
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

func correrClase() {
	for _, c := range casosClase {
		check("math", c.fn, c.esperado, claseDe(c.fn, c.a))
	}
}

func correrBits() {
	for _, c := range casosBits {
		var tengo uint64
		switch c.fn {
		case "OnesCount8":
			tengo = uint64(bits.OnesCount8(uint8(c.x)))
		case "RotateLeft32":
			tengo = uint64(bits.RotateLeft32(uint32(c.x), c.k))
		case "RotateRight32":
			tengo = uint64(bits.RotateRight32(uint32(c.x), c.k))
		case "LeadingZeros32":
			tengo = uint64(bits.LeadingZeros32(uint32(c.x)))
		case "TrailingZeros32":
			tengo = uint64(bits.TrailingZeros32(uint32(c.x)))
		case "Len32":
			tengo = uint64(bits.Len32(uint32(c.x)))
		}
		check("math/bits", c.fn, strconv.FormatUint(c.esperado, 10), strconv.FormatUint(tengo, 10))
	}
}

func correrHash() {
	for _, c := range casosHash {
		h32 := fnv.New32a()
		h32.Write([]byte(c.texto))
		h64 := fnv.New64a()
		h64.Write([]byte(c.texto))
		check("hash/fnv", "Sum32 "+strconv.Quote(c.texto), strconv.FormatUint(uint64(c.e32), 10), strconv.FormatUint(uint64(h32.Sum32()), 10))
		check("hash/fnv", "Sum64 "+strconv.Quote(c.texto), strconv.FormatUint(c.e64, 10), strconv.FormatUint(h64.Sum64(), 10))
		check("encoding/hex", "Encode "+strconv.Quote(c.texto), c.hex, hex.EncodeToString([]byte(c.texto)))
	}
}

func correrBinary() {
	for _, c := range casosBinary {
		var u16, u32 uint64
		u64 := ""
		if len(c.bytes) >= 2 {
			u16 = uint64(binary.LittleEndian.Uint16(c.bytes))
		}
		if len(c.bytes) >= 4 {
			u32 = uint64(binary.LittleEndian.Uint32(c.bytes))
		}
		if len(c.bytes) >= 8 {
			u64 = strconv.FormatUint(binary.LittleEndian.Uint64(c.bytes), 10)
		}
		check("encoding/binary", "LittleEndian.Uint16", strconv.FormatUint(c.u16, 10), strconv.FormatUint(u16, 10))
		check("encoding/binary", "LittleEndian.Uint32", strconv.FormatUint(c.u32, 10), strconv.FormatUint(u32, 10))
		if c.u64 != "" {
			check("encoding/binary", "LittleEndian.Uint64", c.u64, u64)
		}
	}

	for _, c := range casosRead {
		r := bytes.NewReader(c.dato)
		var valor string
		switch c.tipo {
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
			valor = "bits " + hex64(bits64(float64(v)))
		case "float64":
			var v float64
			binary.Read(r, binary.LittleEndian, &v)
			valor = "bits " + hex64(bits64(v))
		}
		check("encoding/binary", "Read "+c.tipo, c.valor, valor)
	}
}

// correrPropiedades: lo que NO se puede comparar contra Go (el generador de rand es propio, ver el
// limite declarado en el paquete). Se prueba lo que importa: determinismo por semilla y rango.
func correrPropiedades() {
	// la misma semilla da la misma secuencia
	a := rand.New(rand.NewSource(12345))
	b := rand.New(rand.NewSource(12345))
	iguales := true
	for i := 0; i < 100; i++ {
		if a.Int63() != b.Int63() {
			iguales = false
			break
		}
	}
	check("math/rand", "misma semilla, misma secuencia", "true", strconv.FormatBool(iguales))

	// semillas distintas dan secuencias distintas
	c := rand.New(rand.NewSource(54321))
	d := rand.New(rand.NewSource(12345))
	distintas := false
	for i := 0; i < 100; i++ {
		if c.Int63() != d.Int63() {
			distintas = true
			break
		}
	}
	check("math/rand", "semillas distintas, secuencias distintas", "true", strconv.FormatBool(distintas))

	// Intn(n) siempre en [0, n): 1000 muestras, con n potencia de dos y n comun
	r := rand.New(rand.NewSource(7))
	fuera := 0
	var distintos [10]bool // sin mapas: el runtime de la consola no los trae
	for i := 0; i < 1000; i++ {
		v := r.Intn(10)
		if v < 0 || v >= 10 {
			fuera++
		}
		distintos[v] = true
	}
	check("math/rand", "Intn(10) en [0,10)", "0", strconv.Itoa(fuera))
	// OJO: con un arreglo, len() da 10 siempre; hay que contar los true de verdad
	cubiertos := 0
	for _, v := range distintos {
		if v {
			cubiertos++
		}
	}
	check("math/rand", "Intn(10) cubre los 10 valores", "10", strconv.Itoa(cubiertos))
	check("math/rand", "Intn(1) siempre 0", "0", strconv.Itoa(r.Intn(1)))
	check("math/rand", "Intn(256) en [0,256)", "0", strconv.Itoa(fueraDe(r, 256, 1000)))
	check("math/rand", "Int() no negativo", "true", strconv.FormatBool(r.Int() >= 0))

	// container/ring: el anillo de 5 con Move y Do
	an := ring.New(5)
	for i := 0; i < 5; i++ {
		an.Value = i
		an = an.Next()
	}
	check("container/ring", "Len", "5", strconv.Itoa(an.Len()))
	check("container/ring", "Move(2) desde el 0", "2", strconv.Itoa(an.Move(2).Value.(int)))
	check("container/ring", "Move(-1) desde el 0", "4", strconv.Itoa(an.Move(-1).Value.(int)))
	suma := 0
	an.Do(func(v interface{}) {
		if n, ok := v.(int); ok {
			suma += n
		}
	})
	check("container/ring", "Do suma 0..4", "10", strconv.Itoa(suma))
}

// fueraDe cuenta cuantas de n muestras de Intn(k) se salen del rango.
func fueraDe(r *rand.Rand, k, n int) int {
	fuera := 0
	for i := 0; i < n; i++ {
		v := r.Intn(k)
		if v < 0 || v >= k {
			fuera++
		}
	}
	return fuera
}

// medirCostoFloat64 pone numero al costo de la aritmetica de punto flotante en la consola, que es un
// dato que el motor necesita (cuanto cuesta un frame con cuentas). Se mide con el reloj de KOS
// (shim) y el resultado acumulado se imprime: si no se usara, el compilador podria borrar el bucle
// entero y la medicion seria mentira. Los operandos van en VARIABLES y realimentan el resultado por
// la misma razon (un bucle con constantes se pliega y da un numero falsamente bajo: la primera
// version de esta medicion dio 10 ms por 1e6 operaciones, que es imposible en software).
func medirCostoFloat64() {
	a := 1.0000001
	b := 1e-9
	x := 0.5
	suma := 0.0
	t0 := d2c_us()
	for i := 0; i < 1000000; i++ {
		x = x*a + b
		if x > 1e6 {
			x = 0.5
		}
	}
	suma += x
	t1 := d2c_us()
	println("STD-TEST: costo float64: 1e6 (mul+suma)", strconv.FormatUint((t1-t0)/1000, 10), "ms")

	var f32 float32 = 0.5
	a32 := float32(1.0000001)
	b32 := float32(b)
	t0 = d2c_us()
	for i := 0; i < 1000000; i++ {
		f32 = f32*a32 + b32
		if f32 > 1e6 {
			f32 = 0.5
		}
	}
	suma += float64(f32)
	t1 = d2c_us()
	println("STD-TEST: costo float32: 1e6 (mul+suma)", strconv.FormatUint((t1-t0)/1000, 10), "ms")

	t0 = d2c_us()
	for i := 0; i < 100000; i++ {
		suma += math.Sqrt(x + float64(i))
	}
	t1 = d2c_us()
	println("STD-TEST: costo float64: 1e5 sqrt", strconv.FormatUint((t1-t0)/1000, 10), "ms")

	t0 = d2c_us()
	for i := 0; i < 10000; i++ {
		suma += math.Exp(0.5) + math.Log(2.5)
	}
	t1 = d2c_us()
	println("STD-TEST: costo float64: 1e4 (exp+log)", strconv.FormatUint((t1-t0)/1000, 10), "ms")

	t0 = d2c_us()
	for i := 0; i < 10000; i++ {
		suma += math.Sin(0.5) + math.Cos(0.5)
	}
	t1 = d2c_us()
	println("STD-TEST: costo float64: 1e4 (sin+cos)", strconv.FormatUint((t1-t0)/1000, 10), "ms")

	t0 = d2c_us()
	for i := 0; i < 10000; i++ {
		suma += math.Atan2(1.0, 2.0) + math.Pow(2.0, 3.0)
	}
	t1 = d2c_us()
	println("STD-TEST: costo float64: 1e4 (atan2+pow)", strconv.FormatUint((t1-t0)/1000, 10), "ms")

	println("STD-TEST: costo float64 (control, no tiene que ser 0):", strconv.FormatUint(bits64(suma)&1, 10))
}

func correrErrores() {
	base := errors.New("base")
	e := fmt.Errorf("contexto: %w (codigo %d)", base, 7)
	check("errors", "Errorf con %w", "contexto: base (codigo 7)", e.Error())
	check("errors", "Is", "true", strconv.FormatBool(errors.Is(e, base)))
	check("errors", "Unwrap", "base", errors.Unwrap(e).Error())
	check("errors", "Join", "a\nb", errors.Join(errors.New("a"), nil, errors.New("b")).Error())

	// La FIRMA del metodo decide si un tipo implementa una interfaz (DC-002.2f): un envoltorio de
	// VARIOS %w tiene Unwrap() []error, asi que implementa interface{ Unwrap() []error } y NO
	// interface{ Unwrap() error }. Los esperados son los de Go real.
	//
	// Este caso es el que caza al mutante del runtime: si implements_interface vuelve a mirar solo el
	// nombre del metodo, la asercion de abajo dice true y el corpus se pone rojo.
	doble := fmt.Errorf("contexto: %w y %w", base, errors.New("otro"))
	_, varias := doble.(interface{ Unwrap() []error })
	_, solo := doble.(interface{ Unwrap() error })
	check("errors", "aserción Unwrap() []error sobre un envoltorio de varios", "true", strconv.FormatBool(varias))
	check("errors", "aserción Unwrap() error sobre un envoltorio de varios", "false", strconv.FormatBool(solo))
	check("errors", "Is con dos envueltos", "true", strconv.FormatBool(errors.Is(doble, base)))
}
