//go:build dreamcast

// DC-002.23: DONDE SE ROMPE EL ERROR DEL MOTOR.
//
// La tarjeta anterior (DC-002.22) cerro con un salto a una direccion basura (`PC 0000000b`) en el cuadro 0
// del motor, pintando el texto de su cuadro de error, y dejo abierto de que lado estaba: el itab del metodo
// `PrintAt` del renderer o el estado del `GlyphPrinter`. La medicion de esta tarjeta, con los registros del
// SH-4 que Flycast imprime en la excepcion y el desensamblado del `prog.elf` (sh-elf-gdb / objdump), dice
// que NO es ninguno de los dos: la instruccion que se cae es `jsr @r0` en `d2app.App.updateInitError`
// (app.go:718), con `r0 = *(r1+4)`, `r1 = *(a+156)` = el **itab del `error` `a.errorMessage`**, y `r0 = 0xb`.
// O sea: el motor ni llega a `DrawTextf`; se cae evaluando el ARGUMENTO (`a.errorMessage.Error()`), sobre una
// interfaz `error` cuyo itab no es una tabla de metodos valida.
//
// Este POC mide ESO, sin el motor: las formas en las que el motor se topa con un `error` (y en particular la
// de `d2asset.NewAssetManager`, que es la que llena `a.errorMessage`: un multi-retorno `(*T, error)` cuyo
// segundo valor es una VARIABLE que quedo nil), leyendo los dos bytes de la interfaz con `unsafe`:
//
//	itab  = la primera palabra (la tabla de metodos)
//	data  = la segunda palabra (el puntero al valor)
//	m0    = *(itab+4), o sea el metodo 0 de la interfaz: para `error` (un solo metodo) es `Error()`
//
// Con el layout ya medido en la consola (itab: [+0] descriptor del tipo concreto, [+4] metodo[0]) un `error`
// sano tiene que dar: itab en .rodata, m0 en .text, y `e.Error()` tiene que devolver el texto.
//
// El despacho (`e.Error()`) va al FINAL y marcado: si uno se cae, se sabe cual por la ultima marca.
package main

import (
	"errors"
	"strconv"
	"unsafe"
)

//extern d2c_serial_init
func d2cSerialInit()

//extern d2c_frame
func d2cFrame()

type carga struct {
	a, b, c, d, e, f, g int
	s                   string
}

type miError struct{ n int }

func (e *miError) Error() string { return "miError" }

type conError struct{ err error }

// trabajo: ensucia registros y monton entre la variable nil y el `return` (lo que hace el motor entre
// `var err error` y su `return manager, err`).
func trabajo(c *carga) {
	basura := make([]int, 64)
	for i := range basura {
		basura[i] = i * 7
	}
	c.a, c.b, c.s = basura[3], basura[5], "trabajo"
}

// 1. el segundo valor es el LITERAL nil
func dosValoresLiteralNil() (*carga, error) {
	return &carga{a: 1}, nil
}

// 2. el segundo valor es una VARIABLE que quedo nil (la forma de d2asset.NewAssetManager)
func dosValoresVarNil() (*carga, error) {
	var err error

	c := &carga{b: 2}
	trabajo(c)

	return c, err
}

// 3. el error viaja por dos niveles (la forma del motor: el error cruza paquetes)
func dosValoresProfundo() (*carga, error) {
	x, err := dosValoresVarNil()

	return x, err
}

// 4. un error de verdad, en un `if` (la forma del error de init del motor)
func dosValoresErrorReal() (*carga, error) {
	var err error

	c := &carga{c: 3}
	if err = errors.New("error de verdad"); err != nil {
		return c, err
	}

	return c, err
}

// 5. el error como PRIMER valor (la otra forma del multi-retorno)
func errorPrimero() (error, int) {
	var err error
	if 1 == 1 {
		err = errors.New("error primero")
	}

	return err, 7
}

func hex(v uintptr) string {
	return "0x" + strconv.FormatUint(uint64(v), 16)
}

// ---- LA FORMA DEL MOTOR: asignacion multiple a CAMPOS DE STRUCT ----
//
// app.asset, app.errorMessage = d2asset.NewAssetManager(...)  (app.go:134)
//
// Es una asignacion multiple donde TODOS los lados izquierdos son SELECTORES de campo, no variables.
type appForma struct {
	t *carga
	e error
}

func nuevaFormaVarNil() *appForma {
	a := &appForma{}
	a.t, a.e = dosValoresVarNil()

	return a
}

func nuevaFormaLiteral() *appForma {
	a := &appForma{}
	a.t, a.e = dosValoresLiteralNil()

	return a
}

func nuevaFormaError() *appForma {
	a := &appForma{}
	a.t, a.e = dosValoresErrorReal()

	return a
}

func nuevaFormaProfundo() *appForma {
	a := &appForma{}
	a.t, a.e = dosValoresProfundo()

	return a
}

