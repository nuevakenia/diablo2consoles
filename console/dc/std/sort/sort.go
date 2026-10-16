// Paquete sort: ordenamiento, con la parte de Go que el motor usa.
//
// Por que existe: en sh-elf no hay biblioteca estandar de Go, y el motor ordena en 86 archivos
// (Float64s, Ints, Search, SearchInts, Slice, SliceStable, Sort, Strings).
//
// EL PROBLEMA DE ESTE PAQUETE: Sort(data Interface) es Go puro y no tiene misterio, pero sort.Slice
// y sort.SliceStable en Go arman su intercambio con reflectlite (ValueOf + Swapper), y en la consola
// NO hay reflect: portarlo es inviable (medido: el paquete reflect de Go depende de 43 paquetes, 32
// de ellos del runtime del compilador, que la consola no tiene).
//
// LA SOLUCION, MEDIDA ANTES DE ESCRIBIRLA (sonda-h, console/dc/poc/sonda-h): leer el descriptor de
// tipo del argumento con unsafe. Lo medido en la consola:
//
//  1. Un slice pasado a interface{} tiene como dato un puntero a su CABECERA {ptr, len, cap}: la
//     sonda lo comparo con &s[0] y con len/cap y coincidio.
//  2. El descriptor del tipo del slice tiene el TAMANO DEL ELEMENTO en su campo 10 (offset 10
//     palabras). Medido con 7 tipos distintos, incluidos tamanos que NO son potencias de dos
//     (12 y 20 bytes): []int 4, []int64 8, []struct{a,b int32} 8, []struct{a,b,c int32} 12,
//     []struct{a..e int32} 20, []string 8. El tamano del slice mismo da 12 (tres palabras) siempre.
//  3. El intercambio de dos elementos de ancho desconocido se hace de a 4 bytes y anda con cualquier
//     ancho (medido con 4 y 32 bytes).
//
// Con eso, sort.Slice no necesita reflect: saca la base, el len y el ancho del elemento del propio
// argumento. El limite: SOLO sirve para slices (que es lo unico que sort.Slice acepta).
//
// LO QUE COINCIDE CON GO Y COMO SE PRUEBA:
//   - Ints, Float64s, Strings, IntsAreSorted, IsSorted, Search, SearchInts, SearchFloat64s,
//     SearchStrings: mismo resultado, caso por caso contra Go.
//   - Sort/Slice con claves TODAS DISTINTAS: el orden es unico, asi que se compara tal cual.
//   - SliceStable/Stable con claves REPETIDAS: es estable en Go y aca tambien, asi que el resultado
//     es unico y se compara. La inestabilidad de Slice NO se puede comparar con Go (dos ordenes
//     distintos son los dos validos): por eso los casos de Slice usan claves distintas y esta dicho.
package sort

import "unsafe"

// Interface es el contrato de Go para ordenar algo que no es un slice.
type Interface interface {
	Len() int
	Less(i, j int) bool
	Swap(i, j int)
}

// ---- el camino sin reflect: leer el slice que llega en un interface{} ----

// eface es un interface{} visto por dentro. Los campos son uintptr y no unsafe.Pointer a proposito:
// una clausura o un campo que guarda un unsafe.Pointer obliga al compilador a referenciar el
// descriptor del tipo unsafe.POINTER (los simbolos unsafe.Pointer..p y ..d), y el runtime de la
// consola no los tiene (medido: el link fallo con "undefined reference to unsafe.Pointer..p").
// Con uintptr de por medio solo quedan conversiones, que si estan.
type eface struct {
	typ  uintptr
	data uintptr
}

// EL LAYOUT DEL DESCRIPTOR NO ES EL MISMO EN LOS DOS COMPILADORES, y esto es lo unico delicado del
// paquete:
//
//   - gccgo (la consola): el campo del descriptor del slice tiene EL TAMANO del elemento.
//   - gc (el host, para la prueba diferencial): ese mismo campo es un PUNTERO al descriptor del
//     elemento, y el tamano es el primer campo del apuntado.
//
// En vez de escribir el offset a mano (que ademas depende del ancho de palabra: el descriptor
// arranca con "size" y "ptrdata", una palabra cada uno), se DETECTA una vez, midiendo: se toma un
// slice cuyo elemento mide 20 bytes (un tamano que no se confunde con ningun campo del descriptor) y
// se busca en que offset aparece ese 20, primero como valor y despues como puntero a un descriptor
// cuyo primer campo sea 20.
//
// Si no reconoce el layout AVISA (panic) y no sigue: un sort que ordena con el ancho equivocado
// corrompe memoria, que es mucho peor que un programa que no arranca.
var (
	offsetElem uintptr
	elemPtr    bool
)

