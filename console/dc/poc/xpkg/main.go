//go:build dreamcast

// Caso minimo de DOS paquetes: el tipo y su metodo viven en el paquete "saludo", la asercion de
// interfaz la hace main. Es el mismo cruce que hace una biblioteca estandar, sin nada mas encima.
package main

import "saludo"

//extern d2c_serial_init
func d2c_serial_init()

func main() {
	d2c_serial_init()
	println("XP1: arranca")

	e := saludo.ComoError("valor")
	println("XP2: llamada al metodo del otro paquete:", e.Error())

	var v interface{} = e
	if x, ok := v.(error); ok {
		println("XP3: asercion interface{}->error OK ->", x.Error())
	} else {
		println("XP3: asercion interface{}->error FALLO")
	}

	p := saludo.Desenvuelto("puntero")
	println("XP4: metodo con puntero:", p.Error())

	var w interface{} = p
	if x, ok := w.(error); ok {
		println("XP5: asercion con puntero OK ->", x.Error())
	} else {
		println("XP5: asercion con puntero FALLO")
	}

	if t, ok := v.(saludo.Texto); ok {
		println("XP6: asercion a tipo concreto OK ->", t.Nombre())
	} else {
		println("XP6: asercion a tipo concreto FALLO")
	}

	println("XP7: fin")
	for {
	}
}
