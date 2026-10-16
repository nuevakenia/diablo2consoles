//go:build dreamcast

// Sonda de DC-002.2f: ANTES de tocar el runtime, medir si la FIRMA de un metodo se puede comparar.
//
// El defecto medido (sonda-e, E29-E32): implements_interface (libgodc/runtime/interface_dreamcast.c)
// compara el nombre del metodo y nada mas, asi que *wrapErrors (con Unwrap() []error) "implementa"
// interface{ Unwrap() error } y la llamada devuelve basura.
//
// El arreglo obvio es comparar tambien el tipo del metodo (lhsMethod->__typ contra rhsMethod->__mtyp o
// rhsMethod->__typ). Antes de escribirlo hay que medir dos cosas, porque una comparacion mal hecha
// ROMPE los 589 casos que hoy pasan:
//
//   1. Cual de los dos campos del descriptor del metodo (__mtyp o __typ) es el que corresponde al
//      __typ del metodo de la interfaz.
//   2. Si ese descriptor es el MISMO puntero cuando el tipo lo define un paquete y el metodo lo pide la
//      interfaz de OTRO paquete. Si no lo fuera, comparar por puntero seria un falso negativo en
//      todas las aserciones entre paquetes (que son las que hacen andar el corpus).
//
// No hay dependencia nueva: los descriptores se leen con unsafe, con los layouts que libgodc ya
// verifica con _Static_assert en runtime/type_descriptors.h.
package main

import (
	"strconv"
	"unsafe"

	"github.com/OpenDiablo2/OpenDiablo2/console/dc/poc/sonda-f/paq1"
)

type envuelveUno interface{ Unwrap() error }

type envuelveMuchos interface{ Unwrap() []error }

//extern d2c_serial_init
func d2c_serial_init()

// --- los descriptores de tipo, como los declara libgodc (32 bits) ---

type goString struct {
	data *byte
	len  uintptr
}

type imethod struct {
	name    *goString
	pkgPath *goString
	typ     uintptr // descriptor del tipo del metodo (sin receptor)
}

type method struct {
	name    *goString
	pkgPath *goString
	mtyp    uintptr // "method type"
	typ     uintptr // "function type"
	tfn     uintptr // puntero a la funcion
}

type uncommonType struct {
	name    *goString
	pkgPath *goString
	methods *method
	count   uintptr
	cap     uintptr
}

type typeDesc struct {
	size       uintptr
	ptrdata    uintptr
	hash       uint32
	tflag      uint8
	align      uint8
	fieldAlign uint8
	code       uint8
	equalfn    uintptr
	gcdata     uintptr
	reflection *goString
	uncommon   *uncommonType
	ptrToThis  uintptr
}

type interfaceType struct {
	common  typeDesc
	methods *imethod
	count   uintptr
	cap     uintptr
}

// --- ayudantes ---

// tipoDe devuelve el descriptor de tipo de un valor metido en interface{} (Eface = {tipo, dato}).
func tipoDe(v interface{}) uintptr {
	return *(*uintptr)(unsafe.Pointer(&v))
}

// interDeIface devuelve el descriptor de la INTERFAZ a partir de una variable de interfaz no vacia:
// el itab esta en el primer campo, y el Itab empieza con el puntero a la interfaz, justo antes de
// methods[0] (que es lo que guarda la variable).
func interDeIface(p unsafe.Pointer) uintptr {
	tab := *(*uintptr)(p)
	if tab == 0 {
		return 0
	}
	return *(*uintptr)(unsafe.Pointer(tab - 4))
}

func texto(s *goString) string {
	if s == nil {
		return "<nil>"
	}
	b := make([]byte, int(s.len))
	src := unsafe.Slice(s.data, s.len)
	for i := uintptr(0); i < s.len; i++ {
		b[i] = src[i]
	}
	return string(b)
}

func hex(x uintptr) string { return strconv.FormatUint(uint64(x), 16) }

func reflexion(t uintptr) string {
	if t == 0 {
		return "<0>"
	}
	d := (*typeDesc)(unsafe.Pointer(t))
	return texto(d.reflection)
}

func imetodo(inter uintptr, i int) *imethod {
	it := (*interfaceType)(unsafe.Pointer(inter))
	if it.methods == nil || uintptr(i) >= it.count {
		return nil
	}
	return (*imethod)(unsafe.Pointer(uintptr(unsafe.Pointer(it.methods)) + uintptr(i)*unsafe.Sizeof(*it.methods)))
}

