//go:build dreamcast

// Sonda 4: separa dos sospechas: la concatenacion de tres textos y la asercion interface{}->error.
package main

import (
	"errors"
	"fmt"
	"strconv"
)

//extern d2c_serial_init
func d2c_serial_init()

type miError struct{ s string }

func (e miError) Error() string { return e.s }

type miStringer interface{ String() string }

type miTexto struct{}

func (miTexto) String() string { return "miTexto" }

func main() {
	d2c_serial_init()
	println("S1: arranca")

	println("S2 dos textos:", "a"+"b")
	println("S3 tres textos:", "a"+"b"+"c")
	d := "3500000"
	println("S4 tres trozos:", d[:1]+"."+d[1:])
	println("S5 corte y concat:", d[:1], d[1:])
	println("S6 Itoa:", strconv.Itoa(1234))
	println("S7 FormatFloat(3.5,2):", strconv.FormatFloat(3.5, 'f', 2, 64))

	eLocal := error(miError{"error local"})
	println("S8 error local, Error():", eLocal.Error())

	eAjeno := error(errors.New("error de otro paquete"))
	println("S9 error ajeno, Error():", eAjeno.Error())

	var vacio interface{} = miError{"por interface{}"}
	if v, ok := vacio.(error); ok {
		println("S10 asercion interface{}->error: ok ->", v.Error())
	} else {
		println("S10 asercion interface{}->error: FALLO")
	}

	var s miStringer = miTexto{}
	if v, ok := interface{}(s).(miStringer); ok {
		println("S11 asercion interface{}->miStringer: ok ->", v.String())
	} else {
		println("S11 asercion interface{}->miStringer: FALLO")
	}

	println("S12 %v de un error local:", fmt.Sprintf("%v", eLocal))
	println("S13 %v de un texto:", fmt.Sprintf("%v", "texto"))
	println("S14 fin")
	for {
	}
}
