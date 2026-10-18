// sonda-isexit: LA TABLA DE REFERENCIA de `os.IsExist` y `os.IsNotExist` de la biblioteca de Go.
//
// DC-002.31. La necesita el corpus porque la consola NO puede comparar contra Go en el host (el paquete os
// de la stdlib minima habla con KOS por //extern y no compila aca), asi que sus valores esperados tienen
// que venir de una MEDICION de la biblioteca real y no de la memoria de nadie. Este programa es esa
// medicion: corre en el host, imprime una linea por caso y el corpus exige esa tabla.
//
// POR QUE IMPORTA (el defecto que la tarjeta encontro): el paquete os de la consola tenia
//
//	func IsExist(err error) bool { return !IsNotExist(err) }
//
// y con `err == nil` -- el caso del os.Stat de un archivo que SI esta, que es lo que mira el `Exists` del
// proveedor de sistema de archivos del motor -- eso da TRUE donde Go da FALSE. El mismo codigo del motor
// respondia distinto en la consola que en el host.
//
// Uso:  go run ./console/dc/sonda-isexit
package main

import (
	"errors"
	"fmt"
	"os"
)

// envoltorio: un error con Unwrap, para medir si Go camina los envoltorios (NO los camina: solo mira
// *PathError/*LinkError/*SyscallError).
type envoltorio struct{ err error }

func (e envoltorio) Error() string { return "envolviendo: " + e.err.Error() }
func (e envoltorio) Unwrap() error { return e.err }

func main() {
	casos := []struct {
		nombre string
		err    error
	}{
		{"nil", nil},
		{"ErrExist", os.ErrExist},
		{"ErrNotExist", os.ErrNotExist},
		{"ErrPermission", os.ErrPermission},
		{"otro", errors.New("otro")},
		{"envuelto_ErrNotExist", envoltorio{os.ErrNotExist}},
	}

	for _, c := range casos {
		fmt.Printf("SONDA-ISEXIT %s exist=%v notexist=%v\n", c.nombre, os.IsExist(c.err), os.IsNotExist(c.err))
	}

	// y con el error DE VERDAD de un archivo que no esta, y con el nil de uno que si esta
	_, err := os.Open("/tmp/dc231-sonda-isexit-no-existe.txt")
	fmt.Printf("SONDA-ISEXIT error_de_Open exist=%v notexist=%v\n", os.IsExist(err), os.IsNotExist(err))

	_, err = os.Stat("/tmp")
	fmt.Printf("SONDA-ISEXIT nil_de_Stat exist=%v notexist=%v\n", os.IsExist(err), os.IsNotExist(err))
}