func mmetodo(t uintptr, nombre string) *method {
	d := (*typeDesc)(unsafe.Pointer(t))
	if d.uncommon == nil {
		return nil
	}
	um := d.uncommon
	for i := uintptr(0); i < um.count; i++ {
		m := (*method)(unsafe.Pointer(uintptr(unsafe.Pointer(um.methods)) + i*unsafe.Sizeof(*um.methods)))
		if texto(m.name) == nombre {
			return m
		}
	}
	return nil
}

func main() {
	d2c_serial_init()

	println("SONDA-F arranca: la firma en el chequeo de interfaces (DC-002.2f)")

	// --- descriptores de las interfaces (por el itab de un valor que si las implementa) ---
	var vUno envuelveUno = &paq1.ConUno{}
	var vMuchos envuelveMuchos = &paq1.ConMuchos{}
	iUno := interDeIface(unsafe.Pointer(&vUno))
	iMuchos := interDeIface(unsafe.Pointer(&vMuchos))

	println("F0 ifaceUnwrapError=", hex(iUno), reflexion(iUno))
	println("F1 ifaceUnwrapSlice=", hex(iMuchos), reflexion(iMuchos))

	// --- descriptores de los tipos concretos ---
	tUno := tipoDe(&paq1.ConUno{})
	tMuchosT := tipoDe(&paq1.ConMuchos{})
	println("F2 tipoConUno=", hex(tUno), reflexion(tUno))
	println("F3 tipoConMuchos=", hex(tMuchosT), reflexion(tMuchosT))

	// --- el metodo de la interfaz y los dos campos del metodo del tipo ---
	im := imetodo(iUno, 0)
	println("F4 imethod: nombre=", texto(im.name), " pkg=", texto(im.pkgPath), " typ=", hex(im.typ), reflexion(im.typ))
	println("F5 imethod.typ reflexion=", reflexion(im.typ))

	m := mmetodo(tUno, "Unwrap")
	if m == nil {
		println("F6 no encontre Unwrap en ConUno")
	} else {
		println("F6 method ConUno: nombre=", texto(m.name), " pkg=", texto(m.pkgPath))
		println("F7   mtyp=", hex(m.mtyp), reflexion(m.mtyp))
		println("F8   typ =", hex(m.typ), reflexion(m.typ))
		println("F9   imethod.typ == mtyp ?", strconv.FormatBool(im.typ == m.mtyp))
		println("F10  imethod.typ == typ  ?", strconv.FormatBool(im.typ == m.typ))
	}

	// --- el caso MALO: el mismo nombre con otra firma ---
	mm := mmetodo(tMuchosT, "Unwrap")
	if mm == nil {
		println("F11 no encontre Unwrap en ConMuchos")
	} else {
		println("F11 method ConMuchos: mtyp=", hex(mm.mtyp), reflexion(mm.mtyp))
		println("F12   typ=", hex(mm.typ), reflexion(mm.typ))
	}

	// Ahora la asercion como la hace el compilador: interface{}(x).(envuelveUno)
	malo := interface{}(&paq1.ConMuchos{})
	if ifc, ok := malo.(envuelveUno); ok {
		println("F13 ASERCION MALA DICE ok=true (el defecto)")
		tab := *(*uintptr)(unsafe.Pointer(&ifc))
		inter2 := *(*uintptr)(unsafe.Pointer(tab - 4))
		im2 := imetodo(inter2, 0)
		if m != nil && im2 != nil {
			println("F14 en el itab del caso malo: imethod.typ=", hex(im2.typ), " igual a mtyp de ConUno?", strconv.FormatBool(im2.typ == m.mtyp))
		}
		if mm != nil {
			println("F15   imethod.typ == mtyp(ConMuchos)?", strconv.FormatBool(im2.typ == mm.mtyp),
				" == typ(ConMuchos)?", strconv.FormatBool(im2.typ == mm.typ))
		}
	} else {
		println("F13 ASERCION MALA DICE ok=false (el defecto no esta)")
	}

	// y el caso bueno, con la firma que corresponde
	bueno := interface{}(&paq1.ConMuchos{})
	if _, ok := bueno.(envuelveMuchos); ok {
		println("F16 asercion buena dice ok=true")
	} else {
		println("F16 asercion buena dice ok=false")
	}

	// --- el mismo metodo declarado en el MISMO paquete que la interfaz (control) ---
	println("F17 fin de la sonda")

	for {
	}
}
