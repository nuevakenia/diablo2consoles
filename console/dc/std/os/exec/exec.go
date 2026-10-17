// Package exec: la parte de os/exec que la consola puede tener.
//
// Para que sirve en el juego: d2hero/d2s_integrity.go lo usa para llamar a un programa externo al
// verificar los archivos de guardado (una herramienta del sistema). En la Dreamcast no hay programas que
// ejecutar: no hay shell, ni procesos, ni disco.
//
// Por eso aca Command existe para que el codigo compile, pero correrlo SIEMPRE falla, con un error que
// dice por que. No se finge: si el juego llega a necesitar esa verificacion, va a fallar de forma clara
// (y en la consola no hace falta: el guardado no existe).
package exec

import "d2std/errors"

// ErrNoSePuede es el error de "en la consola no se ejecutan programas".
var ErrNoSePuede = errors.New("exec: en la consola no hay programas externos que ejecutar")

// Cmd: el programa a ejecutar (la forma de os/exec).
type Cmd struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

// Command arma el comando (no lo ejecuta).
func Command(nombre string, args ...string) *Cmd {
	return &Cmd{Path: nombre, Args: args}
}

// Run: siempre falla, con el motivo.
func (c *Cmd) Run() error { return ErrNoSePuede }

// Start / Wait: lo mismo.
func (c *Cmd) Start() error { return ErrNoSePuede }
func (c *Cmd) Wait() error  { return ErrNoSePuede }

// Output / CombinedOutput: sin salida y con el error.
func (c *Cmd) Output() ([]byte, error)         { return nil, ErrNoSePuede }
func (c *Cmd) CombinedOutput() ([]byte, error) { return nil, ErrNoSePuede }

// String: la descripcion.
func (c *Cmd) String() string { return "exec.Cmd(" + c.Path + ")" }
