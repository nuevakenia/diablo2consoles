// Package fs: las interfaces del sistema de archivos de Go (io/fs), para la consola.
//
// Son las interfaces que usa os y que d2mpq pide. Aca van los TIPOS y las interfaces (que es lo que
// hace falta para compilar y para hablar de archivos); las implementaciones concretas viven en os, que
// en la consola camina sobre el romdisk.
//
// Limite declarado: no estan sub.FS, Glob ni WalkDir (los que recorren arboles): en la consola la lista
// de archivos se resuelve con el .manifiesto, que es la forma que ya se uso para el resto.
package fs

import "d2std/time"

// FileMode y FileInfo: los mismos tipos de Go (alias a lo que usa os).
type FileMode uint32

const (
	ModeDir        FileMode = 1 << (32 - 1 - iota) // d: es un directorio
	ModeAppend                                     // a: solo agregar
	ModeExclusive                                  // l: uso exclusivo
	ModeTemporary                                  // T: temporal
	ModeSymlink                                    // L: enlace simbolico
	ModeDevice                                     // D: dispositivo
	ModeNamedPipe                                  // p: tuberia con nombre
	ModeSocket                                     // S: socket
	ModeSetuid                                     // u: setuid
	ModeSetgid                                     // g: setgid
	ModeCharDevice                                 // c: dispositivo de caracteres
	ModeSticky                                     // t: sticky
	ModeIrregular                                  // ?: irregular
	ModeType    = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice | ModeCharDevice | ModeIrregular
	ModePerm    = 0777
)

func (m FileMode) IsDir() bool      { return m&ModeDir != 0 }
func (m FileMode) IsRegular() bool  { return m&ModeType == 0 }
func (m FileMode) Perm() FileMode   { return m & ModePerm }
func (m FileMode) String() string   { return "fs.FileMode" }

// FileInfo es la interfaz de Go, igual.
type FileInfo interface {
	Name() string
	Size() int64
	Mode() FileMode
	ModTime() time.Time
	IsDir() bool
	Sys() interface{}
}

// DirEntry es la interfaz de Go, igual.
type DirEntry interface {
	Name() string
	IsDir() bool
	Type() FileMode
	Info() (FileInfo, error)
}

// FS es la interfaz de Go, igual: un sistema de archivos que sabe abrir archivos por nombre.
type FS interface {
	Open(name string) (File, error)
}

// File es la interfaz de Go, igual.
type File interface {
	Stat() (FileInfo, error)
	Read([]byte) (int, error)
	Close() error
}

// ReadFileFS / ReadDirFS: las interfaces opcionales que Go agrega encima de FS.
type ReadFileFS interface {
	ReadFile(name string) ([]byte, error)
}

type ReadDirFS interface {
	ReadDir(name string) ([]DirEntry, error)
}

// ValidPath: la validacion de Go, tal cual (un camino valido no esta vacio, no es "." y no arranca ni
// termina con barra).
func ValidPath(name string) bool {
	if name == "" || name == "." {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] == 0 {
			return false
		}
	}
	if name[0] == '/' {
		return false
	}
	if len(name) > 1 && name[len(name)-1] == '/' {
		return false
	}
	return true
}
