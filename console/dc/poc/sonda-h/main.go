//go:build dreamcast

// DC-002.5, sonda H: que hace falta para escribir sort.Slice SIN reflect.
//
// sort.Slice en Go usa reflectlite (ValueOf + Swapper) y en la consola no hay reflect. La
// alternativa es leer el descriptor de tipo del argumento con unsafe. Esta sonda MIDE el terreno
// antes de escribir nada:
//
//  1. un slice pasado a interface{}: ?es su data un puntero a la cabecera {ptr, len, cap}?
//  2. el descriptor del tipo: se puede sacar el TAMANO DEL ELEMENTO de ahi? (en el layout de gc el
//     tipo del slice apunta al tipo del elemento; en el de gccgo/libgodc hay que ver donde esta)
//  3. el swap de dos elementos de tamano desconocido.
//
// Lo que se mida aca decide como se escribe sort.Slice (y queda en la evidencia).
package main

import (
	"strconv"
	"unsafe"
)

//extern d2c_serial_init
func d2c_serial_init()

//extern d2c_pantalla_init
func d2c_pantalla_init()

//extern d2c_pantalla_color
func d2c_pantalla_color(r, g, b float32)

//extern d2c_frame
func d2c_frame()

// eface es un interface{} vacio visto por dentro: el tipo y el dato.
type eface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

func partes(v interface{}) (unsafe.Pointer, unsafe.Pointer) {
	e := *(*eface)(unsafe.Pointer(&v))
	return e.typ, e.data
}

func u(v uintptr) string { return strconv.FormatUint(uint64(v), 10) }

func main() {
	d2c_serial_init()
	println("SONDA-H: descriptores de tipo para sort.Slice sin reflect")

	// ---- 1. la cabecera del slice dentro del interface ----
	s := make([]int, 3, 8)
	s[0], s[1], s[2] = 11, 22, 33
	typ, data := partes(s)
	cab := (*[3]uintptr)(data)
	println("SONDA-H: []int: cabecera=", u(cab[0]), u(cab[1]), u(cab[2]))
	println("SONDA-H: []int: &s[0]=", u(uintptr(unsafe.Pointer(&s[0]))), " len=3 cap=8")
	// si la hipotesis es cierta, cab = {&s[0], 3, 8}

	// ---- 2. el descriptor del tipo ----
	// Los primeros campos de un descriptor de tipo del estilo de gc.
	type desc struct {
		size       uintptr
		ptrdata    uintptr
		hash       uint32
		tflag      uint8
		align      uint8
		fieldAlign uint8
		kind       uint8
		equal      unsafe.Pointer
		gcdata     unsafe.Pointer
		str        int32
		ptrToThis  int32
	}
	d := (*desc)(typ)
	println("SONDA-H: []int: size=", u(d.size), " kind=", u(uintptr(d.kind)), " align=", u(uintptr(d.align)), " str=", strconv.FormatInt(int64(d.str), 10))

	// comparo el descriptor del []int con el del []int64 y el de una estructura: el elemento tiene
	// que estar en algun campo (probando los primeros 24 uintptr del descriptor).
	s64 := []int64{1, 2, 3}
	typ64, _ := partes(s64)
	type par struct{ a, b int32 }
	sp := []par{{1, 2}}
	typp, _ := partes(sp)
	println("SONDA-H: primeros 24 campos del descriptor, []int | []int64 | []par(8 bytes)")
	for i := 0; i < 24; i++ {
		a := *(*uintptr)(unsafe.Pointer(uintptr(typ) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		b := *(*uintptr)(unsafe.Pointer(uintptr(typ64) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		c := *(*uintptr)(unsafe.Pointer(uintptr(typp) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		println("SONDA-H: campo", u(uintptr(i)), ":", u(a), u(b), u(c))
	}

	// ---- 2b. el campo 10 con tamanos que NO son potencias de dos ----
	// Con 4 y 8 el campo 10 podria ser cualquier cosa que de una potencia de dos; con 12 y 20 no.
	type tres struct{ a, b, c int32 }        // 12 bytes
	type cinco struct{ a, b, c, d, e int32 } // 20 bytes
	type suelto struct{ a int32 }            // 4 bytes
	tresT, _ := partes([]tres{{}})
	cincoT, _ := partes([]cinco{{}})
	sueltoT, _ := partes([]suelto{{}})
	stringT, _ := partes([]string{""})
	casos := []struct {
		nombre string
		typ    unsafe.Pointer
		espera uintptr
	}{
		{"[]int (4)", typ, 4},
		{"[]int64 (8)", typ64, 8},
		{"[]par (8)", typp, 8},
		{"[]tres (12)", tresT, 12},
		{"[]cinco (20)", cincoT, 20},
		{"[]suelto (4)", sueltoT, 4},
		{"[]string (8 en 32 bits)", stringT, 8},
	}
	for _, c := range casos {
		campo10 := *(*uintptr)(unsafe.Pointer(uintptr(c.typ) + 10*unsafe.Sizeof(uintptr(0))))
		tamSlice := *(*uintptr)(unsafe.Pointer(uintptr(c.typ)))
		coincide := "si"
		if campo10 != c.espera {
			coincide = "NO"
		}
		println("SONDA-H: campo10 de", c.nombre, "=", u(campo10), "esperaba", u(c.espera), "coincide:", coincide, " tam del slice=", u(tamSlice))
	}

	// ---- 3. el swap de dos elementos de tamano desconocido ----
	arr := []int{1, 2, 3, 4}
	base := uintptr(unsafe.Pointer(&arr[0]))
	ancho := unsafe.Sizeof(arr[0])
	swapBytes(unsafe.Pointer(base), unsafe.Pointer(base+ancho), ancho)
	println("SONDA-H: swap de dos int (4 bytes): 1,2 ->", u(uintptr(arr[0])), u(uintptr(arr[1])))

	// 32 bytes, para ver que el swap sirve con cualquier ancho
	var x, y [32]byte
	x[0], y[0] = 0xaa, 0xbb
	swapBytes(unsafe.Pointer(&x), unsafe.Pointer(&y), 32)
	println("SONDA-H: swap de 32 bytes: x[0]=", u(uintptr(x[0])), " y[0]=", u(uintptr(y[0])), "(tiene que dar 187 170)")

	d2c_pantalla_init()
	d2c_pantalla_color(0.0, 0.3, 0.6)
	for {
		d2c_frame()
	}
}

// swapBytes intercambia ancho bytes entre p y q, de a 4 (el SH-4 direcciona de a bytes y alinea a 4).
func swapBytes(p, q unsafe.Pointer, ancho uintptr) {
	var t [4]byte
	n := ancho
	for n >= 4 {
		a := (*[4]byte)(p)
		b := (*[4]byte)(q)
		t = *a
		*a = *b
		*b = t
		p = unsafe.Pointer(uintptr(p) + 4)
		q = unsafe.Pointer(uintptr(q) + 4)
		n -= 4
	}
	for n > 0 {
		a := (*byte)(p)
		b := (*byte)(q)
		*a, *b = *b, *a
		p = unsafe.Pointer(uintptr(p) + 1)
		q = unsafe.Pointer(uintptr(q) + 1)
		n--
	}
}
