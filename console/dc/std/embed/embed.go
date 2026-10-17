// Package embed: la parte de embed que la consola puede tener.
//
// Para que sirve en Go: meter archivos DENTRO del binario con una directiva //go:embed. En la consola eso
// tiene otro camino: los archivos van al romdisk (build-dc.sh los hornea ahi) y se leen con os.
//
// Por que igual existe: el motor importa embed en d2common/d2quest (el archivo de mensajes de los NPC).
// Sin este paquete, el que compila con gccgo corta con "build system did not supply embed configuration";
// con el, el paquete compila y la variable queda vacia.
//
// LIMITE DECLARADO: FS es un sistema de archivos VACIO. Si el juego necesita el texto de los mensajes de
// los NPC, hay que hacer lo que se hizo con el resto de los assets: hornear el archivo al romdisk y
// leerlo de ahi con os. No se finge que hay contenido: ReadFile devuelve el error de "no esta".
package embed

import (
	"errors"
	"io/fs"
)

// FS es el sistema de archivos embebido. Aca no embebe nada: existe para que el codigo que lo declara
// compile, y para que su lectura falle de forma clara (y no con contenido inventado).
type FS struct{}

// String: la descripcion.
func (f FS) String() string { return "embed.FS (vacio en la consola: los archivos van al romdisk)" }

// Open: no hay archivos.
func (f FS) Open(name string) (fs.File, error) { return nil, errNoExiste }

// ReadFile: no hay archivos.
func (f FS) ReadFile(name string) ([]byte, error) { return nil, errNoExiste }

// ReadDir: no hay directorios.
func (f FS) ReadDir(name string) ([]fs.DirEntry, error) { return nil, errNoExiste }

// Sub: la vista de un subarbol. Devuelve la misma (vacia).
func (f FS) Sub(dir string) (FS, error) { return f, nil }

var errNoExiste = errors.New("embed: en la consola los archivos van al romdisk, no embebidos")