func nuevaFormaErrorPrimero() *appForma {
	a := &appForma{}
	_, a.t, a.e = errorPrimeroCarga()

	return a
}

// errorPrimeroCarga: un multi-retorno donde el error va ANTES del valor (el orden del ABI importa).
func errorPrimeroCarga() (*carga, *carga, error) {
	var err error
	if 1 == 1 {
		err = nil
	}

	return &carga{f: 5}, &carga{g: 6}, err
}

// mirarForma: las dos palabras del campo `e`, leidas DONDE las lee el compilador (`unsafe.Offsetof`),
// mas el offset para compararlo con el 156 que usa la sonda del motor.
func mirarForma(caso string, a *appForma) {
	off := unsafe.Offsetof(a.e)
	base := uintptr(unsafe.Pointer(a)) + off

	itab := palabra(base, 0)
	data := palabra(base, 1)

	m0 := uintptr(0)
	if itab != 0 {
		m0 = palabra(itab, 1)
	}

	esnil := "no"
	if a.e == nil {
		esnil = "si"
	}

	println("IFACE-FORMA", caso, "offset=", off, "esnil=", esnil, "itab=", hex(itab), "data=", hex(data),
		"m0=", hex(m0))
}

func palabra(base, i uintptr) uintptr {
	return *(*uintptr)(unsafe.Pointer(base + i*4))
}

func mirar(caso string, e error) {
	base := uintptr(unsafe.Pointer(&e))
	itab := palabra(base, 0)
	data := palabra(base, 1)

	m0 := uintptr(0)
	if itab != 0 {
		m0 = palabra(itab, 1)
	}

	esnil := "no"
	if e == nil {
		esnil = "si"
	}

	println("IFACE-ERROR", caso, "esnil=", esnil, "itab=", hex(itab), "data=", hex(data), "m0=", hex(m0))
}

func main() {
	d2cSerialInit()
	println("IFACE-ERROR arranca (DC-002.23: de que lado se rompe el error del motor)")

	e1 := errors.New("uno")

	_, e2 := dosValoresLiteralNil()
	_, e3 := dosValoresVarNil()
	_, e4 := dosValoresProfundo()
	_, e5 := dosValoresErrorReal()
	e6, _ := errorPrimero()

	h := &conError{}
	h.err = errors.New("guardado en un campo")

	var e7 error = &miError{9}

	// ---- 1a parte: los numeros, sin despachar nada
	mirar("1 errors.New", e1)
	mirar("2 multi literal nil", e2)
	mirar("3 multi var nil", e3)
	mirar("4 multi por 2 niveles", e4)
	mirar("5 multi error real", e5)
	mirar("6 error como 1er valor", e6)
	mirar("7 campo de struct", h.err)
	mirar("8 tipo local", e7)
	mirar("9 nil pelado", nil)

	// ---- 2a parte: EL MISMO DATO, PERO COMO LO GUARDA EL MOTOR (app.go:134)
	//
	// `app.asset, app.errorMessage = d2asset.NewAssetManager(...)`: una ASIGNACION MULTIPLE a dos CAMPOS DE
	// STRUCT del retorno de dos valores de una funcion. Las dos formas de arriba (locales) dan nil correcto.
	// Las de abajo miden la forma del motor, y por que importa: en el arranque del motor (motor-arranque,
	// gox con la sonda de DC-002.23) el campo `errorMessage` del App aparece con basura en el itab
	// (`itab=0x9 data=0x8c912590`) y NO lo cambia nadie entre que se guarda y se usa. Esta parte dice si el
	// defecto es de la ASIGNACION a campos (y entonces se reproduce sin el motor).
	f10 := nuevaFormaVarNil()
	mirarForma("10 campo, var nil (forma app.go:134)", f10)

	f11 := nuevaFormaLiteral()
	mirarForma("11 campo, literal nil", f11)

	f12 := nuevaFormaError()
	mirarForma("12 campo, error real", f12)

	f13 := nuevaFormaProfundo()
	mirarForma("13 campo, por 2 niveles", f13)

	f14 := nuevaFormaErrorPrimero()
	mirarForma("14 campo, error como 1er valor", f14)

	// ---- 3a parte: el despacho, marcado uno por uno (el que se caiga se lleva los siguientes)
	println("IFACE-ERROR despacho 1")
	println("IFACE-ERROR 1 ->", e1.Error())
	println("IFACE-ERROR despacho 2 (multi var nil, la forma del motor)")
	if e3 == nil {
		println("IFACE-ERROR 2 es nil: no se despacha")
	} else {
		println("IFACE-ERROR 2 ->", e3.Error())
	}

	println("IFACE-ERROR despacho 3 (campo de struct)")
	println("IFACE-ERROR 3 ->", h.err.Error())

	println("IFACE-ERROR fin")

	for i := 0; i < 8; i++ {
		d2cFrame()
	}

	println("IFACE-ERROR saliendo")
}
