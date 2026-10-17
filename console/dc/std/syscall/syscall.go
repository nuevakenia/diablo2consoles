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

import "d2std/errors"

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

// ---------------------------------------------------------------- las senales
//
// En la consola NO hay senales del sistema operativo: no hay procesos que manden senales, y el juego se
// apaga con el boton de la maquina o reiniciando. Existen para que el codigo que las NOMBRA compile
// (d2app/app.go:156 arma el apagado ordenado con signal.Notify(c, os.Interrupt, syscall.SIGTERM)).
//
// El tipo implementa os.Signal sin importar os (tiene que poder, porque el canal de signal.Notify es de
// os.Signal): con String() y Signal() alcanza, que es como funcionan las interfaces de Go.

// Signal: una senal. No hay numeros de senal en la consola, solo el nombre.
type Signal struct {
	nombre string
}

// String: el nombre de la senal.
func (s Signal) String() string { return s.nombre }

// Signal: la marca que hace que esto sea un os.Signal.
func (s Signal) Signal() {}

// Las senales que el motor nombra, con los nombres de Linux.
var (
	SIGINT  = Signal{nombre: "interrupt"}
	SIGTERM = Signal{nombre: "terminated"}
	SIGKILL = Signal{nombre: "killed"}
	SIGHUP  = Signal{nombre: "hangup"}
	SIGQUIT = Signal{nombre: "quit"}
)
