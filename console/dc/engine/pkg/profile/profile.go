// Package profile: el perfilador de github.com/pkg/profile, para la consola.
//
// PARA QUE SE USA: d2app/app.go lo usa para sacar un perfil de CPU o de memoria mientras corre (una
// herramienta del que programa, para ver donde se va el tiempo).
//
// POR QUE ES UN STUB Y NO UN PERFILADOR: un perfil de verdad NECESITA ESCRIBIR UN ARCHIVO (el .pprof que
// despues se mira con `go tool pprof`), y en la consola no se puede escribir: el romdisk es de solo
// lectura. Un perfilador que junta datos para que nadie los pueda sacar no sirve.
//
// Y ademas, para medir donde se va el tiempo en la consola YA HAY algo mejor y que si funciona: la consola
// SERIE (los mensajes que el juego imprime) y los contadores del POC. Eso es lo que se usa aca.
//
// Las funciones existen para que el codigo compile y para que la herramienta se pueda activar sin que se
// caiga nada: simplemente no junta nada.
package profile

// Profile: el perfilador.
type Profile struct{}

// Las opciones, con los nombres que usa el motor (d2app/app.go:679-683).
func CPUProfile(p *Profile) {}
func MemProfile(p *Profile) {}
func NoShutdownHook(p *Profile) {}
func ProfilePath(ruta string) func(*Profile) {
	return func(p *Profile) {}
}
func Quiet(p *Profile) {}
func TraceProfile(p *Profile) {}

// Start: arranca el perfil. En la consola no hay archivo donde escribirlo, asi que no arranca nada y
// devuelve un perfil vacio (para que el codigo que lo guarda y despues llama a Stop siga su curso).
func Start(opciones ...func(*Profile)) *Profile {
	return &Profile{}
}

// Stop: termina el perfil. No hay nada que terminar.
func (p *Profile) Stop() {}
