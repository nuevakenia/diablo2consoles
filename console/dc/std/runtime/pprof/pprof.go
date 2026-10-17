// Package pprof: los perfiles de rendimiento, en la parte que la consola puede tener.
//
// PARA QUE SE USA: d2app/autoperf.go junta un perfil de CPU o de memoria mientras corre el juego, para que
// el que programa vea donde se va el tiempo. Es una herramienta de desarrollo.
//
// POR QUE NO ANDA: un perfil NO SIRVE EN MEMORIA. Su unico proposito es escribir un archivo .pprof que
// despues se mira con `go tool pprof` en la computadora. En la consola no se puede escribir (el romdisk es
// de solo lectura), o sea que un perfil juntado aca no lo puede leer nadie: no es una limitacion del
// puerto, es que la herramienta no tiene sentido sin disco.
//
// Y ademas el perfil de CPU en la consola seria mentiroso: el runtime de Go de la consola no tiene las
// senales del sistema operativo con las que se toma la muestra (no hay kernel POSIX), asi que igual no
// habria de donde sacarla.
//
// LO QUE SI HAY PARA MEDIR: la consola SERIE (los mensajes que el juego imprime), el reloj del hardware
// (TimerUsGettime64) y los contadores del POC. Por ahi se mide, y se mide de verdad.
//
// Las funciones existen para que el codigo compile y devuelven el error con el motivo, asi que el que
// llama se entera en vez de recibir un archivo vacio que parece bueno.
package pprof

import (
	"d2std/errors"
	"d2std/io"
)

// ErrSinDisco: el motivo, para que el error lo diga.
var ErrSinDisco = errors.New("pprof: en la consola no se puede escribir el perfil (el romdisk es de solo lectura)")

// Profile: un perfil juntado. En la consola nunca tiene datos.
type Profile struct {
	nombre string
}

// Lookup: busca un perfil por nombre ("allocs", "heap", "goroutine", ...). Devuelve uno vacio para que el
// que llama pueda pedirle WriteTo sin caerse.
func Lookup(nombre string) *Profile { return &Profile{nombre: nombre} }

// WriteTo: escribe el perfil. No se puede (ver arriba).
func (p *Profile) WriteTo(w io.Writer, debug int) error { return ErrSinDisco }

// StartCPUProfile: arranca el perfil de CPU. No se puede.
func StartCPUProfile(w io.Writer) error { return ErrSinDisco }

// StopCPUProfile: lo termina. Como nunca arranco, no hace nada.
func StopCPUProfile() {}

// WriteHeapProfile: escribe el perfil de memoria. No se puede.
func WriteHeapProfile(w io.Writer) error { return ErrSinDisco }