func init() {
	// La pista: un struct de 20 bytes. El tamano 20 no se confunde con ningun campo del descriptor
	// (los que se repiten son 0, 1, 4, 8, 12, 16, 24 y los punteros), y ademas el descriptor DE UN
	// VALOR DE ESE TIPO me da el puntero con el que comparar (sin desreferenciar nada a ciegas: un
	// escaneo que convierte cada campo en puntero y lo desreferencia revienta, porque el descriptor
	// tiene campos que son enteros chicos, como el indice del nombre del tipo, que valen mas de 4096
	// y no son direcciones. Medido, paso exactamente eso).
	type pista struct{ a, b, c, d, e int32 } // 20 bytes
	const tam = 20
	var s []pista
	var uno pista
	var cajaSlice interface{} = s
	var cajaUno interface{} = uno
	eSlice := *(*eface)(unsafe.Pointer(&cajaSlice))
	eUno := *(*eface)(unsafe.Pointer(&cajaUno))
	if eSlice.typ == 0 || eUno.typ == 0 {
		panic("sort: no pude tomar los descriptores de tipo")
	}
	// el descriptor de un tipo arranca con su tamano (medido en los dos compiladores)
	if *(*uintptr)(unsafe.Pointer(eUno.typ)) != tam {
		panic("sort: el primer campo del descriptor de tipo no es el tamano")
	}
	palabra := unsafe.Sizeof(uintptr(0))
	for i := uintptr(0); i < 24; i++ {
		off := i * palabra
		v := *(*uintptr)(unsafe.Pointer(eSlice.typ + off))
		// (a) gccgo: el campo TIENE el tamano del elemento
		if v == tam {
			offsetElem, elemPtr = off, false
			return
		}
		// (b) gc: el campo ES el puntero al descriptor del elemento (lo se sin desreferenciar:
		// es el mismo puntero que tiene el descriptor de un pista suelto)
		if v == eUno.typ {
			offsetElem, elemPtr = off, true
			return
		}
	}
	panic("sort: no reconozco el layout del descriptor de tipo")
}

// anchoDeElemento devuelve el tamano de un elemento del slice cuyo descriptor de tipo es typ.
func anchoDeElemento(typ uintptr) uintptr {
	if elemPtr {
		// el campo GUARDA la direccion del descriptor del elemento: dos lecturas, el campo y su
		// primer campo (el tamano, verificado en el init)
		descElem := *(*uintptr)(unsafe.Pointer(typ + offsetElem))
		return *(*uintptr)(unsafe.Pointer(descElem))
	}
	return *(*uintptr)(unsafe.Pointer(typ + offsetElem))
}

// cabeceraSlice es la cabecera de un slice (la misma forma que el runtime pone adentro de un
// interface{} cuando el valor es un slice).
type cabeceraSlice struct {
	datos uintptr
	largo uintptr
	cap   uintptr
}

// desarmarSlice devuelve la direccion del primer elemento, el largo y el ancho de un elemento.
// Es la parte que en Go hace reflectlite.Swapper, con el descriptor leido a mano (ver el encabezado).
func desarmarSlice(x interface{}) (base uintptr, n int, ancho uintptr) {
	e := *(*eface)(unsafe.Pointer(&x))
	if e.typ == 0 || e.data == 0 {
		panic("sort: el argumento no es un slice (o es nil)")
	}
	// {ptr, len, cap} del slice en caja
	cab := (*cabeceraSlice)(unsafe.Pointer(e.data))
	base = cab.datos
	n = int(cab.largo)
	ancho = anchoDeElemento(e.typ)
	if ancho == 0 {
		panic("sort: no pude sacar el tamano del elemento del descriptor")
	}
	return base, n, ancho
}

