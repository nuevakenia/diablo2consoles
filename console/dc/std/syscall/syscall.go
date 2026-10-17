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

// ---------------------------------------------------------------- los tiempos del proceso
//
// PARA QUE SE USA: d2app/autoperf_unix.go mide cuanto tiempo de CPU gasto el proceso (que, a diferencia
// del reloj de pared, no se infla cuando otra cosa usa la maquina). Es una herramienta de medicion del
// que programa, no parte del juego.
//
// POR QUE ES UN STUB: la Dreamcast no tiene procesos de sistema operativo ni contabilidad de tiempos de
// CPU por proceso (no hay kernel POSIX). El valor que devuelve es CERO, y esta dicho: no se finge una
// medicion que no existe. Para medir de verdad en la consola estan el reloj del hardware (TimerUsGettime64)
// y la consola serie.

// Rusage: el uso de recursos del proceso. Rusage = "resource usage".
type Rusage struct {
	Utime Timeval // tiempo en modo usuario
	Stime Timeval // tiempo en modo sistema
}

// RUSAGE_SELF: "el propio proceso" (el unico valor que tiene sentido en la consola).
const RUSAGE_SELF = 0

// Getrusage: llena el uso de recursos. En la consola deja todo en cero (ver arriba) y devuelve nil, que
// es lo que el que llama espera cuando hay respuesta; el cero es el valor.
func Getrusage(quien int, uso *Rusage) error {
	if uso != nil {
		*uso = Rusage{}
	}
	return nil
}

// GetCurrentProcess: el identificador del proceso. En la consola hay uno solo: 1.
func GetCurrentProcess() int { return 1 }

// Timeval: un tiempo en segundos y microsegundos, como en POSIX.
type Timeval struct {
	Sec  int64
	Usec int64
}

// Nano: los nanosegundos totales (asi se usa en Go: ru.Utime.Nano()).
func (t Timeval) Nano() int64 { return t.Sec*1e9 + t.Usec*1e3 }

// Los errores de red. En la consola no hay red (ver el paquete net), pero el motor los NOMBRA al comparar
// el error que devolvio una conexion (d2networking/d2gsnet/env.go:29: si el error es ECONNRESET, la
// conexion se corto de golpe). Existen para poder comparar; nunca los va a devolver nadie.
var (
	ECONNRESET   = errors.New("connection reset by peer")
	ECONNREFUSED = errors.New("connection refused")
	ETIMEDOUT    = errors.New("connection timed out")
)
