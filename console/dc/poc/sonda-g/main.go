//go:build dreamcast

// Sonda de DC-002.2f, segunda parte: el caso del corpus (errors.Is con el envoltorio de %w), paso a
// paso, con la stdlib de la CONSOLA (fmt, errors y strconv de console/dc/std).
//
// Con la comparacion de firmas puesta, la sonda-f mostro que la asercion de firma equivocada ya no
// pasa (F13 false) y que la buena sigue pasando (F16 true). Pero el corpus sigue en 589/1: hay que
// medir DONDE se corta la cadena en vez de suponerlo.
package main

import (
	"errors"
	"fmt"
	"strconv"
	"unsafe"
)

//extern d2c_serial_init
func d2c_serial_init()

type goString struct {
	data *byte
	len  uintptr
}

type imethod struct {
	name    *goString
	pkgPath *goString
	typ     uintptr
}

type method struct {
	name    *goString
	pkgPath *goString
	mtyp    uintptr
	typ     uintptr
	tfn     uintptr
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

func tipoDe(v interface{}) uintptr {
	return *(*uintptr)(unsafe.Pointer(&v))
}

func texto(s *goString) string {
	if s == nil {
		return "<nil>"
	}
	if s.len > 200 {
		return "<len raro " + strconv.FormatUint(uint64(s.len), 10) + ">"
	}
	b := make([]byte, int(s.len))
	src := unsafe.Slice(s.data, s.len)
	for i := uintptr(0); i < s.len; i++ {
		b[i] = src[i]
	}
	return string(b)
}

// palabras de un descriptor: para mirar la estructura de un itab sin suponer el layout
func palabra(base uintptr, i uintptr) uintptr {
	if base == 0 {
		return 0
	}
	return *(*uintptr)(unsafe.Pointer(base + i*4))
}

// itabYDato devuelve los dos campos de una variable de interfaz no vacia: {itab, dato}
func itabYDato(p unsafe.Pointer) (uintptr, uintptr) {
	return *(*uintptr)(p), *(*uintptr)(unsafe.Add(p, 4))
}

func hex(x uintptr) string { return strconv.FormatUint(uint64(x), 16) }

func reflexion(t uintptr) string {
	if t == 0 {
		return "<0>"
	}
	return texto((*typeDesc)(unsafe.Pointer(t)).reflection)
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

// imetodoDelItab recibe el PUNTERO a una variable de interfaz no vacia (el itab es su primer campo y
// el Itab empieza con el descriptor de la interfaz, justo antes de methods[0]).
func imetodoDelItab(p unsafe.Pointer, i int) *imethod {
	tab := *(*uintptr)(p)
	if tab == 0 {
		return nil
	}
	inter := *(*uintptr)(unsafe.Pointer(tab - 4))
	it := (*interfaceType)(unsafe.Pointer(inter))
	if it.methods == nil || uintptr(i) >= it.count {
		return nil
	}
	return (*imethod)(unsafe.Pointer(uintptr(unsafe.Pointer(it.methods)) + uintptr(i)*unsafe.Sizeof(*it.methods)))
}

func main() {
	d2c_serial_init()

	println("SONDA-G arranca: el caso del corpus (errors.Is + %w) paso a paso")

	base := errors.New("base")
	envuelto := fmt.Errorf("contexto: %w", base)

	tEnv := tipoDe(envuelto)
	println("G0 tipo de envuelto =", hex(tEnv), reflexion(tEnv))

	m := mmetodo(tEnv, "Unwrap")
	if m == nil {
		println("G1 no encontre el metodo Unwrap en el tipo de envuelto")
	} else {
		println("G1 method.Unwrap: mtyp=", hex(m.mtyp), reflexion(m.mtyp))
		println("G2   typ=", hex(m.typ), reflexion(m.typ))
	}

	// la asercion que usa errors.Unwrap (y la primera rama de Is)
	if x, ok := envuelto.(interface{ Unwrap() error }); ok {
		println("G3 asercion Unwrap() error -> ok=true")
		u := x.Unwrap()
		if u == nil {
			println("G4   Unwrap() devolvio nil")
		} else {
			println("G4   Unwrap() ->", hex(tipoDe(u)), reflexion(tipoDe(u)), " igual a base?", strconv.FormatBool(u == base))
		}
		if m != nil {
			if im := imetodoDelItab(unsafe.Pointer(&x), 0); im != nil {
				println("G5 imethod.typ=", hex(im.typ), reflexion(im.typ))
				println("G6 imethod.typ == mtyp ?", strconv.FormatBool(im.typ == m.mtyp))
			}
		}
	} else {
		println("G3 asercion Unwrap() error -> ok=false  <-- aca se corta la cadena")
	}

	// la interfaz de la otra rama de Is
	if _, ok := envuelto.(interface{ Unwrap() []error }); ok {
		println("G7 asercion Unwrap() []error -> ok=true (no deberia: es *wrapError)")
	} else {
		println("G7 asercion Unwrap() []error -> ok=false")
	}

	println("G8 errors.Unwrap(envuelto) == nil ?", strconv.FormatBool(errors.Unwrap(envuelto) == nil))
	println("G9 errors.Is(envuelto, base) =", strconv.FormatBool(errors.Is(envuelto, base)))
	println("G10 errors.Is(envuelto, envuelto) =", strconv.FormatBool(errors.Is(envuelto, envuelto)))

	// --- la comparacion que falla: donde esta la diferencia ---
	e2 := errors.Unwrap(envuelto)
	println("G12 e2 == base ?", strconv.FormatBool(e2 == base))
	i1a, d1a := itabYDato(unsafe.Pointer(&e2))
	i2a, d2a := itabYDato(unsafe.Pointer(&base))
	println("G13 itab(e2)=", hex(i1a), " data(e2)=", hex(d1a))
	println("G14 itab(base)=", hex(i2a), " data(base)=", hex(d2a))
	println("G15 mismos datos?", strconv.FormatBool(d1a == d2a), " mismo itab?", strconv.FormatBool(i1a == i2a))
	println("G16 itab(e2) palabras 0..3:", hex(palabra(i1a, 0)), hex(palabra(i1a, 1)), hex(palabra(i1a, 2)), hex(palabra(i1a, 3)))
	println("G17 itab(base) palabras 0..3:", hex(palabra(i2a, 0)), hex(palabra(i2a, 1)), hex(palabra(i2a, 2)), hex(palabra(i2a, 3)))
	println("G18 reflexion palabra1(e2)=", reflexion(palabra(i1a, 1)))
	println("G19 reflexion palabra1(base)=", reflexion(palabra(i2a, 1)))

	println("G11 fin de la sonda")

	for {
	}
}