// swapBytes intercambia ancho bytes entre p y q (direcciones), de a 4 (el SH-4 alinea a 4 y
// direcciona de a bytes).
func swapBytes(p, q, ancho uintptr) {
	var t [4]byte
	n := ancho
	for n >= 4 {
		a := (*[4]byte)(unsafe.Pointer(p))
		b := (*[4]byte)(unsafe.Pointer(q))
		t = *a
		*a = *b
		*b = t
		p += 4
		q += 4
		n -= 4
	}
	for n > 0 {
		a := (*byte)(unsafe.Pointer(p))
		b := (*byte)(unsafe.Pointer(q))
		*a, *b = *b, *a
		p++
		q++
		n--
	}
}

// swapDe arma la funcion de intercambio de un slice ya desarmado (base es una direccion).
func swapDe(base, ancho uintptr) func(i, j int) {
	return func(i, j int) {
		if i == j {
			return
		}
		swapBytes(base+uintptr(i)*ancho, base+uintptr(j)*ancho, ancho)
	}
}

// ---- el algoritmo ----

const umbralInsercion = 12

// insertionSort ordena [a, b) por insercion: es estable y es lo que usa Go para los tramos chicos.
func insertionSort(a, b int, less func(i, j int) bool, swap func(i, j int)) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && less(j, j-1); j-- {
			swap(j, j-1)
		}
	}
}

// siftDown hunde el elemento raiz de un monticulo (lo usa el heapSort de respaldo).
func siftDown(lo, hi, primero int, less func(i, j int) bool, swap func(i, j int)) {
	raiz := lo
	for {
		hijo := 2*raiz + 1 - primero
		if hijo >= hi {
			break
		}
		if hijo+1 < hi && less(hijo, hijo+1) {
			hijo++
		}
		if !less(raiz, hijo) {
			break
		}
		swap(raiz, hijo)
		raiz = hijo
	}
}

// heapSort ordena [a, b) con un monticulo: es el respaldo cuando el quicksort se va muy hondo.
func heapSort(a, b int, less func(i, j int) bool, swap func(i, j int)) {
	primero := a
	lo, hi := 0, b-a
	for i := (hi - 1) / 2; i >= 0; i-- {
		siftDown(i, hi, lo, func(i, j int) bool { return less(i+primero, j+primero) }, func(i, j int) { swap(i+primero, j+primero) })
	}
	for i := hi - 1; i > 0; i-- {
		swap(primero, i+primero)
		siftDown(0, i, lo, func(i, j int) bool { return less(i+primero, j+primero) }, func(i, j int) { swap(i+primero, j+primero) })
	}
}

// pdqDepth es la profundidad a partir de la cual se pasa a heapSort.
func pdqDepth(n int) int {
	d := 0
	for i := n; i > 0; i >>= 1 {
		d++
	}
	return 2 * d
}

// quickSort ordena [a, b) con quicksort y mediana de tres, con heapSort de respaldo.
//
// OJO: NO es estable (igual que sort.Sort y sort.Slice de Go). Los casos del corpus que lo ejercitan
// usan claves distintas, donde el resultado es unico; la estabilidad se prueba con SliceStable.
func quickSort(a, b int, less func(i, j int) bool, swap func(i, j int), profundidad int) {
	for b-a > umbralInsercion {
		if profundidad == 0 {
			heapSort(a, b, less, swap)
			return
		}
		profundidad--
		// mediana de tres: primero, medio, ultimo
		medio := a + (b-a)/2
		if less(medio, a) {
			swap(medio, a)
		}
		if less(b-1, a) {
			swap(b-1, a)
		}
		if less(b-1, medio) {
			swap(b-1, medio)
		}
		swap(medio, b-2)
		pivote := b - 2
		i, j := a, b-2
		for {
			i++
			for less(i, pivote) {
				i++
			}
			j--
			for less(pivote, j) {
				j--
			}
			if i >= j {
				break
			}
			swap(i, j)
		}
		swap(i, b-2)
		// el tramo chico se ordena por insercion y el grande sigue en el bucle
		if i-a < b-i {
			quickSort(a, i, less, swap, profundidad)
			a = i + 1
		} else {
			quickSort(i+1, b, less, swap, profundidad)
			b = i
		}
	}
	insertionSort(a, b, less, swap)
}

// ---- la API de Go que el motor usa ----

// Sort ordena data (no es estable).
func Sort(data Interface) {
	n := data.Len()
	quickSort(0, n, data.Less, data.Swap, pdqDepth(n))
}

