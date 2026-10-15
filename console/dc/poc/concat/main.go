//go:build dreamcast

// Compara la misma operacion hecha aca dentro contra la que hace el paquete strconv, para saber si
// el problema esta en el codigo o en el cruce de paquetes.
package main

import (
	"strconv"
	"strings"
)

//extern d2c_serial_init
func d2c_serial_init()

// Copia textual del final de insertPoint de nuestro strconv.
func insertLocal(n uint64, prec int) string {
	s := strconv.FormatUint(n, 10)
	if prec == 0 {
		return s
	}
	if len(s) <= prec {
		s = strings.Repeat("0", prec-len(s)+1) + s
	}
	return s[:len(s)-prec] + "." + s[len(s)-prec:]
}

func main() {
	d2c_serial_init()
	println("CC1: arranca")
	println("CC2: local (150,2) ->", insertLocal(150, 2))
	println("CC3: local (0,2) ->", insertLocal(0, 2))
	println("CC4: FormatUint(150,10) ->", strconv.FormatUint(150, 10))
	println("CC5: FormatFloat f2 ->", strconv.FormatFloat(1.5, 'f', 2, 64))
	println("CC6: FormatFloat f6 ->", strconv.FormatFloat(1.5, 'f', 6, 64))
	println("CC7: FormatFloat e2 ->", strconv.FormatFloat(1.5, 'e', 2, 64))
	println("CC8: FormatFloat g2 ->", strconv.FormatFloat(1.5, 'g', 2, 64))
	println("CC9: FormatFloat 0.0 f2 ->", strconv.FormatFloat(0.0, 'f', 2, 64))
	println("CC10: fin")
	for {
	}
}
