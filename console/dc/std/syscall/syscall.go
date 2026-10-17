// Package syscall: la parte de syscall que la consola necesita.
//
// Para que sirve: d2hero/d2s_lock_unix.go toma un lock de archivo con syscall.Flock para que dos ventanas
// del juego que guardan el mismo heroe se turnen. En la Dreamcast no hay disco (el romdisk es de solo
// lectura) y hay un solo jugador, asi que aca Flock dice "ya esta" sin hacer nada: no hay con quien
// pelearse el archivo.
//
// LIMITE DECLARADO: Flock no bloquea de verdad. Si en algun momento hubiera varios escritores, esto no
// los frena (en la consola no los hay: no se puede escribir).
package syscall

import "errors"

// los numeros del lock, con los mismos valores que linux (el motor los compara despues de Flock)
const (
	LOCK_SH = 1
	LOCK_EX = 2
	LOCK_NB = 4
	LOCK_UN = 8
)

// los errores que el motor nombra
var (
	EAGAIN      = errors.New("resource temporarily unavailable")
	EWOULDBLOCK = EAGAIN
)

// Flock: en la consola no hay con quien competir el archivo, asi que siempre dice que si.
func Flock(fd int, como int) error { return nil }