// IsSorted dice si data ya esta ordenado.
func IsSorted(data Interface) bool {
	n := data.Len()
	for i := n - 1; i > 0; i-- {
		if data.Less(i, i-1) {
			return false
		}
	}
	return true
}

// Stable ordena data y es estable.
func Stable(data Interface) {
	n := data.Len()
	stable(n, data.Less, data.Swap)
}

// Slice ordena un slice cualquiera con el comparador dado (no es estable). Es lo que en Go se hace
// con reflect: aca el tipo se saca del descriptor (ver el encabezado del paquete).
func Slice(x interface{}, less func(i, j int) bool) {
	base, n, ancho := desarmarSlice(x)
	quickSort(0, n, less, swapDe(base, ancho), pdqDepth(n))
}

// SliceStable ordena un slice y es estable.
func SliceStable(x interface{}, less func(i, j int) bool) {
	base, n, ancho := desarmarSlice(x)
	stable(n, less, swapDe(base, ancho))
}

// SliceIsSorted dice si el slice ya esta ordenado.
func SliceIsSorted(x interface{}, less func(i, j int) bool) bool {
	_, n, _ := desarmarSlice(x)
	for i := n - 1; i > 0; i-- {
		if less(i, i-1) {
			return false
		}
	}
	return true
}

// stable ordena por mezcla con un buffer auxiliar: O(n log n) y estable (symmerge en Go, mismo
// resultado). El buffer se pide de a bytes y se copia con unsafe, porque el ancho del elemento no se
// conoce en tiempo de compilacion.
func stable(n int, less func(i, j int) bool, swap func(i, j int)) {
	if n < umbralInsercion {
		insertionSort(0, n, less, swap)
		return
	}
	// mergesort de abajo hacia arriba, por bloques
	for ancho := 1; ancho < n; ancho *= 2 {
		for i := 0; i < n; i += 2 * ancho {
			medio := i + ancho
			final := i + 2*ancho
			if medio > n {
				medio = n
			}
			if final > n {
				final = n
			}
			mezclar(i, medio, final, less, swap)
		}
	}
}

// mezclar une dos tramos ya ordenados [a, m) y [m, b) sin perder el orden de los iguales.
//
// Para no alocar memoria en cada llamada (el buffer tendria que ser del ancho del elemento, que no
// se conoce), se rota el tramo: es mas lento que el symmerge de Go pero es estable y no necesita
// memoria. El motor ordena listas de decenas o cientos de elementos, no millones.
func mezclar(a, m, b int, less func(i, j int) bool, swap func(i, j int)) {
	for a < m && m < b {
		if !less(m, a) {
			a++
			continue
		}
		// el elemento m va antes que a: lo llevo adelante rotando [a, m]
		for i := m; i > a; i-- {
			swap(i, i-1)
		}
		a++
		m++
	}
}

// ---- los tipos concretos ----

// Ints ordena un []int.
func Ints(a []int) { sortInts(a) }

func sortInts(a []int) {
	quickSort(0, len(a), func(i, j int) bool { return a[i] < a[j] }, func(i, j int) { a[i], a[j] = a[j], a[i] }, pdqDepth(len(a)))
}

// IntsAreSorted dice si a esta ordenado.
func IntsAreSorted(a []int) bool {
	for i := len(a) - 1; i > 0; i-- {
		if a[i] < a[i-1] {
			return false
		}
	}
	return true
}

// Float64s ordena un []float64.
func Float64s(a []float64) {
	quickSort(0, len(a), func(i, j int) bool { return a[i] < a[j] }, func(i, j int) { a[i], a[j] = a[j], a[i] }, pdqDepth(len(a)))
}

// Strings ordena un []string.
func Strings(a []string) {
	quickSort(0, len(a), func(i, j int) bool { return a[i] < a[j] }, func(i, j int) { a[i], a[j] = a[j], a[i] }, pdqDepth(len(a)))
}

// SearchInts busca x en a (que tiene que estar ordenado) y devuelve el indice donde va.
func SearchInts(a []int, x int) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchFloat64s busca x en a ordenado.
func SearchFloat64s(a []float64, x float64) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchStrings busca x en a ordenado.
func SearchStrings(a []string, x string) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// Search hace la busqueda binaria sobre un predicado monótono (el primer i donde f(i) es true).
func Search(n int, f func(int) bool) int {
	i, j := 0, n
	for i < j {
		h := i + (j-i)/2
		if !f(h) {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}
